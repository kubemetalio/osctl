package disk

import (
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"

	"github.com/kubemetalio/osctl/pkg/osctl/types"
	"github.com/kubemetalio/osctl/pkg/util"
)

const nbd0Device = "/dev/nbd0"

func (o *DiskOptions) InitRootDisk() error {
	klog.V(4).Info("init root disk begin")

	// rootDeviceName like sda
	rootDeviceName, rootNum, err := getRootDeviceNameAndNum(&o.Template.Raids[0])
	if err != nil {
		return errors.Wrapf(err, "Failed to get root device name and number")
	}
	// /dev/sda
	rootDev := getDeviceFullName(rootDeviceName)
	if err := o.cleanRootDisk(rootDev); err != nil {
		return errors.Wrapf(err, "Failed to clean root disk")
	}

	if err := writeImageToDisk(rootDev, o.OSIImageFile); err != nil {
		return errors.Wrapf(err, "Failed to write image to disk")
	}

	if err := fixPartitionTable(rootDev); err != nil {
		return errors.Wrapf(err, "Failed to fix partition table")
	}

	if swapSize, exists := o.Template.SysDisk["SWAP"]; exists && swapSize != "" {
		if err := o.handleSwapPartition(rootDev, rootNum); err != nil {
			return errors.Wrapf(err, "Failed to handle swap partition")
		}
	} else {
		if err := o.handleRootPartition(rootDev, rootNum); err != nil {
			return errors.Wrapf(err, "Failed to handle root partition")
		}
	}

	if err := resizeAndCheckFileSystem(rootDev); err != nil {
		return errors.Wrapf(err, "Failed to resize and check file system")
	}

	klog.V(4).Info("init root disk end")
	return nil
}

func (o *DiskOptions) cleanRootDisk(rootDev string) error {
	if err := unmountDisk(rootDev); err != nil {
		return errors.Wrapf(err, "Failed to unmount root disk")
	}

	if err := deletePartitions(rootDev); err != nil {
		return errors.Wrapf(err, "Failed to delete partitions")
	}

	if err := unmountDisk(nbd0Device); err != nil {
		return errors.Wrapf(err, "Failed to unmount nbd0 disk")
	}

	if _, err := util.RunCommand("qemu-nbd --disconnect /dev/nbd0 || true"); err != nil {
		return err
	}

	return nil
}

func (o *DiskOptions) handleRootPartition(rootDev string, rootNum int) error {
	rootSize := o.Template.SysDisk["/"]
	if rootSize != "rest" {
		rootStartOutput, err := util.RunCommand(fmt.Sprintf("parted %s unit MiB print | awk '/./{start=$2} END{print start}'", rootDev))
		if err != nil {
			return err
		}
		rootStart, _ := strconv.Atoi(rootStartOutput[:len(rootStartOutput)-3])
		_, err = util.RunCommand(fmt.Sprintf("echo Yes | parted ---pretend-input-tty %s -- resizepart %s %dMiB", rootDev, rootNum, 1+rootStart+ToMiB(rootSize)))
		if err != nil {
			return err
		}

		freeEnd, err := freeEnd(rootDev)
		if err != nil {
			return err
		}

		_, err = util.RunCommand(fmt.Sprintf("echo Ignore | parted ---pretend-input-tty %s -- mkpart primary %dMiB 100%%", rootDev, freeEnd))
		if err != nil {
			return err
		}

		// like mkfs.ext4 -F /dev/sda4
		err = makeFS(o.FileSystemType, mergeDeviceName(rootDev, rootNum+1))
		if err != nil {
			return err
		}
	} else {
		_, err := util.RunCommand(fmt.Sprintf("echo Yes | parted ---pretend-input-tty %s -- resizepart %s 100%%", rootDev, rootNum))
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *DiskOptions) handleSwapPartition(rootDev string, rootNum int) error {
	freeEnd, err := freeEnd(rootDev)
	if err != nil {
		return err
	}

	// rm root part
	_, err = util.RunCommand(fmt.Sprintf("echo Ignore | parted ---pretend-input-tty %s rm %d", rootDev, rootNum))
	if err != nil {
		return err
	}

	// swap
	swapSize := o.Template.SysDisk["SWAP"]
	_, err = util.RunCommand(fmt.Sprintf("echo Ignore | parted ---pretend-input-tty %s -- mkpart primary %dMiB %dMiB", rootDev, freeEnd, freeEnd+ToMiB(swapSize)))
	if err != nil {
		return err
	}

	_, err = util.RunCommand(fmt.Sprintf("mkswap %s", mergeDeviceName(rootDev, rootNum)))
	if err != nil {
		return err
	}

	rootNum++

	// root
	rootSize := o.Template.SysDisk["/"]
	if rootSize != "rest" {
		_, err = util.RunCommand(fmt.Sprintf("echo Ignore | parted ---pretend-input-tty %s -- mkpart primary %dMiB %dMiB", rootDev, freeEnd, 1+freeEnd+ToMiB(rootSize)))
		if err != nil {
			return err
		}
		_, err = util.RunCommand(fmt.Sprintf("echo Ignore | parted ---pretend-input-tty %s -- mkpart primary %dMiB 100%%", rootDev, freeEnd))
		if err != nil {
			return err
		}
		err = makeFS(o.FileSystemType, mergeDeviceName(rootDev, rootNum+1))
		if err != nil {
			return err
		}
	} else {
		_, err = util.RunCommand(fmt.Sprintf("echo Ignore | parted ---pretend-input-tty %s -- mkpart primary %dMiB 100%%", rootDev, freeEnd))
		if err != nil {
			return err
		}
	}

	// /dev/sda4
	err = o.handleNbdDevice(mergeDeviceName(rootDev, rootNum))
	if err != nil {
		return err
	}

	return err
}

func (o *DiskOptions) handleNbdDevice(rootDevice string) error {
	// mount image root
	if _, err := util.RunCommand("modprobe nbd nbds_max=1"); err != nil {
		return err
	}

	if _, err := util.RunCommand("qemu-nbd --disconnect /dev/nbd0 || true"); err != nil {
		return err
	}

	if _, err := util.RunCommand(fmt.Sprintf("qemu-nbd --connect /dev/nbd0 %s", o.OSIImageFile)); err != nil {
		return err
	}

	time.Sleep(2 * time.Second)

	if _, err := util.RunCommand("partprobe /dev/nbd0"); err != nil {
		return err
	}

	// copy image root to disk root, cat is always faster than dd for whole disk copy.
	if _, err := util.RunCommand(fmt.Sprintf("cat %s >%s", "/dev/nbd0", rootDevice)); err != nil {
		return err
	}
	// umount image root
	if _, err := util.RunCommand("qemu-nbd --disconnect /dev/nbd0"); err != nil {
		return err
	}
	return nil
}

// getRootDeviceNameAndNum get the device name like sda
func getRootDeviceNameAndNum(raid *types.Raid) (string, int, error) {
	rootSize := getRootSize(raid)
	rootDevice, err := getRootDeviceAttribute(rootSize)
	if err != nil {
		return "", 0, err
	}
	deviceName, num, err := splitDeviceName(rootDevice.Name)
	if err != nil {
		return "", 0, err
	}
	return deviceName, num, nil
}

// getRootDeviceAttribute determine the root partition by finding the partition with the label 'root'.
// If none is found, then look for the partition whose size is greater than 5GB and whose file system type is not 'swap'.
func getRootDeviceAttribute(size int64) (*Device, error) {
	rootDevice, err := getDiskDeviceBySize(size)
	if err != nil {
		return nil, err
	}
	if len(rootDevice.Children) == 0 {
		return nil, errors.New("no root disk detected")
	}

	// find the partition with the label 'root'
	for _, part := range rootDevice.Children {
		if part.Label == "root" {
			return &part, nil
		}
	}

	// find which size > 5G and fstype != swap
	for _, part := range rootDevice.Children {
		if part.Size > 5*1024*1024*1024 && part.FStype != "swap" {
			return &part, nil
		}
	}

	return nil, errors.New("no root disk detected")
}

func getRootSize(raid *types.Raid) int64 {
	raidMultiplier := getRaidMultiplier(raid.RaidLevel, raid.RaidMembers)
	return ToBytes(raid.DiskSize) * raidMultiplier
}

// getRaidMultiplier returns the multiplier based on the RAID level
func getRaidMultiplier(raidLevel string, raidMembers int64) int64 {
	switch raidLevel {
	case "noRaid", "R1":
		return 1
	case "R0":
		return raidMembers
	case "R3", "R5":
		return raidMembers - 1
	case "R6":
		return raidMembers - 2
	case "R10":
		return raidMembers / 2
	default:
		return 1
	}
}

func makeFS(fsType string, device string) error {
	switch fsType {
	case FileSystemTypeEXT4:
		_, err := util.RunCommand(fmt.Sprintf("mkfs.ext4 -F %s", device))
		if err != nil {
			return err
		}
	case FileSystemTypeXFS:
		_, err := util.RunCommand(fmt.Sprintf("mkfs.xfs -f -n ftype=1 %s", device))
		if err != nil {
			return err
		}
	}
	return nil
}

func freeEnd(rootDev string) (int, error) {
	output, err := util.RunCommand(fmt.Sprintf("parted %s unit MiB print free | grep 'Free Space' | tail -1 | awk '{print $1}'", rootDev))
	if err != nil {
		return 0, err
	}
	freeEnd, _ := strconv.Atoi(output[:len(output)-3])
	return freeEnd, nil
}

func unmountDisk(rootDev string) error {
	_, err := util.RunCommand(fmt.Sprintf("cat /proc/mounts | grep ^/%s | awk '{print $2}' | xargs -n1 -i umount {}", rootDev))
	return err
}

func deletePartitions(rootDev string) error {
	_, err := util.RunCommand(fmt.Sprintf("sfdisk --delete %s || true", rootDev))
	return err
}

func writeImageToDisk(rootDev, osiImageFile string) error {
	_, err := util.RunCommand(fmt.Sprintf("qemu-img dd -f qcow2 -O raw bs=16M if=%s of=%s", rootDev, osiImageFile))
	return err
}

func fixPartitionTable(rootDev string) error {
	_, err := util.RunCommand(fmt.Sprintf("echo Fix | parted ---pretend-input-tty %s print", rootDev))
	return err
}

func resizeAndCheckFileSystem(rootDev string) error {
	_, err := util.RunCommand(fmt.Sprintf("e2fsck -y -f %s && resize2fs %s || true", rootDev, rootDev))
	return err
}
