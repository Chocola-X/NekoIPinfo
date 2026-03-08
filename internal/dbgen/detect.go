package dbgen

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func CurrentYearMonth() string {
	now := time.Now()
	return fmt.Sprintf("%d-%02d", now.Year(), now.Month())
}

func AskUser(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [y/N]: ", prompt)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}

func DetectMMDB(dir string) (cityPath, asnPath string) {
	cityPatterns := []string{"dbip-city-lite-*.mmdb", "GeoLite2-City.mmdb"}
	asnPatterns := []string{"dbip-asn-lite-*.mmdb", "GeoLite2-ASN.mmdb"}

	for _, pattern := range cityPatterns {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err == nil && len(matches) > 0 && cityPath == "" {
			cityPath = matches[len(matches)-1]
		}
	}
	for _, pattern := range asnPatterns {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err == nil && len(matches) > 0 && asnPath == "" {
			asnPath = matches[len(matches)-1]
		}
	}
	return
}

func DetectCSV(dir string) string {
	patterns := []string{"dbip-city-lite-*.csv", "dbip-city-lite-*.csv.gz"}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err == nil && len(matches) > 0 {
			return matches[len(matches)-1]
		}
	}
	return ""
}

func DownloadFile(url, destPath string) error {
	fmt.Printf(" 正在下载: %s\n", url)

	client := &http.Client{
		Timeout: 10 * time.Minute,
	}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("下载失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer out.Close()

	var reader io.Reader = resp.Body
	if strings.HasSuffix(url, ".gz") {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return fmt.Errorf("gzip 解压失败: %v", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	written, err := io.Copy(out, reader)
	if err != nil {
		return fmt.Errorf("写入失败: %v", err)
	}
	fmt.Printf(" 下载完成: %s (%.2f MB)\n", destPath, float64(written)/1024/1024)
	return nil
}

func DownloadDBIPFiles(dir string) (cityPath, asnPath string, err error) {
	ym := CurrentYearMonth()
	cityURL := fmt.Sprintf(DbipCityURLTemplate, ym)
	asnURL := fmt.Sprintf(DbipASNURLTemplate, ym)
	cityFile := filepath.Join(dir, fmt.Sprintf("dbip-city-lite-%s.mmdb", ym))
	asnFile := filepath.Join(dir, fmt.Sprintf("dbip-asn-lite-%s.mmdb", ym))

	fmt.Println()
	fmt.Println("正在下载 DB-IP 数据库...")
	if err := DownloadFile(cityURL, cityFile); err != nil {
		return "", "", fmt.Errorf("下载 City 数据库失败: %v", err)
	}
	if err := DownloadFile(asnURL, asnFile); err != nil {
		return "", "", fmt.Errorf("下载 ASN 数据库失败: %v", err)
	}
	fmt.Println()
	return cityFile, asnFile, nil
}