package util

import (
	"fmt"
	"os/exec"
	"strings"

	"k8s.io/klog/v2"
)

func RunCommand(cmd string) (string, error) {
	klog.V(4).Infof("Exec command [%s]", cmd)
	output, err := exec.Command("/bin/sh", "-c", cmd).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("exec [%s], ouput: %s, err: %v", cmd, string(output), err)
	}
	result := strings.TrimSuffix(string(output), "\n")
	return result, nil
}
