package disk

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"

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

// listDiskDevices execute command lsblk to get disk device
func listDiskDevices(device string) ([]BlockDevice, error) {
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
		if device.Type == "disk" && device.Size != 0 {
			disks = append(disks, device)
		}
	}

	if len(disks) == 0 {
		return nil, errors.New("no disk devices found")
	}
	return disks, nil
}

func getDiskDeviceBySize(size int64) (*BlockDevice, error) {
	disks, err := listDiskDevices("")
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

// /dev/nvme0n1 + 1 => /dev/nvme0n1p1; /dev/sda + 1 => /dev/sda1
func mergeDeviceName(deviceName string, num int) string {
	if strings.Contains(deviceName, "nvme") || strings.Contains(deviceName, "nbd") {
		return fmt.Sprintf("%sp%d", deviceName, num)
	}
	return fmt.Sprintf("%s%d", deviceName, num)
}

// sdb => /dev/sdb
func getDeviceFullName(deviceName string) string {
	return fmt.Sprintf("/dev/%s", deviceName)
}

func isDiskSizeEqual(first, second int64) bool {
	difference := math.Abs(float64(first - second))
	return (difference / float64(first)) < 0.05
}
