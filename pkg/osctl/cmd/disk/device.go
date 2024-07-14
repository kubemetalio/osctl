package disk

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/huweihuang/osctl/pkg/osctl/types"
	"github.com/huweihuang/osctl/pkg/util"
)

type BlockDevices struct {
	BlockDevices []BlockDevice `json:"blockdevices"`
}

type BlockDevice struct {
	Device
	Children []Device `json:"children,omitempty"`
}

type Device struct {
	Name       string `json:"name"`
	Label      string `json:"label"`
	Size       int64  `json:"size"` // unit: byte
	FStype     string `json:"fstype"`
	UUID       string `json:"uuid"`
	Type       string `json:"type"` // type: disk
	MountPoint string `json:"mountpoint"`
	Rota       bool   `json:"rota"` // rotational, true: HDD, false: SSD
}

// getDiskDevices execute command lsblk to get disk device
func getDiskDevices(device string) ([]BlockDevice, error) {
	var block BlockDevices
	cmd := fmt.Sprintf("lsblk -J -b -o NAME,LABEL,SIZE,FSTYPE,UUID,TYPE,MOUNTPOINT,ROTA")
	if device != "" {
		cmd = fmt.Sprintf("%s /dev/%s", cmd, device)
	}
	output, err := util.RunCommand(cmd)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(output), &block)
	if err != nil {
		return nil, err
	}

	disks := make([]BlockDevice, 0)
	for _, device := range block.BlockDevices {
		if device.Type == "disk" {
			disks = append(disks, device)
		}
	}

	if len(disks) == 0 {
		return nil, errors.New("no disk devices found")
	}
	return disks, nil
}

// GetRootDeviceNameAndNum get the device name like sda
func GetRootDeviceNameAndNum(raid *types.Raid) (string, int, error) {
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

func getDeviceNumber(deviceName string) (int, error) {
	return strconv.Atoi(regexp.MustCompile(`\d+$`).FindString(deviceName))
}

// sda1 => sda + 1; nvme0n1p1 => nvme0n1 + 1;
func splitDeviceName(deviceName string) (string, int, error) {
	re := regexp.MustCompile(`^(.*?)(\d+)$`)
	matches := re.FindStringSubmatch(deviceName)
	if len(matches) != 3 {
		return deviceName, 0, nil
	}

	num, err := strconv.Atoi(matches[2])
	if err != nil {
		return deviceName, 0, err
	}
	return matches[1], num, nil
}

// nvme0n1 + 1 => nvme0n1p1; sda + 1 => sda1
func mergeDeviceName(rootDisk string, num int) string {
	if strings.HasPrefix(rootDisk, "nvme") || strings.HasPrefix(rootDisk, "nbd") {
		return fmt.Sprintf("%sp%d", rootDisk, num)
	}
	return fmt.Sprintf("%s%d", rootDisk, num)
}

func getDeviceFullName(deviceName string) string {
	return fmt.Sprintf("/dev/%s", deviceName)
}

// getRootDeviceAttribute determine the root partition by finding the partition with the label 'root'.
// If none is found, then look for the partition whose size is greater than 5GB and whose file system type is not 'swap'.
func getRootDeviceAttribute(size int64) (*Device, error) {
	rootDevice, err := getRootParentDevice(size)
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

func getRootParentDevice(size int64) (*BlockDevice, error) {
	disks, err := getDiskDevices("")
	if err != nil {
		return nil, err
	}
	for _, disk := range disks {
		if isDiskSizeEqual(disk.Size, size) {
			return &disk, nil
		}
	}
	for _, disk := range disks {
		if disk.Size > 0 {
			return &disk, nil
		}
	}
	return nil, errors.Errorf("no disk detected: %v", disks)
}

func isDiskSizeEqual(first, second int64) bool {
	difference := math.Abs(float64(first - second))
	return (difference / float64(first)) < 0.05
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
