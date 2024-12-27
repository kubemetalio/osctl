package app

import (
	"flag"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/version/verflag"

	"github.com/huweihuang/osctl/pkg/osctl/cmd/disk"
	"github.com/huweihuang/osctl/pkg/osctl/cmd/raid"
)

func NewOSCtlCommand() *cobra.Command {
	cmds := &cobra.Command{
		Use:   "osctl",
		Short: "osctl setup disk os",
		Run: func(cmd *cobra.Command, _ []string) {
			verflag.PrintAndExitIfRequested()
			cliflag.PrintFlags(cmd.Flags())
			cmd.Help()
		},
	}

	// raid, disk, network,
	cmds.AddCommand(disk.DiskCmd)
	cmds.AddCommand(raid.RaidCmd)

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Set("logtostderr", "false")
	pflag.Set("alsologtostderr", "true")
	pflag.Set("log_file", fmt.Sprintf("%s/osctl.log", "/tmp"))

	return cmds
}
