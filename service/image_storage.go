package service

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

const (
	imageStorageDir     = "/data/images"
	imageCleanupMaxAge  = 24 * time.Hour
	imageCleanupInterval = 1 * time.Hour
)

// SaveBase64ToLocal 将 base64 图片数据保存到本地文件，返回相对路径
func SaveBase64ToLocal(base64Data string) (string, error) {
	// 去除 data URI 前缀
	if idx := strings.Index(base64Data, ","); idx != -1 {
		base64Data = base64Data[idx+1:]
	}

	decoded, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	// 检测图片格式
	ext := detectImageExt(decoded)

	// 按日期分目录
	now := time.Now()
	dateDir := now.Format("2006/01/02")
	dirPath := filepath.Join(imageStorageDir, dateDir)

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// 生成唯一文件名
	filename := fmt.Sprintf("%s%s", common.GetUUID(), ext)
	filePath := filepath.Join(dirPath, filename)

	if err := os.WriteFile(filePath, decoded, 0644); err != nil {
		return "", fmt.Errorf("failed to write image file: %w", err)
	}

	// 返回相对路径
	relativePath := fmt.Sprintf("/images/%s/%s", dateDir, filename)
	return relativePath, nil
}

// GetImageURL 拼接完整的图片访问 URL
func GetImageURL(relativePath string) string {
	serverAddr := strings.TrimRight(system_setting.ServerAddress, "/")
	if serverAddr == "" {
		serverAddr = "http://localhost:3000"
	}
	return serverAddr + relativePath
}

// detectImageExt 根据文件头检测图片格式
func detectImageExt(data []byte) string {
	if len(data) < 4 {
		return ".png"
	}
	// PNG
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return ".png"
	}
	// JPEG
	if data[0] == 0xFF && data[1] == 0xD8 {
		return ".jpg"
	}
	// WebP
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return ".webp"
	}
	// GIF
	if string(data[0:3]) == "GIF" {
		return ".gif"
	}
	return ".png"
}

// StartImageCleanup 启动定期清理过期图片的 goroutine
func StartImageCleanup() {
	go func() {
		ticker := time.NewTicker(imageCleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			cleanupExpiredImages()
		}
	}()
	common.SysLog("image storage cleanup task started")
}

func cleanupExpiredImages() {
	cutoff := time.Now().Add(-imageCleanupMaxAge)
	_ = filepath.Walk(imageStorageDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(path)
		}
		return nil
	})

	// 清理空目录
	_ = filepath.Walk(imageStorageDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() || path == imageStorageDir {
			return nil
		}
		entries, _ := os.ReadDir(path)
		if len(entries) == 0 {
			os.Remove(path)
		}
		return nil
	})
}
