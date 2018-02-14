package service

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	log "github.com/sirupsen/logrus"
	"github.com/thecodeteam/gofsutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var fs = &gofsutil.FS{
	ScanEntry: entryScanFunc,
}

func entryScanFunc(
	ctx context.Context,
	entry gofsutil.Entry,
	cache map[string]gofsutil.Entry) (
	info gofsutil.Info, valid bool, failed error) {

	// Validate the mount table entry.
	validFSType, _ := regexp.MatchString(
		`(?i)^devtmpfs|(?:tmpfs)$`, entry.FSType)
	sourceHasSlashPrefix := strings.HasPrefix(entry.MountSource, "/")
	if valid = validFSType || sourceHasSlashPrefix; !valid {
		return
	}

	// Copy the Entry object's fields to the Info object.
	info.Device = entry.MountSource
	info.Opts = make([]string, len(entry.MountOpts))
	copy(info.Opts, entry.MountOpts)
	info.Path = entry.MountPoint
	info.Type = entry.FSType
	info.Source = entry.MountSource

	// If this is the first time a source is encountered in the
	// output then cache its mountPoint field as the filesystem path
	// to which the source is mounted as a non-bind mount.
	//
	// Subsequent encounters with the source will resolve it
	// to the cached root value in order to set the mount info's
	// Source field to the the cached mountPont field value + the
	// value of the current line's root field.
	if cachedEntry, ok := cache[entry.MountSource]; ok {
		info.Source = path.Join(cachedEntry.MountPoint, entry.Root)
	} else {
		cache[entry.MountSource] = entry
	}

	return
}

// Device is a struct for holding details about a block device
type Device struct {
	FullPath string
	Name     string
	RealDev  string
}

// GetDeviceInDir returns a Device struct with info about the given device,
// by looking for name in dir.
func GetDeviceInDir(dir, name string) (*Device, error) {
	dp := filepath.Join(dir, name)
	return GetDevice(dp)
}

// GetDevice returns a Device struct with info about the given device, or
// an error if it doesn't exist or is not a block device
func GetDevice(path string) (*Device, error) {

	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}

	// eval any symlinks and make sure it points to a device
	d, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}

	// TODO does EvalSymlinks throw error if link is to non-
	// existent file? assuming so by masking error below
	ds, _ := os.Stat(d)
	dm := ds.Mode()
	if dm&os.ModeDevice == 0 {
		return nil, fmt.Errorf(
			"%s is not a block device", path)
	}

	dev := &Device{
		Name:     fi.Name(),
		FullPath: path,
		RealDev:  d,
	}

	log.WithField("device", dev).Debug("got device")
	return dev, nil
}

// ListDevices returns a slice of Device for all valid blockdevices found
// in the given device directory
func ListDevices(dir string) ([]*Device, error) {
	fi, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("path %s does not exist", dir)
		}
		return nil, err
	}
	mode := fi.Mode()
	if !mode.IsDir() {
		return nil, fmt.Errorf("path %s is not a directory", dir)
	}

	fields := log.Fields{
		"path": dir,
	}

	devs := []*Device{}

	walk := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Error(err)
			return err
		}
		if info.IsDir() {
			if path == dir {
				return nil
			}
			log.WithField("file", path).Debug("skipping dir")
			return filepath.SkipDir
		}
		log.WithFields(fields).WithField("file", info.Name()).Debug(
			"examining file")

		dev, deverr := GetDevice(path)
		if deverr != nil {
			log.WithFields(fields).WithField("file", info.Name()).WithError(deverr).Debug(
				"not a device")
			return nil
		}
		devs = append(devs, dev)
		return nil
	}

	log.WithFields(fields).Debug("listing devices")
	err = filepath.Walk(dir, walk)
	if err != nil {
		return nil, err
	}
	return devs, nil
}

// publishVolume uses the parameters in req to bindmount the underlying block
// device to the requested target path. A private mount is performed first
// within the given privDir directory.
//
// publishVolume handles both Mount and Block access types
func publishVolume(
	req *csi.NodePublishVolumeRequest,
	privDir, device string) error {

	id := req.GetVolumeId()

	target := req.GetTargetPath()
	if target == "" {
		return status.Error(codes.InvalidArgument,
			"target_path is required")
	}

	ro := req.GetReadonly()

	volCap := req.GetVolumeCapability()
	if volCap == nil {
		return status.Errorf(codes.InvalidArgument,
			"volume capability required")
	}

	accMode := volCap.GetAccessMode()
	if accMode == nil {
		return status.Errorf(codes.InvalidArgument,
			"access mode required")
	}

	// make sure device is valid
	sysDevice, err := GetDevice(device)
	if err != nil {
		return status.Errorf(codes.Internal,
			"error getting block device for volume: %s, err: %s",
			id, err.Error())
	}

	// make sure target is created
	tgtStat, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return status.Errorf(codes.FailedPrecondition,
				"publish target: %s not pre-created", target)
		}
		return status.Errorf(codes.Internal,
			"failed to stat target, err: %s", err.Error())
	}

	// make sure privDir exists and is a directory
	privDirStat, err := os.Stat(privDir)
	if err != nil {
		if os.IsNotExist(err) {
			return status.Errorf(codes.Internal,
				"plugin private dir: %s not pre-created", privDir)
		}
		return status.Errorf(codes.Internal,
			"failed to stat private dir, err: %s", err.Error())
	}
	if !privDirStat.IsDir() {
		return status.Errorf(codes.Internal,
			"private dir: %s is not a directory", privDir)
	}

	isBlock := false
	typeSet := false
	if blockVol := volCap.GetBlock(); blockVol != nil {
		// Read-only is not supported for BlockVolume. Doing a read-only
		// bind mount of the device to the target path does not prevent
		// the underlying block device from being modified, so don't
		// advertise a false sense of security
		if ro {
			return status.Error(codes.InvalidArgument,
				"read only not supported for Block Volume")
		}
		isBlock = true
		typeSet = true
	}
	mntVol := volCap.GetMount()
	if mntVol != nil {
		typeSet = true
	}
	if !typeSet {
		return status.Errorf(codes.InvalidArgument,
			"access type required")
	}

	// check that target is right type for vol type
	if !(tgtStat.IsDir() == !isBlock) {
		return status.Errorf(codes.FailedPrecondition,
			"target: %s wrong type (file vs dir) Access Type", target)
	}

	// Path to mount device to
	privTgt := getPrivateMountPoint(privDir, id)

	f := log.Fields{
		"id":           id,
		"volumePath":   sysDevice.FullPath,
		"device":       sysDevice.RealDev,
		"target":       target,
		"privateMount": privTgt,
	}

	ctx := context.Background()

	// Check if device is already mounted
	devMnts, err := getDevMounts(sysDevice)
	if err != nil {
		return status.Errorf(codes.Internal,
			"could not reliably determine existing mount status: %s",
			err.Error())
	}

	if len(devMnts) == 0 {
		// Device isn't mounted anywhere, do the private mount
		log.WithFields(f).Debug("attempting mount to private area")

		// Make sure private mount point exists
		var created bool
		if isBlock {
			created, err = mkfile(privTgt)
		} else {
			created, err = mkdir(privTgt)
		}
		if err != nil {
			return status.Errorf(codes.Internal,
				"Unable to create private mount point: %s",
				err.Error())
		}
		if !created {
			log.WithFields(f).Debug("private mount target already exists")

			// The place where our device is supposed to be mounted
			// already exists, but we also know that our device is not mounted anywhere
			// Either something didn't clean up correctly, or something else is mounted
			// If the private mount is not in use, it's okay to re-use it. But make sure
			// it's not in use first

			mnts, err := fs.GetMounts(ctx)
			if err != nil {
				return status.Errorf(codes.Internal,
					"could not reliably determine existing mount status: %s",
					err.Error())
			}
			for _, m := range mnts {
				if m.Path == privTgt {
					log.WithFields(f).WithField("mountedDevice", m.Device).Error(
						"mount point already in use by device")
					return status.Error(codes.Internal,
						"Unable to use private mount point")
				}
			}
		}

		if !isBlock {
			fs := mntVol.GetFsType()
			mntFlags := mntVol.GetMountFlags()

			if err := handlePrivFSMount(
				ctx, accMode, sysDevice, mntFlags, fs, privTgt); err != nil {
				return err
			}
		} else {
			if err := fs.BindMount(ctx, sysDevice.FullPath, privTgt); err != nil {
				return status.Errorf(codes.Internal,
					"failure bind-mounting block device to private mount: %s", err.Error())
			}
		}

	} else {
		// Device is already mounted. Need to ensure that it is already
		// mounted to the expected private mount, with correct rw/ro perms
		mounted := false
		for _, m := range devMnts {
			if m.Path == privTgt {
				mounted = true
				rwo := "rw"
				if ro {
					rwo = "ro"
				}
				if contains(m.Opts, rwo) {
					log.WithFields(f).Debug(
						"private mount already in place")
					break
				} else {
					return status.Error(codes.InvalidArgument,
						"access mode conflicts with existing mounts")
				}
			}
		}
		if !mounted {
			return status.Error(codes.Internal,
				"device already in use and mounted elsewhere")
		}
	}

	// Private mount in place, now bind mount to target path

	// If mounts already existed for this device, check if mount to
	// target path was already there
	if len(devMnts) > 0 {
		for _, m := range devMnts {
			if m.Path == target {
				// volume already published to target
				// if mount options look good, do nothing
				rwo := "rw"
				if accMode.GetMode() == csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY {
					rwo = "ro"
				}
				if !contains(m.Opts, rwo) {
					return status.Error(codes.Internal,
						"volume previously published with different options")

				}
				// Existing mount satisfies request
				log.WithFields(f).Debug("volume already published to target")
				return nil
			}
		}

	}

	var mntFlags []string
	if isBlock {
		mntFlags = make([]string, 0)
	} else {
		mntFlags = mntVol.GetMountFlags()
		if accMode.GetMode() == csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY {
			mntFlags = append(mntFlags, "ro")
		}
	}
	if err := fs.BindMount(ctx, privTgt, target, mntFlags...); err != nil {
		return status.Errorf(codes.Internal,
			"error publish volume to target path: %s",
			err.Error())
	}

	return nil
}

func handlePrivFSMount(
	ctx context.Context,
	accMode *csi.VolumeCapability_AccessMode,
	sysDevice *Device,
	mntFlags []string,
	filesys, privTgt string) error {

	// If read-only access mode, we don't allow formatting
	if accMode.GetMode() == csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY {
		mntFlags = append(mntFlags, "ro")
		if err := fs.Mount(ctx, sysDevice.FullPath, privTgt, filesys, mntFlags...); err != nil {
			return status.Errorf(codes.Internal,
				"error performing private mount: %s",
				err.Error())
		}
		return nil
	} else if accMode.GetMode() == csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER {
		if err := fs.FormatAndMount(ctx, sysDevice.FullPath, privTgt, filesys, mntFlags...); err != nil {
			return status.Errorf(codes.Internal,
				"error performing private mount: %s",
				err.Error())
		}
		return nil
	}
	return status.Error(codes.Internal, "Invalid access mode")
}

func getPrivateMountPoint(privDir string, name string) string {
	return filepath.Join(privDir, name)
}

func contains(list []string, item string) bool {
	for _, x := range list {
		if x == item {
			return true
		}
	}
	return false
}

// mkfile creates a file specified by the path if needed.
// return pair is a bool flag of whether file was created, and an error
func mkfile(path string) (bool, error) {
	st, err := os.Stat(path)
	if os.IsNotExist(err) {
		file, err := os.OpenFile(path, os.O_CREATE, 0755)
		if err != nil {
			log.WithField("dir", path).WithError(
				err).Error("Unable to create dir")
			return false, err
		}
		file.Close()
		log.WithField("path", path).Debug("created file")
		return true, nil
	}
	if st.IsDir() {
		return false, fmt.Errorf("existing path is a directory")
	}
	return false, nil
}

// mkdir creates the directory specified by path if needed.
// return pair is a bool flag of whether dir was created, and an error
func mkdir(path string) (bool, error) {
	st, err := os.Stat(path)
	if os.IsNotExist(err) {
		if err := os.Mkdir(path, 0755); err != nil {
			log.WithField("dir", path).WithError(
				err).Error("Unable to create dir")
			return false, err
		}
		log.WithField("path", path).Debug("created directory")
		return true, nil
	}
	if !st.IsDir() {
		return false, fmt.Errorf("existing path is not a directory")
	}
	return false, nil
}

// unpublishVolume removes the bind mount to the target path, and also removes
// the mount to the private mount directory if the volume is no longer in use.
// It determines this by checking to see if the volume is mounted anywhere else
// other than the private mount.
func unpublishVolume(
	req *csi.NodeUnpublishVolumeRequest,
	privDir, device string) error {

	ctx := context.Background()
	id := req.GetVolumeId()

	target := req.GetTargetPath()
	if target == "" {
		return status.Error(codes.InvalidArgument,
			"target_path is required")
	}

	// make sure device is valid
	sysDevice, err := GetDevice(device)
	if err != nil {
		return status.Errorf(codes.Internal,
			"error getting block device for volume: %s, err: %s",
			id, err.Error())
	}

	// Path to mount device to
	privTgt := getPrivateMountPoint(privDir, id)

	mnts, err := getDevMounts(sysDevice)
	if err != nil {
		return status.Errorf(codes.Internal,
			"could not reliably determine existing mount status: %s",
			err.Error())
	}

	tgtMnt := false
	privMnt := false
	for _, m := range mnts {
		if m.Source == sysDevice.RealDev || m.Device == sysDevice.RealDev {
			if m.Path == privTgt {
				privMnt = true
			} else if m.Path == target {
				tgtMnt = true
			}
		}
	}

	if tgtMnt {
		if err := fs.Unmount(ctx, target); err != nil {
			return status.Errorf(codes.Internal,
				"Error unmounting target: %s", err.Error())
		}
	}

	if privMnt {
		if err := unmountPrivMount(ctx, sysDevice, privTgt); err != nil {
			return status.Errorf(codes.Internal,
				"Error unmounting private mount: %s", err.Error())
		}
	}

	return nil
}

func unmountPrivMount(
	ctx context.Context,
	dev *Device,
	target string) error {

	mnts, err := getDevMounts(dev)
	if err != nil {
		return err
	}

	log.Debug("checking if we can unmount priv mount")
	// remove private mount if we can
	if len(mnts) == 1 && mnts[0].Path == target {
		if err := fs.Unmount(ctx, target); err != nil {
			return err
		}
		log.WithField("directory", target).Debug(
			"removing directory")
		os.Remove(target)
	}
	return nil
}

func getDevMounts(
	sysDevice *Device) ([]gofsutil.Info, error) {

	ctx := context.Background()
	devMnts := make([]gofsutil.Info, 0)

	mnts, err := fs.GetMounts(ctx)
	if err != nil {
		return devMnts, err
	}
	for _, m := range mnts {
		if m.Device == sysDevice.RealDev ||
			(m.Device == "devtmpfs" && m.Source == sysDevice.RealDev) ||
			(m.Device == "tmpfs" && m.Source == sysDevice.RealDev) {
			devMnts = append(devMnts, m)
		}
	}
	log.Debugf("device mounts: %+v", devMnts)
	return devMnts, nil
}
