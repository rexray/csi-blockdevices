# CSI plugin for local block devices [![Build Status](http://travis-ci.org/thecodeteam/csi-blockdevices.svg?branch=master)]

## Description
CSI-BlockDevices is a Container Storage Interface
([CSI](https://github.com/container-storage-interface/spec)) plugin for locally
attached block devices. Block devices can be exposed to the plugin by
symlinking them into a directory, by default `/dev/disk/csi-blockdevices`. See
sample commands for details.

This project may be compiled as a stand-alone binary using Golang that,
when run, provides a valid CSI endpoint. This project can also be
vendored or built as a Golang plugin in order to extend the functionality
of other programs.

## Runtime Dependencies
None

## Installation

CSI-BlockDevices can be installed with Go and the following command:

`$ go get github.com/thecodeteam/csi-blockdevices`

The resulting binary will be installed to `$GOPATH/bin/csi-blockdevices`.

If you want to build `csi-nblockdevices` with accurate version information,
you'll need to run the `go generate` command and build again:

```bash
$ go get github.com/thecodeteam/csi-blockdevices
$ cd $GOPATH/src/github.com/thecodeteam/csi-blockdevices
$ go generate && go install
```

The binary will once again be installed to `$GOPATH/bin/csi-blockdevices`.

## Start plugin

Before starting the plugin please set the environment variable
`CSI_ENDPOINT` to a valid Go network address such as `csi.sock`:

```bash
$ CSI_ENDPOINT=csi.sock ./csi-blockdevices
INFO[0000] configured com.thecodeteam.blockdevices       devicedir=/dev/disk/csi-blockdevices privatedir=/dev/disk/csi-bd-private
INFO[0000] identity service registered
INFO[0000] controller service registered
INFO[0000] node service registered
INFO[0000] serving                                       endpoint="unix://csi.sock"
```

The server can be shutdown by using `Ctrl-C` or sending the process
any of the standard exit signals.

## Using plugin
The CSI specification uses the gRPC protocol for plug-in communication.
The easiest way to interact with a CSI plugin is via the Container
Storage Client (`csc`) program provided via the
[GoCSI](https://github.com/thecodeteam/gocsi) project:

```bash
$ go get github.com/thecodeteam/gocsi
$ go install github.com/thecodeteam/gocsi/csc
```

Then, set have `csc` use the same `CSI_ENDPOINT`, and you can issue commands
to the plugin. Some examples...

Get the plugin's supported versions and plugin info:

```bash
$ ./csc -e csi.sock identity supported-versions
0.1.0

$ ./csc -v 0.1.0 -e csi.sock identity plugin-info
"com.thecodeteam.blockdevices"	"0.1.0+11"
"commit"="24167e6b3486c7938243c4a97fd5fb410390b8e5"
"formed"="Wed, 14 Feb 2018 18:40:13 UTC"
"semver"="0.1.0+11"
"url"="https://github.com/thecodeteam/csi-nfs"
```

Create a loopback device and make it available to plugin:

```bash
$ mkdir /dev/disk/csi-blockdevices
$ cd /dev/disk/csi-blockdevices

# make 100MiB disk image
$ dd if=/dev/zero of=test.img bs=1024 count=102400

# attach disk image to /dev/loop0
$ losetup /dev/loop0 test.img

# create symlink named loop0 -> /dev/loop0
$ ln -s /dev/loop0

$ csc -e csi.sock -v 0.1.0 c ls
"loop0"	0
```

Publish the "loop0" volume as a block volume

```bash
# create file to mount device to
$ touch /mnt/target
$ csc -v 0.1.0 n publish --cap SINGLE_NODE_WRITER,block --target-path /mnt/target loop0
loop0
$ mount | grep -e loop -e target
devtmpfs on /dev/disk/csi-bd-private/loop0 type devtmpfs (rw,relatime,seclabel,size=241476k,nr_inodes=60369,mode=755)
devtmpfs on /mnt/target type devtmpfs (rw,relatime,seclabel,size=241476k,nr_inodes=60369,mode=755) (rw,relatime,seclabel,size=241476k,nr_inodes=60369,mode=755)
$ csc -v 0.1.0 n unpublish --target-path /mnt/target loop0
loop0
$ mount | grep loop
$
```

Publish the "loop0" volume as a mount volume, formatted with ext4

```bash
# create directory to mount filesystem to
$ mkdir /mnt/test
$ csc -v 0.1.0 n publish --cap SINGLE_NODE_WRITER,mount,ext4 --target-path /mnt/test loop0
loop0
$ mount | grep loop
/dev/loop0 on /dev/disk/csi-bd-private/loop0 type ext4 (rw,relatime,seclabel,data=ordered)
/dev/loop0 on /mnt/test type ext4 (rw,relatime,seclabel,data=ordered)
$ csc -v 0.1.0 n unpublish --target-path /mnt/target loop0
loop0
$ mount | grep loop
$
```

### Parameters
No additional parameters are currently supported/required by the plugin

## Configuration
The CSI-BlockDevices SP is built using the GoCSI CSP package. Please see its
[configuration section](https://github.com/thecodeteam/gocsi#configuration)
for a complete list of the environment variables that may be used to
configure this SP.

The following table is a list of this SP's default configuration values:

| Name | Value |
|------|-------|
| `X_CSI_SPEC_REQ_VALIDATION` | `true` |
| `X_CSI_SERIAL_VOL_ACCESS` | `true` |
| `X_CSI_SUPPORTED_VERSIONS` | `0.1.0` |
| `X_CSI_PRIVATE_MOUNT_DIR` | `/dev/disk/csi-bd-private` |

The following table is a list of configuration values that are specific
to BlockDevices, their default values, and whether they are required for operation:

| Name | Description | Default Val | Required |
|------|-------------|-------------|----------|
| `X_CSI_BD_DEVDIR` | Directory to scan for block devices | `/dev/disk/csi-blockdevices` | `false` |

## Capable operational modes
The CSI spec defines a set of AccessModes that a volume can have. CSI-BlockDevices
supports the following modes for volumes that will be mounted as a filesystem:

```
// Can only be published once as read/write on a single node,
// at any given time.
SINGLE_NODE_WRITER = 1;

// Can only be published once as readonly on a single node,
// at any given time.
SINGLE_NODE_READER_ONLY = 2;
```

This means that mount volumes can be mounted only at one node at a time (because
the disk is local to the node) and can be mounted as read-write or read-only.

For volumes that are used as block devices, only the following are supported:

```
// Can only be published once as read/write on a single node,
// at any given time.
SINGLE_NODE_WRITER = 1;
```

This means that giving a workload read-only access to a block device is not
supported.


## Support
For any questions or concerns please file an issue with the
[csi-blockdevices](https://github.com/thecodeteam/csi-blockdevices/issues)
project or join the Slack channel #project-rexray at codecommunity.slack.com.
