package utils

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// CopyFile copies a file from src to dst. If dst already exists, it will be overwritten.
func CopyFile(src, dst string) error {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return os.ErrInvalid
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	if _, err := io.Copy(destination, source); err != nil {
		return err
	}
	return nil
}

// DownloadFile downloads a file from urlStr and saves it to destPath with a 30s timeout.
func DownloadFile(urlStr, destPath string) error {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Get(urlStr)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下載失敗，HTTP 狀態碼: %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
