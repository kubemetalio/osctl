package disk

import (
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"

	"github.com/huweihuang/osctl/pkg/osctl/types"
	"github.com/huweihuang/osctl/pkg/util"
)

const nbd0Device = "nbd0"

func (o *DiskOptions) InitRootDisk() error {
	klog.V(4).Info("init root disk begin")

	// rootDisk like sda
	rootDisk, rootNum, err := getRootDeviceNameAndNum(&o.Template.Raids[0])
	if err != nil {
		return errors.Wrapf(err, "Failed to get root device name and number")
	}

	if err := o.cleanRootDisk(rootDisk); err != nil {
		return errors.Wrapf(err, "Failed to clean root disk")
	}

	if err := writeImageToDisk(rootDisk, o.OSIImageFile); err != nil {
		return errors.Wrapf(err, "Failed to write image to disk")
	}

	if err := fixPartitionTable(rootDisk); err != nil {
		return errors.Wrapf(err, "Failed to fix partition table")
	}

	if swapSize, exists := o.Template.SysDisk["SWAP"]; exists && swapSize != "" {
		if err := o.handleSwapPartition(rootDisk, rootNum); err != nil {
			return errors.Wrapf(err, "Failed to handle swap partition")
		}
	} else {
		if err := o.handleRootPartition(rootDisk, rootNum); err != nil {
			return errors.Wrapf(err, "Failed to handle root partition")
		}
	}

	if err := resizeAndCheckFileSystem(rootDisk); err != nil {
		fmt.Println("Failed to resize and check file system:", err)
		return nil
	}

	klog.V(4).Info("init root disk end")
	return nil
}

func (o *DiskOptions) cleanRootDisk(rootDisk string) error {
	if err := unmountDisk(rootDisk); err != nil {
		return errors.Wrapf(err, "Failed to unmount root disk")
	}

	if err := deletePartitions(rootDisk); err != nil {
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

func (o *DiskOptions) handleRootPartition(rootDisk string, rootNum int) error {
	rootSize := o.Template.SysDisk["/"]
	if rootSize != "rest" {
		rootStartOutput, err := util.RunCommand(fmt.Sprintf("parted %s unit MiB print | awk '/./{start=$2} END{print start}'", rootDisk))
		if err != nil {
			return err
		}
		rootStart, _ := strconv.Atoi(rootStartOutput[:len(rootStartOutput)-3])
		_, err = util.RunCommand(fmt.Sprintf("echo Yes | parted ---pretend-input-tty %s -- resizepart %s %dMiB", rootDisk, rootNum, 1+rootStart+ToMiB(rootSize)))
		if err != nil {
			return err
		}

		freeEnd, err := freeEnd(rootDisk)
		if err != nil {
			return err
		}

		_, err = util.RunCommand(fmt.Sprintf("echo Ignore | parted ---pretend-input-tty %s -- mkpart primary %dMiB 100%%", rootDisk, freeEnd))
		if err != nil {
			return err
		}

		err = makeFS(o.FileSystemType, mergeDeviceName(rootDisk, rootNum+1))
		if err != nil {
			return err
		}
	} else {
		_, err := util.RunCommand(fmt.Sprintf("echo Yes | parted ---pretend-input-tty %s -- resizepart %s 100%%", rootDisk, rootNum))
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *DiskOptions) handleSwapPartition(rootDisk string, rootNum int) error {
	freeEnd, err := freeEnd(rootDisk)
	if err != nil {
		return err
	}

	// rm root part
	_, err = util.RunCommand(fmt.Sprintf("echo Ignore | parted ---pretend-input-tty %s rm %d", rootDisk, rootNum))
	if err != nil {
		return err
	}

	// swap
	swapSize := o.Template.SysDisk["SWAP"]
	_, err = util.RunCommand(fmt.Sprintf("echo Ignore | parted ---pretend-input-tty %s -- mkpart primary %dMiB %dMiB", rootDisk, freeEnd, freeEnd+ToMiB(swapSize)))
	if err != nil {
		return err
	}

	_, err = util.RunCommand(fmt.Sprintf("mkswap %s", mergeDeviceName(rootDisk, rootNum)))
	if err != nil {
		return err
	}

	rootNum++

	// root
	rootSize := o.Template.SysDisk["/"]
	if rootSize != "rest" {
		_, err = util.RunCommand(fmt.Sprintf("echo Ignore | parted ---pretend-input-tty %s -- mkpart primary %dMiB %dMiB", rootDisk, freeEnd, 1+freeEnd+ToMiB(rootSize)))
		if err != nil {
			return err
		}
		_, err = util.RunCommand(fmt.Sprintf("echo Ignore | parted ---pretend-input-tty %s -- mkpart primary %dMiB 100%%", rootDisk, freeEnd))
		if err != nil {
			return err
		}
		err = makeFS(o.FileSystemType, mergeDeviceName(rootDisk, rootNum+1))
		if err != nil {
			return err
		}
	} else {
		_, err = util.RunCommand(fmt.Sprintf("echo Ignore | parted ---pretend-input-tty %s -- mkpart primary %dMiB 100%%", rootDisk, freeEnd))
		if err != nil {
			return err
		}
	}

	err = o.handleNbdDevice(mergeDeviceName(rootDisk, rootNum))
	if err != nil {
		return err
	}

	return err
}

func (o *DiskOptions) handleNbdDevice(rootDevice string) error {
	// mount image root
	_, err := util.RunCommand("modprobe nbd nbds_max=1")
	if err != nil {
		return err
	}

	_, err = util.RunCommand("qemu-nbd --disconnect /dev/nbd0 || true")
	if err != nil {
		return err
	}

	_, err = util.RunCommand(fmt.Sprintf("qemu-nbd --connect /dev/nbd0 %s", o.OSIImageFile))
	if err != nil {
		return err
	}

	time.Sleep(2 * time.Second)

	_, err = util.RunCommand("partprobe /dev/nbd0")
	if err != nil {
		return err
	}

	// copy image root to disk root, cat is always faster than dd for whole disk copy.
	nbdDev := getDeviceFullName("nbd0")
	_, err = util.RunCommand(fmt.Sprintf("cat %s >%s", nbdDev, rootDevice))
	if err != nil {
		return err
	}
	// umount image root
	_, err = util.RunCommand("qemu-nbd --disconnect /dev/nbd0")
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

func freeEnd(rootDisk string) (int, error) {
	output, err := util.RunCommand(fmt.Sprintf("parted %s unit MiB print free | grep 'Free Space' | tail -1 | awk '{print $1}'", rootDisk))
	if err != nil {
		return 0, err
	}
	freeEnd, _ := strconv.Atoi(output[:len(output)-3])
	return freeEnd, nil
}

func unmountDisk(rootDisk string) error {
	_, err := util.RunCommand(fmt.Sprintf("cat /proc/mounts | grep ^/dev/%s | awk '{print $2}' | xargs -n1 -i umount {}", rootDisk))
	return err
}

func deletePartitions(rootDisk string) error {
	_, err := util.RunCommand(fmt.Sprintf("sfdisk --delete %s || true", rootDisk))
	return err
}

func writeImageToDisk(rootDisk, osiImageFile string) error {
	_, err := util.RunCommand(fmt.Sprintf("qemu-img dd -f qcow2 -O raw bs=16M if=%s of=%s", rootDisk, osiImageFile))
	return err
}

func fixPartitionTable(rootDisk string) error {
	_, err := util.RunCommand(fmt.Sprintf("echo Fix | parted ---pretend-input-tty %s print", rootDisk))
	return err
}

func resizeAndCheckFileSystem(rootDisk string) error {
	rootDev := getDeviceFullName(rootDisk)
	_, err := util.RunCommand(fmt.Sprintf("e2fsck -y -f %s && resize2fs %s || true", rootDev, rootDev))
	return err
}
