package raid

import (
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

var RaidCmd = &cobra.Command{
	Use:   "raid",
	Short: "init raid setting",
}

func init() {
	RaidCmd.AddCommand(NewInitCmd())
}

func NewInitCmd() *cobra.Command {
	option := NewRaidOptions()
	cmd := &cobra.Command{
		Use:   "init",
		Short: "init raid setting",
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

func (o *RaidOptions) RunInit() error {
	return nil
}
