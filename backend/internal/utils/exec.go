package utils

import (
	"bytes"
	"os/exec"
)

// RunCmd 執行外部命令並回傳輸出，便於除錯。
func RunCmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return stderr.String(), err
	}
	if stderr.Len() > 0 {
		return stderr.String(), nil
	}
	return out.String(), nil
}
