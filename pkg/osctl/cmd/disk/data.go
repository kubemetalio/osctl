package disk

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"

	"github.com/huweihuang/osctl/pkg/util"
)

func (o *DiskOptions) InitDataDisk() error {
	klog.V(4).Info("init data disk begin")
	dataDisk, err := o.listDataDiskDeviceName()
	if err != nil {
		return err
	}
	for _, diskName := range dataDisk {
		klog.V(4).Infof("format data disk [%s]", diskName)
		err = o.formatDataDisk(diskName)
		if err != nil {
			return errors.Wrapf(err, "failed to format data disk: %s", diskName)
		}
	}
	klog.V(4).Info("init data disk end")
	return nil
}

func (o *DiskOptions) listDataDiskDeviceName() ([]string, error) {
	data := make([]string, 0)
	disks, err := listDiskDevices("")
	if err != nil {
		return nil, err
	}
	rootDeviceName, _, err := getRootDeviceNameAndNum(&o.Template.Raids[0])
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get root device name and number")
	}
	for _, disk := range disks {
		if disk.Name != rootDeviceName && !strings.HasPrefix(disk.Name, "nbd") {
			data = append(data, getDeviceFullName(disk.Name))
		}
	}
	return data, nil
}

func (o *DiskOptions) formatDataDisk(device string) error {
	_, err := util.RunCommand(fmt.Sprintf("sfdisk --delete %s; echo label:gpt | sfdisk %s", device, device))
	if err != nil {
		return err
	}

	_, err = util.RunCommand(fmt.Sprintf("partprobe %s", device))
	if err != nil {
		return err
	}

	err = makeFS(o.FileSystemType, device)
	return err
}
