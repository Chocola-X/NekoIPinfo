package dbgen

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/cockroachdb/pebble/v2"
	json "github.com/goccy/go-json"
)

func BackupDatabase(dbPath, backupDir string) error {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil
	}

	timestamp := time.Now().Format("20060102_150405")
	backupPath := filepath.Join(backupDir, fmt.Sprintf("backup_%s", timestamp))

	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return fmt.Errorf("创建备份目录失败: %v", err)
	}

	err := filepath.Walk(dbPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(dbPath, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(backupPath, relPath)
		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()
		dstFile, err := os.Create(destPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()
		_, err = io.Copy(dstFile, srcFile)
		return err
	})

	if err != nil {
		return fmt.Errorf("备份失败: %v", err)
	}
	fmt.Printf(" 数据库已备份到: %s\n", backupPath)
	return nil
}

func RecordChange(clDB *pebble.DB, key [16]byte, action string, oldJSON, newJSON []byte) {
	if clDB == nil {
		return
	}
	ts := time.Now().UnixNano()
	clKey := make([]byte, 24)
	copy(clKey[:16], key[:])
	clKey[16] = byte(ts >> 56)
	clKey[17] = byte(ts >> 48)
	clKey[18] = byte(ts >> 40)
	clKey[19] = byte(ts >> 32)
	clKey[20] = byte(ts >> 24)
	clKey[21] = byte(ts >> 16)
	clKey[22] = byte(ts >> 8)
	clKey[23] = byte(ts)

	entry := ChangelogEntry{
		Timestamp: time.Now().Format(time.RFC3339Nano),
		Action:    action,
		OldJSON:   string(oldJSON),
		NewJSON:   string(newJSON),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	clDB.Set(clKey, data, pebble.NoSync)
}