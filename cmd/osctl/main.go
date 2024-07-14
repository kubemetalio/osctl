package main

import (
	"os"

	"k8s.io/component-base/logs"

	"github.com/huweihuang/osctl/cmd/osctl/app"
)

const (
	bashCompleteFile = "/etc/bash_completion.d/osctl.bash_complete"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	command := app.NewOSCtlCommand()
	command.GenBashCompletionFile(bashCompleteFile)
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
