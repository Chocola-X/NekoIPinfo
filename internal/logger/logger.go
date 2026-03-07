package logger

import (
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	json "github.com/goccy/go-json"

	"github.com/Chocola-X/NekoIPinfo/internal/model"
	"github.com/cockroachdb/pebble/v2"
)

type AsyncLogger struct {
	pdb        *pebble.DB
	console    bool
	days       int
	seq        atomic.Uint64
	batch      *pebble.Batch
	batchCount int
	mu         sync.Mutex
	stopCh     chan struct{}
	flushTick  *time.Ticker
}

func New(logDir string, days int) (*AsyncLogger, error) {
	al := &AsyncLogger{
		days:   days,
		stopCh: make(chan struct{}),
	}

	if days == -2 {
		return nil, nil
	}

	if days == -1 {
		al.console = true
		return al, nil
	}

	os.MkdirAll(logDir, 0755)
	opts := &pebble.Options{
		BytesPerSync:    512 << 10,
		DisableWAL:      false,
		FormatMajorVersion: pebble.FormatNewest,
	}
	pdb, err := pebble.Open(logDir, opts)
	if err != nil {
		return nil, fmt.Errorf("打开日志数据库失败: %v", err)
	}

	al.pdb = pdb
	al.console = true
	al.batch = pdb.NewBatch()

	al.flushTick = time.NewTicker(2 * time.Second)
	go al.flushLoop()

	if days > 0 {
		go al.cleanupLoop()
	}

	return al, nil
}

func (a *AsyncLogger) Log(clientIP, queryIP string, code int, country, province, city, isp string, latencyUs int64) {
	if a == nil {
		return
	}

	now := time.Now()

	if a.console {
		fmt.Fprintf(os.Stderr, " [%s] %s -> %s | %d | %s %s %s | %s | %dµs\n",
			now.Format("15:04:05.000"),
			clientIP, queryIP, code,
			country, province, city, isp, latencyUs)
	}

	if a.pdb == nil {
		return
	}

	entry := model.AccessLog{
		Timestamp: now.UnixMicro(),
		ClientIP:  clientIP,
		QueryIP:   queryIP,
		Code:      code,
		Country:   country,
		Province:  province,
		City:      city,
		ISP:       isp,
		LatencyUs: latencyUs,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	seq := a.seq.Add(1)

	var key [12]byte
	binary.BigEndian.PutUint64(key[:8], uint64(now.UnixMicro()))
	binary.BigEndian.PutUint32(key[8:12], uint32(seq))

	a.mu.Lock()
	a.batch.Set(key[:], data, nil)
	a.batchCount++
	if a.batchCount >= 500 {
		a.batch.Commit(pebble.NoSync)
		a.batch.Reset()
		a.batchCount = 0
	}
	a.mu.Unlock()
}

func (a *AsyncLogger) flushLoop() {
	for {
		select {
		case <-a.stopCh:
			return
		case <-a.flushTick.C:
			a.flush()
		}
	}
}

func (a *AsyncLogger) flush() {
	if a.pdb == nil {
		return
	}
	a.mu.Lock()
	if a.batchCount > 0 {
		a.batch.Commit(pebble.NoSync)
		a.batch.Reset()
		a.batchCount = 0
	}
	a.mu.Unlock()
}

func (a *AsyncLogger) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	a.cleanup()

	for {
		select {
		case <-a.stopCh:
			return
		case <-ticker.C:
			a.cleanup()
		}
	}
}

func (a *AsyncLogger) cleanup() {
	if a.pdb == nil || a.days <= 0 {
		return
	}
	cutoff := time.Now().Add(-time.Duration(a.days) * 24 * time.Hour).UnixMicro()

	var upperKey [12]byte
	binary.BigEndian.PutUint64(upperKey[:8], uint64(cutoff))

	a.pdb.DeleteRange(make([]byte, 12), upperKey[:], pebble.NoSync)
}

func (a *AsyncLogger) Close() {
	if a == nil {
		return
	}
	close(a.stopCh)
	if a.flushTick != nil {
		a.flushTick.Stop()
	}
	a.flush()
	if a.pdb != nil {
		a.batch.Close()
		a.pdb.Close()
	}
}

func CountLogs(logDir string) (int64, error) {
	opts := &pebble.Options{ReadOnly: true}
	pdb, err := pebble.Open(logDir, opts)
	if err != nil {
		return 0, err
	}
	defer pdb.Close()

	iter, err := pdb.NewIter(nil)
	if err != nil {
		return 0, err
	}
	defer iter.Close()

	var count int64
	for valid := iter.First(); valid; valid = iter.Next() {
		count++
	}
	return count, nil
}

func ExportLogsCSV(logDir, outputPath string) (int64, error) {
	opts := &pebble.Options{ReadOnly: true}
	pdb, err := pebble.Open(logDir, opts)
	if err != nil {
		return 0, err
	}
	defer pdb.Close()

	outFile, err := os.Create(outputPath)
	if err != nil {
		return 0, err
	}
	defer outFile.Close()

	fmt.Fprintln(outFile, "timestamp,client_ip,query_ip,code,country,province,city,isp,latency_us")

	iter, err := pdb.NewIter(nil)
	if err != nil {
		return 0, err
	}
	defer iter.Close()

	var count int64
	for valid := iter.First(); valid; valid = iter.Next() {
		valBytes, valErr := iter.ValueAndErr()
		if valErr != nil {
			continue
		}
		var entry model.AccessLog
		if err := json.Unmarshal(valBytes, &entry); err != nil {
			continue
		}
		ts := time.UnixMicro(entry.Timestamp).Format(time.RFC3339Nano)
		fmt.Fprintf(outFile, "%s,%s,%s,%d,%s,%s,%s,%s,%d\n",
			ts, entry.ClientIP, entry.QueryIP, entry.Code,
			entry.Country, entry.Province, entry.City, entry.ISP, entry.LatencyUs)
		count++
	}

	return count, nil
}