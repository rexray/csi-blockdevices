package service

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/thecodeteam/csi-blockdevices/block"
)

var (
	emptyNodePubResp   = &csi.NodePublishVolumeResponse{}
	emptyNodeUnpubResp = &csi.NodeUnpublishVolumeResponse{}
)

func (s *service) NodePublishVolume(
	ctx context.Context,
	req *csi.NodePublishVolumeRequest) (
	*csi.NodePublishVolumeResponse, error) {

	id := req.VolumeId

	dev, err := GetDeviceInDir(s.DevDir, id)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	if err := publishVolume(req, s.privDir, dev.RealDev); err != nil {
		return nil, err
	}

	return emptyNodePubResp, nil
}

func (s *service) NodeUnpublishVolume(
	ctx context.Context,
	req *csi.NodeUnpublishVolumeRequest) (
	*csi.NodeUnpublishVolumeResponse, error) {

	id := req.VolumeId

	dev, err := GetDeviceInDir(s.DevDir, id)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	if err := unpublishVolume(req, s.privDir, dev.RealDev); err != nil {
		return nil, err
	}

	return emptyNodeUnpubResp, nil
}

func (s *service) GetNodeID(
	ctx context.Context,
	req *csi.GetNodeIDRequest) (*csi.GetNodeIDResponse, error) {

	return &csi.GetNodeIDResponse{}, nil
}

func (s *service) NodeProbe(
	ctx context.Context,
	req *csi.NodeProbeRequest) (*csi.NodeProbeResponse, error) {

	if err := block.Supported(); err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}

	return &csi.NodeProbeResponse{}, nil
}

func (s *service) NodeGetCapabilities(
	ctx context.Context,
	req *csi.NodeGetCapabilitiesRequest) (
	*csi.NodeGetCapabilitiesResponse, error) {

	return &csi.NodeGetCapabilitiesResponse{}, nil
}
