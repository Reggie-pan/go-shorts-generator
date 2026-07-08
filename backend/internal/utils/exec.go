package utils

import (
	"bytes"
	"context"
	"os/exec"
	"time"
)

// RunCmdContext 執行外部命令並支持 Context 取消 (D1)
func RunCmdContext(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return stderr.String(), ctx.Err()
	}
	if err != nil {
		return stderr.String(), err
	}
	if stderr.Len() > 0 {
		return stderr.String(), nil
	}
	return out.String(), nil
}

// RunCmdTimeoutContext 加入 timeout 與 Context 版本 (D1)
func RunCmdTimeoutContext(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error) {
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(tctx, name, args...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if tctx.Err() == context.DeadlineExceeded {
		return stderr.String(), context.DeadlineExceeded
	}
	if ctx.Err() != nil {
		return stderr.String(), ctx.Err()
	}
	if err != nil {
		return stderr.String(), err
	}
	if stderr.Len() > 0 {
		return stderr.String(), nil
	}
	return out.String(), nil
}

// RunCmd 執行外部命令並返回輸出，便於除錯
func RunCmd(name string, args ...string) (string, error) {
	return RunCmdContext(context.Background(), name, args...)
}

// RunCmdTimeout 加入 timeout 版本，避免外部程式卡住
func RunCmdTimeout(timeout time.Duration, name string, args ...string) (string, error) {
	return RunCmdTimeoutContext(context.Background(), timeout, name, args...)
}
