package utils

import (
	"bytes"
	"context"
	"os/exec"
	"time"
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

// RunCmdTimeout 加入 timeout 版本，避免外部程式卡住。
func RunCmdTimeout(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return stderr.String(), context.DeadlineExceeded
	}
	if err != nil {
		return stderr.String(), err
	}
	if stderr.Len() > 0 {
		return stderr.String(), nil
	}
	return out.String(), nil
}
