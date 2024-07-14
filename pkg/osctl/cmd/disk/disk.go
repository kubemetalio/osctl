package disk

import (
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

var DiskCmd = &cobra.Command{
	Use:   "disk",
	Short: "init root disk and data disk",
}

func init() {
	DiskCmd.AddCommand(NewInitCmd())
}

func NewInitCmd() *cobra.Command {
	option := NewDiskOptions()
	cmd := &cobra.Command{
		Use:   "init",
		Short: "init root disk and data disk",
		Run: func(cmd *cobra.Command, _ []string) {
			if err := option.Validate(); err != nil {
				klog.Errorf("init option is invalid: %w", err)
				return
			}
			if err := option.Complete(); err != nil {
				klog.Errorf("fail to complete the init option: %w", err)
				return
			}
			if err := option.RunInit(); err != nil {
				klog.Errorf("fail to init disk: %w", err)
				return
			}
		},
	}

	fs := cmd.Flags()
	option.AddFlags(fs)
	return cmd
}

func (o *DiskOptions) RunInit() error {
	switch o.DiskType {
	case RootDiskType:
		if err := o.InitRootDisk(); err != nil {
			return err
		}
	case DataDiskType:
		if err := o.InitDataDisk(); err != nil {
			return err
		}
	default:
		if err := o.InitRootDisk(); err != nil {
			return err
		}
		if err := o.InitDataDisk(); err != nil {
			return err
		}
	}
	return nil
}
