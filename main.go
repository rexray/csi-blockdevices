package main

import (
	"context"

	"github.com/thecodeteam/csi-blockdevices/provider"
	"github.com/thecodeteam/csi-blockdevices/service"
	"github.com/thecodeteam/gocsi"
)

// main is ignored when this package is built as a go plug-in
func main() {
	gocsi.Run(
		context.Background(),
		service.Name,
		"A local block device Container Storage Interface (CSI) Plugin",
		usage,
		provider.New())
}

const usage = `    X_CSI_BD_DEVDIR
        Specifies the path to search for blockdevices made available to this
        plugin. Devices that should be used by this plugin should be symlink'd
        in this directory.
`
