package block

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	log "github.com/sirupsen/logrus"
)

var (
	fsmap = map[string]struct{}{
		"xfs":   struct{}{},
		"ext3":  struct{}{},
		"ext4":  struct{}{},
		"btrfs": struct{}{},
	}
)

// Supported queries the underlying system to check if the required system
// executables are present
// If not, it returns an error
func Supported() error {
	switch runtime.GOOS {
	case "linux":
		fss, err := GetHostFileSystems("")
		if err != nil {
			return err
		}
		if len(fss) == 0 {
			return fmt.Errorf("%s", "No supported filesystems found")
		}
		return nil
	default:
		return fmt.Errorf("%s", "Plugin only supported on Linux OS")
	}
}

// GetHostFileSystems returns a slice of strings of filesystems supported by the
// host. Supported filesytems are restricted to ext3,ext4,xfs,btrfs
func GetHostFileSystems(binPath string) ([]string, error) {
	if binPath == "" {
		binPath = "/sbin"
	}

	s := filepath.Join(binPath, "mkfs.*")
	m, err := filepath.Glob(s)
	if err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, nil
	}

	fields := log.Fields{
		"binpath":  binPath,
		"globpath": s,
		"binaries": m,
	}

	log.WithFields(fields).Debug("found mkfs binaries")

	fss := make([]string, 0)
	for _, f := range m {
		fs := filepath.Ext(f)
		fs = strings.TrimLeft(fs, ".")
		if _, ok := fsmap[fs]; ok {
			fss = append(fss, fs)
		}
	}
	log.WithField("filesystems", fss).Info("found supported filesystems")

	return fss, nil
}
