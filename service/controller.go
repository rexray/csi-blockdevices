package service

import (
	"context"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/container-storage-interface/spec/lib/go/csi"

	"github.com/thecodeteam/csi-blockdevices/block"
)

func (s *service) ControllerGetCapabilities(
	ctx context.Context,
	req *csi.ControllerGetCapabilitiesRequest) (
	*csi.ControllerGetCapabilitiesResponse, error) {

	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: []*csi.ControllerServiceCapability{
			&csi.ControllerServiceCapability{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
					},
				},
			},
		},
	}, nil
}

func (s *service) CreateVolume(
	ctx context.Context,
	req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {

	return nil, status.Error(codes.Unimplemented, "")
}

func (s *service) DeleteVolume(
	ctx context.Context,
	req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {

	return nil, status.Error(codes.Unimplemented, "")
}

func (s *service) ControllerPublishVolume(
	ctx context.Context,
	req *csi.ControllerPublishVolumeRequest) (
	*csi.ControllerPublishVolumeResponse, error) {

	return nil, status.Error(codes.Unimplemented, "")
}

func (s *service) ControllerUnpublishVolume(
	ctx context.Context,
	req *csi.ControllerUnpublishVolumeRequest) (
	*csi.ControllerUnpublishVolumeResponse, error) {

	return nil, status.Error(codes.Unimplemented, "")
}

func (s *service) ValidateVolumeCapabilities(
	ctx context.Context,
	req *csi.ValidateVolumeCapabilitiesRequest) (
	*csi.ValidateVolumeCapabilitiesResponse, error) {

	r := &csi.ValidateVolumeCapabilitiesResponse{
		Supported: true,
	}

	_, err := GetDeviceInDir(s.DevDir, req.VolumeId)
	if err != nil {
		log.WithError(err).Error("device does not appear to exist")
		return nil, status.Error(codes.NotFound, "volumeID not found")
	}

	for _, c := range req.VolumeCapabilities {
		if t := c.GetMount(); t != nil {
			// If a filesystem is given, make sure host supports it
			fs := t.GetFsType()
			if fs != "" {
				hostFSs, err := block.GetHostFileSystems("")
				if err != nil {
					return nil, status.Errorf(
						codes.Internal,
						"unable to get host supported filesystesm: %s", err.Error())
				}
				if !contains(hostFSs, fs) {
					return nil, status.Errorf(
						codes.InvalidArgument,
						"no host support for fstype: %s", fs)
				}
			}
			// TODO: Check mount flags
			//for _, f := range t.GetMountFlags() {}
		}
		if t := c.GetAccessMode(); t != nil {
			if t.GetMode() != csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER ||
				t.GetMode() != csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY {
				return nil, status.Error(codes.InvalidArgument,
					"invalid access mode")
			}
		}
	}

	return r, nil
}

func (s *service) ListVolumes(
	ctx context.Context,
	req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {

	vols, err := ListDevices(s.DevDir)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"unable to list devices: %s", err.Error())
	}

	entries := []*csi.ListVolumesResponse_Entry{}
	for _, v := range vols {
		vi := &csi.VolumeInfo{
			Id: v.Name,
		}
		entries = append(entries,
			&csi.ListVolumesResponse_Entry{
				VolumeInfo: vi,
			})

	}
	return &csi.ListVolumesResponse{
		Entries: entries,
	}, nil
}

func (s *service) GetCapacity(
	ctx context.Context,
	req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {

	return nil, status.Error(codes.Unimplemented, "")
}

func (s *service) ControllerProbe(
	ctx context.Context,
	req *csi.ControllerProbeRequest) (*csi.ControllerProbeResponse, error) {

	return &csi.ControllerProbeResponse{}, nil
}
