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

const usage = ``
