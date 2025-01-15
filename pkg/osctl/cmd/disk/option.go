package disk

import (
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/kubemetalio/osctl/pkg/osctl/types"
)

const (
	RootDiskType = "root"
	DataDiskType = "data"

	FileSystemTypeEXT4 = "ext4"
	FileSystemTypeXFS  = "xfs"
)

type DiskOptions struct {
	DiskType       string
	FileSystemType string
	OSIImageFile   string
	Template       types.Template
}

func NewDiskOptions() *DiskOptions {
	return &DiskOptions{}
}

func (o *DiskOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.DiskType, "type", "", "disk type: root disk or data disk")
	fs.StringVar(&o.FileSystemType, "file-system", FileSystemTypeEXT4, "file system type")
	fs.StringVar(&o.OSIImageFile, "image", "osi.qcow2", "osi image file name")
}

func (o *DiskOptions) Validate() error {
	allErrs := field.ErrorList{}
	return allErrs.ToAggregate()
}

func (o *DiskOptions) Complete() (err error) {
	return nil
}
