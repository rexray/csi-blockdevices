package services

import (
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/thecodeteam/gocsi/csi"
)

const (
	Name    = "csi-blockdevices"
	Version = "0.1.0"

	blockDirEnvVar = "X_CSI_BD_DEVDIR"

	defaultDevDir = "/dev/disk/csi-blockdevices"
)

var (
	CSIVersions = []*csi.Version{
		&csi.Version{
			Major: 0,
			Minor: 0,
			Patch: 0,
		},
	}
)

// Service is the CSI Network File System (NFS) service provider.
type Service interface {
	csi.ControllerServer
	csi.IdentityServer
	csi.NodeServer
}

// storagePlugin contains parameters for the plugin
type storagePlugin struct {
	DevDir  string
	privDir string
}

// New returns a new Service
func New() Service {

	sp := &storagePlugin{
		DevDir: defaultDevDir,
	}
	if dd := os.Getenv(blockDirEnvVar); dd != "" {
		sp.DevDir = dd
	}
	sp.privDir = filepath.Join(sp.DevDir, ".mounts")
	log.WithFields(map[string]interface{}{
		"devDir":  sp.DevDir,
		"privDir": sp.privDir,
	}).Info("created new " + Name + " service")

	return sp
}
