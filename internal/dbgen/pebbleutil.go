package dbgen

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/cockroachdb/pebble/v2"
)

func OpenPebbleForWrite(dbPath string, isNew bool) (*pebble.DB, error) {
	if isNew {
		os.RemoveAll(dbPath)
	}
	opts := &pebble.Options{
		BytesPerSync:       512 << 10,
		DisableWAL:         true,
		FormatMajorVersion: pebble.FormatNewest,
	}
	if !isNew {
		opts.ErrorIfNotExists = true
	}
	return pebble.Open(dbPath, opts)
}

func OpenPebbleReadOnly(dbPath string) (*pebble.DB, error) {
	opts := &pebble.Options{
		ReadOnly: true,
	}
	return pebble.Open(dbPath, opts)
}

func OpenChangelog(changelogDir string) (*pebble.DB, error) {
	os.MkdirAll(changelogDir, 0755)
	opts := &pebble.Options{
		BytesPerSync:       512 << 10,
		FormatMajorVersion: pebble.FormatNewest,
	}
	return pebble.Open(changelogDir, opts)
}

func CompactAndClose(pdb *pebble.DB) {
	Neko("正在压缩数据库...", ColorLavend)
	if err := pdb.Compact(nil, bytes.Repeat([]byte{0xff}, 17), false); err != nil {
		log.Printf("压缩警告: %v", err)
	}
	pdb.Close()
}

func PrintDBSize(dbPath string) {
	var totalSize int64
	filepath.Walk(dbPath, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})
	Neko(fmt.Sprintf(" 数据库大小: %.2f MB", float64(totalSize)/1024/1024), ColorPink)
}

func CommitBatch(batch *pebble.Batch, count int64) error {
	if err := batch.Commit(pebble.NoSync); err != nil {
		return fmt.Errorf("提交批次失败: %v", err)
	}
	batch.Reset()
	Nekof(" 已处理 %d 条记录...", ColorDim, count)
	return nil
}