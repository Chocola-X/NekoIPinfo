package main

import (
	"database/sql"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	maxminddb "github.com/oschwald/maxminddb-golang"
)

type mmdbRecord struct {
	Country struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
	Subdivisions []struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"subdivisions"`
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
	} `maxminddb:"location"`
	Traits struct {
		ISP          string `maxminddb:"isp"`
		Organization string `maxminddb:"organization"`
	} `maxminddb:"traits"`
}

type asnRecord struct {
	AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
	ISP                          string `maxminddb:"isp"`
	Organization                 string `maxminddb:"organization"`
}

func getName(names map[string]string) string {
	if v, ok := names["zh-CN"]; ok && v != "" {
		return v
	}
	if v, ok := names["en"]; ok && v != "" {
		return v
	}
	for _, v := range names {
		if v != "" {
			return v
		}
	}
	return ""
}

func lastIPInNetwork(network *net.IPNet) net.IP {
	ip := network.IP.To16()
	if ip == nil {
		return nil
	}
	mask := network.Mask
	if len(mask) == 4 {
		m := make(net.IPMask, 16)
		copy(m[:12], []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
		copy(m[12:], mask)
		mask = m
	}
	broadcast := make(net.IP, 16)
	for i := 0; i < 16; i++ {
		broadcast[i] = ip[i] | ^mask[i]
	}
	return broadcast
}

func lookupASN(asnDB *maxminddb.Reader, ip net.IP) string {
	if asnDB == nil {
		return ""
	}
	var record asnRecord
	if err := asnDB.Lookup(ip, &record); err != nil {
		return ""
	}
	if record.ISP != "" {
		return record.ISP
	}
	if record.Organization != "" {
		return record.Organization
	}
	if record.AutonomousSystemOrganization != "" {
		return record.AutonomousSystemOrganization
	}
	return ""
}

func floatToStr(f float64) string {
	if f == 0 {
		return ""
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.6f", f), "0"), ".")
}

func isIPv4Mapped(ip net.IP) bool {
	b := ip.To16()
	if b == nil {
		return false
	}
	for i := 0; i < 10; i++ {
		if b[i] != 0 {
			return false
		}
	}
	return b[10] == 0xff && b[11] == 0xff
}

func main() {
	inputPath := flag.String("input", "", "City MMDB 文件路径")
	asnPath := flag.String("asn", "", "ASN MMDB 文件路径")
	outPath := flag.String("out", "ip_info.db", "输出 SQLite 数据库路径")
	flag.Parse()

	if *inputPath == "" {
		fmt.Println("用法: mb2db -input <City.mmdb> [-asn <ASN.mmdb>] [-out ip_info.db]")
		os.Exit(1)
	}

	reader, err := maxminddb.Open(*inputPath)
	if err != nil {
		log.Fatalf("无法打开 MMDB: %v", err)
	}
	defer reader.Close()

	var asnDB *maxminddb.Reader
	if *asnPath != "" {
		asnDB, err = maxminddb.Open(*asnPath)
		if err != nil {
			log.Printf("打开 ASN MMDB 失败: %v", err)
		} else {
			defer asnDB.Close()
			fmt.Println("已加载 ASN 数据库")
		}
	}

	os.Remove(*outPath)
	sqlDB, err := sql.Open("sqlite3", *outPath+"?_journal_mode=OFF&_synchronous=OFF")
	if err != nil {
		log.Fatalf("创建 SQLite 数据库失败: %v", err)
	}
	defer sqlDB.Close()

	sqlDB.Exec("PRAGMA page_size=16384")

	_, err = sqlDB.Exec(`CREATE TABLE IF NOT EXISTS ip_info (
		network_start INTEGER NOT NULL,
		network_end INTEGER NOT NULL,
		ip_info_json TEXT NOT NULL DEFAULT '{}'
	)`)
	if err != nil {
		log.Fatalf("创建 IPv4 表失败: %v", err)
	}

	_, err = sqlDB.Exec(`CREATE TABLE IF NOT EXISTS ip_info_v6 (
		network_start_hi INTEGER NOT NULL,
		network_start_lo INTEGER NOT NULL,
		network_end_hi INTEGER NOT NULL,
		network_end_lo INTEGER NOT NULL,
		ip_info_json TEXT NOT NULL DEFAULT '{}'
	)`)
	if err != nil {
		log.Fatalf("创建 IPv6 表失败: %v", err)
	}

	tx, err := sqlDB.Begin()
	if err != nil {
		log.Fatalf("开始事务失败: %v", err)
	}

	stmtV4, _ := tx.Prepare(`INSERT INTO ip_info (network_start, network_end, ip_info_json) VALUES (?,?,?)`)
	stmtV6, _ := tx.Prepare(`INSERT INTO ip_info_v6 (network_start_hi, network_start_lo, network_end_hi, network_end_lo, ip_info_json) VALUES (?,?,?,?,?)`)

	startTime := time.Now()
	var v4Count, v6Count, skipped int64

	networks := reader.Networks(maxminddb.SkipAliasedNetworks)
	for networks.Next() {
		var record mmdbRecord
		network, err := networks.Network(&record)
		if err != nil {
			skipped++
			continue
		}

		startIP := network.IP.To16()
		if startIP == nil {
			skipped++
			continue
		}

		endIP := lastIPInNetwork(network)
		if endIP == nil {
			skipped++
			continue
		}

		country := getName(record.Country.Names)
		province := ""
		if len(record.Subdivisions) > 0 {
			province = getName(record.Subdivisions[0].Names)
		}
		city := getName(record.City.Names)

		isp := record.Traits.ISP
		if isp == "" {
			isp = record.Traits.Organization
		}
		if isp == "" {
			isp = lookupASN(asnDB, network.IP)
		}

		latitude := floatToStr(record.Location.Latitude)
		longitude := floatToStr(record.Location.Longitude)

		if country == "" && province == "" && city == "" &&
			math.Abs(record.Location.Latitude) < 0.0001 &&
			math.Abs(record.Location.Longitude) < 0.0001 {
			skipped++
			continue
		}

		jsonStr := fmt.Sprintf(`{"country":"%s","province":"%s","city":"%s","isp":"%s","latitude":"%s","longitude":"%s"}`,
			country, province, city, isp, latitude, longitude)

		if isIPv4Mapped(startIP) {
			startU32 := uint32(startIP[12])<<24 | uint32(startIP[13])<<16 | uint32(startIP[14])<<8 | uint32(startIP[15])
			end16 := endIP.To16()
			endU32 := uint32(end16[12])<<24 | uint32(end16[13])<<16 | uint32(end16[14])<<8 | uint32(end16[15])
			stmtV4.Exec(int64(startU32), int64(endU32), jsonStr)
			v4Count++
		} else {
			startHi := int64(binary.BigEndian.Uint64(startIP[:8]))
			startLo := int64(binary.BigEndian.Uint64(startIP[8:16]))
			end16 := endIP.To16()
			endHi := int64(binary.BigEndian.Uint64(end16[:8]))
			endLo := int64(binary.BigEndian.Uint64(end16[8:16]))
			stmtV6.Exec(startHi, startLo, endHi, endLo, jsonStr)
			v6Count++
		}

		total := v4Count + v6Count
		if total%50000 == 0 {
			stmtV4.Close()
			stmtV6.Close()
			tx.Commit()
			tx, _ = sqlDB.Begin()
			stmtV4, _ = tx.Prepare(`INSERT INTO ip_info (network_start, network_end, ip_info_json) VALUES (?,?,?)`)
			stmtV6, _ = tx.Prepare(`INSERT INTO ip_info_v6 (network_start_hi, network_start_lo, network_end_hi, network_end_lo, ip_info_json) VALUES (?,?,?,?,?)`)
			fmt.Printf("\r  已处理 %d 条（IPv4: %d, IPv6: %d）...", total, v4Count, v6Count)
		}
	}

	stmtV4.Close()
	stmtV6.Close()
	tx.Commit()

	fmt.Println()
	fmt.Println("正在创建索引...")
	sqlDB.Exec("CREATE INDEX IF NOT EXISTS idx_v4_start ON ip_info(network_start)")
	sqlDB.Exec("CREATE INDEX IF NOT EXISTS idx_v6_start ON ip_info_v6(network_start_hi, network_start_lo)")

	elapsed := time.Since(startTime)
	fmt.Println()
	fmt.Println("========================================")
	fmt.Printf("  IPv4 记录: %d\n", v4Count)
	fmt.Printf("  IPv6 记录: %d\n", v6Count)
	fmt.Printf("  跳过记录: %d\n", skipped)
	fmt.Printf("  输出文件: %s\n", *outPath)
	fmt.Printf("  总耗时: %v\n", elapsed.Round(time.Millisecond))
	fmt.Println("========================================")
}