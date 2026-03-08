package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Chocola-X/NekoIPinfo/internal/dbgen"
	"github.com/Chocola-X/NekoIPinfo/internal/logger"
	"github.com/cockroachdb/pebble/v2"
	_ "github.com/mattn/go-sqlite3"
	maxminddb "github.com/oschwald/maxminddb-golang"
)

func printHelp() {
	dbgen.NekoHeader("NekoIPinfo 数据库工具")
	fmt.Println()
	dbgen.Neko("用法:", dbgen.ColorMagenta)
	dbgen.Neko("  dbgen                                         自动检测并生成数据库", dbgen.ColorLavend)
	dbgen.Neko("  dbgen -input <City MMDB/CSV> [-asn <ASN MMDB>] 指定文件生成数据库", dbgen.ColorLavend)
	dbgen.Neko("  dbgen -sqlite <旧版SQLite数据库>               从 SQLite 迁移", dbgen.ColorLavend)
	dbgen.Neko("  dbgen -update                                  自动检测并增量更新", dbgen.ColorLavend)
	dbgen.Neko("  dbgen -update -overwrite                       自动检测并覆盖更新", dbgen.ColorLavend)
	dbgen.Neko("  dbgen -input <City MMDB> -update               用 City 库单独更新", dbgen.ColorLavend)
	dbgen.Neko("  dbgen -asn <ASN MMDB> -update                  用 ASN 库单独更新 ISP", dbgen.ColorLavend)
	dbgen.Neko("  dbgen -input <City> -asn <ASN> -update         同时更新全部字段", dbgen.ColorLavend)
	dbgen.Neko("  dbgen -dump                                    导出数据库统计信息", dbgen.ColorLavend)
	dbgen.Neko("  dbgen -dump -csv <output.csv>                  导出为 CSV", dbgen.ColorLavend)
	dbgen.Neko("  dbgen -dump -sqlite-out <output.db>            导出为 SQLite", dbgen.ColorLavend)
	dbgen.Neko("  dbgen -dump -sample 20                         预览前 N 条数据", dbgen.ColorLavend)
	dbgen.Neko("  dbgen -logdb <日志目录>                         查看日志统计", dbgen.ColorLavend)
	dbgen.Neko("  dbgen -logdb <日志目录> -logout <output.csv>    导出日志为 CSV", dbgen.ColorLavend)
	fmt.Println()
	dbgen.Neko("参数:", dbgen.ColorMagenta)
	dbgen.Neko("  -input      输入文件路径（.mmdb 或 .csv）", dbgen.ColorLavend)
	dbgen.Neko("  -asn        ASN 数据库路径（.mmdb，补充 ISP 信息）", dbgen.ColorLavend)
	dbgen.Neko("  -sqlite     旧版 SQLite 数据库路径", dbgen.ColorLavend)
	dbgen.Neko("  -out        输出 Pebble 数据库路径（默认 ip_info）", dbgen.ColorLavend)
	dbgen.Neko("  -update     增量更新模式", dbgen.ColorLavend)
	dbgen.Neko("  -overwrite  强制覆盖已有记录", dbgen.ColorLavend)
	dbgen.Neko("  -dump       导出/查看数据库", dbgen.ColorLavend)
	dbgen.Neko("  -csv        导出为 CSV 文件路径", dbgen.ColorLavend)
	dbgen.Neko("  -sqlite-out 导出为 SQLite 文件路径", dbgen.ColorLavend)
	dbgen.Neko("  -sample     预览前 N 条数据", dbgen.ColorLavend)
	dbgen.Neko("  -logdb      日志数据库目录路径", dbgen.ColorLavend)
	dbgen.Neko("  -logout     日志导出输出路径（.csv）", dbgen.ColorLavend)
	dbgen.Neko("  -no-color   禁用终端彩色输出", dbgen.ColorLavend)
	dbgen.Neko("  -help       显示帮助", dbgen.ColorLavend)
	fmt.Println()
	ym := dbgen.CurrentYearMonth()
	dbgen.Neko("数据源下载:", dbgen.ColorMagenta)
	dbgen.Nekof("  City: https://download.db-ip.com/free/dbip-city-lite-%s.mmdb.gz", dbgen.ColorRose, ym)
	dbgen.Nekof("  ASN:  https://download.db-ip.com/free/dbip-asn-lite-%s.mmdb.gz", dbgen.ColorRose, ym)
	fmt.Println()
	dbgen.Neko("示例:", dbgen.ColorMagenta)
	dbgen.Neko("  dbgen", dbgen.ColorLavend)
	dbgen.Neko("  dbgen -input dbip-city-lite-2026-03.mmdb -asn dbip-asn-lite-2026-03.mmdb", dbgen.ColorLavend)
	dbgen.Neko("  dbgen -asn dbip-asn-lite-2026-03.mmdb -update", dbgen.ColorLavend)
	dbgen.Neko("  dbgen -dump -csv export.csv", dbgen.ColorLavend)
	dbgen.Neko("  dbgen -dump -sqlite-out export.db", dbgen.ColorLavend)
	dbgen.Neko("  dbgen -dump -sample 10", dbgen.ColorLavend)
	dbgen.Neko("  dbgen -logdb ip_info_log", dbgen.ColorLavend)
	dbgen.Neko("  dbgen -logdb ip_info_log -logout logs.csv", dbgen.ColorLavend)
	fmt.Println()
}

func main() {
	inputPath := flag.String("input", "", "输入文件路径（.mmdb 或 .csv）")
	asnPath := flag.String("asn", "", "ASN 数据库路径（.mmdb）")
	sqlitePath := flag.String("sqlite", "", "旧版 SQLite 数据库路径")
	dbPath := flag.String("out", dbgen.DefaultDBPath, "输出 Pebble 数据库路径")
	updateMode := flag.Bool("update", false, "增量更新模式")
	overwriteFlag := flag.Bool("overwrite", false, "覆盖已有记录")
	dumpMode := flag.Bool("dump", false, "导出/查看数据库")
	csvOut := flag.String("csv", "", "导出为 CSV 文件路径")
	sqliteOut := flag.String("sqlite-out", "", "导出为 SQLite 文件路径")
	sampleN := flag.Int("sample", 0, "预览前 N 条数据")
	logDB := flag.String("logdb", "", "日志数据库目录路径")
	logOut := flag.String("logout", "", "日志导出输出路径（.csv）")
	noColor := flag.Bool("no-color", false, "禁用终端彩色输出")
	showHelp := flag.Bool("help", false, "显示帮助")
	flag.Parse()

	if *noColor {
		dbgen.SetColorEnabled(false)
	}

	if *showHelp {
		printHelp()
		os.Exit(0)
	}

	if *logDB != "" {
		runLogExport(*logDB, *logOut)
		return
	}

	if *dumpMode {
		runDumpMode(*dbPath, *csvOut, *sqliteOut, *sampleN)
		return
	}

	hasInput := *inputPath != ""
	hasASN := *asnPath != ""
	hasSqlite := *sqlitePath != ""
	isUpdate := *updateMode

	asnOnlyUpdate := hasASN && !hasInput && !hasSqlite && isUpdate

	cwd, _ := os.Getwd()
	var cityFile, asnFile string

	if asnOnlyUpdate {
		asnFile = *asnPath
		runASNOnlyUpdate(asnFile, *dbPath, *overwriteFlag)
		return
	}

	if !hasInput && !hasSqlite {
		cityFile, asnFile = dbgen.DetectMMDB(cwd)
		csvFile := dbgen.DetectCSV(cwd)

		if cityFile == "" && csvFile != "" {
			cityFile = csvFile
		}

		if hasASN && asnFile == "" {
			asnFile = *asnPath
		}

		if cityFile == "" {
			dbgen.NekoHeader("NekoIPinfo 数据库工具")
			fmt.Println()
			dbgen.Neko("未检测到本地 MMDB/CSV 数据库文件。", dbgen.ColorRose)
			fmt.Println()
			ym := dbgen.CurrentYearMonth()
			dbgen.Nekof("  City: https://download.db-ip.com/free/dbip-city-lite-%s.mmdb.gz", dbgen.ColorLavend, ym)
			dbgen.Nekof("  ASN:  https://download.db-ip.com/free/dbip-asn-lite-%s.mmdb.gz", dbgen.ColorLavend, ym)
			fmt.Println()

			if !dbgen.AskUser("是否自动下载 DB-IP City Lite 和 ASN Lite 数据库？") {
				dbgen.Neko("已取消。", dbgen.ColorDim)
				printHelp()
				os.Exit(0)
			}

			var err error
			cityFile, asnFile, err = dbgen.DownloadDBIPFiles(cwd)
			if err != nil {
				log.Fatalf("下载失败: %v", err)
			}
		} else {
			dbgen.NekoHeader("NekoIPinfo 数据库工具")
			dbgen.NekoKV("检测到 City 数据", cityFile)
			if asnFile != "" {
				dbgen.NekoKV("检测到 ASN 数据", asnFile)
			} else {
				dbgen.NekoWarn("ASN 数据: 未检测到（ISP 字段可能为空）")
			}
			fmt.Println()
		}
	} else if hasInput {
		cityFile = *inputPath
		if hasASN {
			asnFile = *asnPath
		}
	}

	runImport(cityFile, asnFile, *sqlitePath, *dbPath, isUpdate, *overwriteFlag, hasSqlite, hasInput, hasASN)
}

func runLogExport(logDBPath, logOutPath string) {
	if _, err := os.Stat(logDBPath); os.IsNotExist(err) {
		dbgen.NekoError(fmt.Sprintf("日志数据库不存在: %s", logDBPath))
		os.Exit(1)
	}

	if logOutPath == "" {
		count, err := logger.CountLogs(logDBPath)
		if err != nil {
			dbgen.NekoError(fmt.Sprintf("读取日志失败: %v", err))
			os.Exit(1)
		}
		dbgen.NekoHeader("日志数据库统计")
		dbgen.NekoKV("日志目录", logDBPath)
		dbgen.NekoKV("日志条数", fmt.Sprintf("%d", count))
		dbgen.NekoFooter()
		return
	}

	dbgen.NekoHeader("导出日志")
	dbgen.NekoKV("日志目录", logDBPath)
	dbgen.NekoKV("输出文件", logOutPath)
	fmt.Println()

	startTime := time.Now()
	count, err := logger.ExportLogsCSV(logDBPath, logOutPath)
	if err != nil {
		dbgen.NekoError(fmt.Sprintf("导出失败: %v", err))
		os.Exit(1)
	}
	elapsed := time.Since(startTime)

	fmt.Println()
	dbgen.NekoHeader("完成")
	dbgen.NekoKV("导出记录", fmt.Sprintf("%d 条", count))
	dbgen.NekoKV("总耗时", elapsed.Round(time.Millisecond).String())
	dbgen.NekoFooter()
}

func runDumpMode(dbPath, csvOut, sqliteOut string, sampleN int) {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		dbgen.NekoError(fmt.Sprintf("数据库不存在: %s", dbPath))
		os.Exit(1)
	}

	startTime := time.Now()

	if csvOut != "" {
		dbgen.NekoHeader("导出为 CSV")
		dbgen.NekoKV("数据库", dbPath)
		dbgen.NekoKV("输出", csvOut)
		fmt.Println()

		count, err := dbgen.DumpToCSV(dbPath, csvOut)
		if err != nil {
			dbgen.NekoError(fmt.Sprintf("导出失败: %v", err))
			os.Exit(1)
		}
		elapsed := time.Since(startTime)

		fmt.Println()
		dbgen.NekoHeader("完成")
		dbgen.NekoKV("导出记录", fmt.Sprintf("%d 条", count))
		dbgen.NekoKV("总耗时", elapsed.Round(time.Millisecond).String())
		dbgen.NekoFooter()
		return
	}

	if sqliteOut != "" {
		dbgen.NekoHeader("导出为 SQLite")
		dbgen.NekoKV("数据库", dbPath)
		dbgen.NekoKV("输出", sqliteOut)
		fmt.Println()

		count, err := dbgen.DumpToSQLite(dbPath, sqliteOut)
		if err != nil {
			dbgen.NekoError(fmt.Sprintf("导出失败: %v", err))
			os.Exit(1)
		}
		elapsed := time.Since(startTime)

		fmt.Println()
		dbgen.NekoHeader("完成")
		dbgen.NekoKV("导出记录", fmt.Sprintf("%d 条", count))
		dbgen.NekoKV("总耗时", elapsed.Round(time.Millisecond).String())
		dbgen.NekoFooter()
		return
	}

	if sampleN > 0 {
		if err := dbgen.DumpSample(dbPath, sampleN); err != nil {
			dbgen.NekoError(fmt.Sprintf("预览失败: %v", err))
			os.Exit(1)
		}
		return
	}

	if err := dbgen.DumpStats(dbPath); err != nil {
		dbgen.NekoError(fmt.Sprintf("统计失败: %v", err))
		os.Exit(1)
	}
}

func runASNOnlyUpdate(asnFile, dbPath string, overwriteFlag bool) {
	dbgen.NekoHeader("NekoIPinfo 数据库工具")
	dbgen.NekoKV("模式", "ASN 单独更新（仅更新 ISP 字段）")
	dbgen.NekoKV("ASN 来源", asnFile)
	dbgen.NekoKV("数据库", dbPath)
	if overwriteFlag {
		dbgen.NekoKV("更新策略", "覆盖")
	} else {
		dbgen.NekoKV("更新策略", "增量（已有相同值跳过）")
	}
	dbgen.NekoFooter()
	fmt.Println()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		dbgen.NekoError("目标数据库不存在，ASN 单独更新需要已有的数据库")
		os.Exit(1)
	}

	backupDir := filepath.Join(filepath.Dir(dbPath), dbgen.DefaultBackupDir)
	changelogDir := filepath.Join(filepath.Dir(dbPath), dbgen.DefaultChangelogDir)

	dbgen.Neko("正在备份现有数据库...", dbgen.ColorLavend)
	if err := dbgen.BackupDatabase(dbPath, backupDir); err != nil {
		dbgen.NekoError(fmt.Sprintf("备份失败: %v", err))
		os.Exit(1)
	}
	fmt.Println()

	pdb, err := dbgen.OpenPebbleForWrite(dbPath, false)
	if err != nil {
		dbgen.NekoError(fmt.Sprintf("打开数据库失败: %v", err))
		os.Exit(1)
	}

	clDB, err := dbgen.OpenChangelog(changelogDir)
	if err != nil {
		log.Printf("打开更新日志失败: %v", err)
		clDB = nil
	}

	startTime := time.Now()
	dbgen.Neko("正在更新 ISP 数据...", dbgen.ColorMagenta)
	count, skipped, err := dbgen.ImportIncrementalASN(asnFile, pdb, clDB, overwriteFlag)
	if err != nil {
		pdb.Close()
		if clDB != nil {
			clDB.Close()
		}
		dbgen.NekoError(fmt.Sprintf("更新失败: %v", err))
		os.Exit(1)
	}

	fmt.Println()
	dbgen.CompactAndClose(pdb)
	if clDB != nil {
		clDB.Close()
	}

	elapsed := time.Since(startTime)

	fmt.Println()
	dbgen.NekoHeader("完成")
	dbgen.NekoKV("更新记录", fmt.Sprintf("%d 条", count))
	dbgen.NekoKV("跳过记录", fmt.Sprintf("%d 条", skipped))
	dbgen.NekoKV("更新日志", changelogDir)
	dbgen.PrintDBSize(dbPath)
	dbgen.NekoKV("总耗时", elapsed.Round(time.Millisecond).String())
	dbgen.NekoFooter()
}

func runImport(cityFile, asnFile, sqlitePath, dbPath string, isUpdate, overwriteFlag, hasSqlite, hasInput, hasASN bool) {
	isNew := !isUpdate
	if isUpdate {
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			dbgen.NekoWarn("目标数据库不存在，将切换为全量新建模式。")
			isNew = true
			isUpdate = false
		}
	}

	var fieldsToUpdate []string
	if isUpdate && hasInput && !hasASN && !hasSqlite {
		fieldsToUpdate = []string{"country", "province", "city", "latitude", "longitude"}
		dbgen.Neko("  City 单独更新模式：仅更新地理位置字段（不覆盖 ISP）", dbgen.ColorLavend)
	}

	dbgen.NekoHeader("任务配置")
	if hasSqlite {
		dbgen.NekoKV("数据来源", fmt.Sprintf("SQLite 迁移 (%s)", sqlitePath))
	} else {
		dbgen.NekoKV("数据来源", cityFile)
		if asnFile != "" {
			dbgen.NekoKV("ASN 来源", asnFile)
		}
	}
	dbgen.NekoKV("输出路径", dbPath)
	if isUpdate {
		if overwriteFlag {
			dbgen.NekoKV("更新模式", "覆盖更新")
		} else {
			dbgen.NekoKV("更新模式", "增量更新")
		}
		if len(fieldsToUpdate) > 0 {
			dbgen.NekoKV("更新字段", strings.Join(fieldsToUpdate, ", "))
		}
	} else {
		dbgen.NekoKV("更新模式", "全量重建")
	}
	dbgen.NekoFooter()
	fmt.Println()

	backupDir := filepath.Join(filepath.Dir(dbPath), dbgen.DefaultBackupDir)
	changelogDir := filepath.Join(filepath.Dir(dbPath), dbgen.DefaultChangelogDir)

	if isUpdate {
		dbgen.Neko("正在备份现有数据库...", dbgen.ColorLavend)
		if err := dbgen.BackupDatabase(dbPath, backupDir); err != nil {
			dbgen.NekoError(fmt.Sprintf("备份失败: %v", err))
			os.Exit(1)
		}
		fmt.Println()
	}

	pdb, err := dbgen.OpenPebbleForWrite(dbPath, isNew)
	if err != nil {
		dbgen.NekoError(fmt.Sprintf("打开数据库失败: %v", err))
		os.Exit(1)
	}

	var clDB *pebble.DB
	if isUpdate {
		clDB, err = dbgen.OpenChangelog(changelogDir)
		if err != nil {
			log.Printf("打开更新日志失败: %v", err)
			clDB = nil
		}
	}

	var asnDB *maxminddb.Reader
	if asnFile != "" {
		asnDB, err = maxminddb.Open(asnFile)
		if err != nil {
			log.Printf("打开 ASN 数据库失败: %v", err)
		} else {
			defer asnDB.Close()
			dbgen.NekoSuccess("已加载 ASN 数据库")
		}
	}

	startTime := time.Now()
	var count, skipped int64

	dbgen.Neko("正在导入数据...", dbgen.ColorMagenta)
	fmt.Println()

	if hasSqlite {
		count, skipped, err = dbgen.ImportSQLite(sqlitePath, pdb, clDB, asnDB, isUpdate, overwriteFlag, fieldsToUpdate)
	} else if isUpdate && !hasSqlite {
		ext := strings.ToLower(filepath.Ext(cityFile))
		switch ext {
		case ".mmdb":
			count, skipped, err = dbgen.ImportIncrementalMMDB(cityFile, pdb, clDB, asnDB, overwriteFlag, fieldsToUpdate)
		case ".csv":
			count, skipped, err = dbgen.ImportCSV(cityFile, pdb, clDB, asnDB, isUpdate, overwriteFlag, fieldsToUpdate)
		default:
			pdb.Close()
			if clDB != nil {
				clDB.Close()
			}
			dbgen.NekoError(fmt.Sprintf("不支持的文件格式: %s", ext))
			os.Exit(1)
		}
	} else {
		ext := strings.ToLower(filepath.Ext(cityFile))
		switch ext {
		case ".mmdb":
			count, skipped, err = dbgen.ImportMMDB(cityFile, pdb, clDB, asnDB, false, overwriteFlag, fieldsToUpdate)
		case ".csv":
			count, skipped, err = dbgen.ImportCSV(cityFile, pdb, clDB, asnDB, false, overwriteFlag, fieldsToUpdate)
		default:
			pdb.Close()
			if clDB != nil {
				clDB.Close()
			}
			dbgen.NekoError(fmt.Sprintf("不支持的文件格式: %s", ext))
			os.Exit(1)
		}
	}

	if err != nil {
		pdb.Close()
		if clDB != nil {
			clDB.Close()
		}
		dbgen.NekoError(fmt.Sprintf("导入失败: %v", err))
		os.Exit(1)
	}

	fmt.Println()
	dbgen.CompactAndClose(pdb)
	if clDB != nil {
		clDB.Close()
	}

	elapsed := time.Since(startTime)

	fmt.Println()
	dbgen.NekoHeader("完成")
	if isUpdate {
		dbgen.NekoKV("更新记录", fmt.Sprintf("%d 条", count))
		dbgen.NekoKV("跳过记录", fmt.Sprintf("%d 条", skipped))
		dbgen.NekoKV("更新日志", changelogDir)
	} else {
		dbgen.NekoKV("导入记录", fmt.Sprintf("%d 条（IPv4 + IPv6）", count))
		dbgen.NekoKV("跳过记录", fmt.Sprintf("%d 条", skipped))
	}
	dbgen.PrintDBSize(dbPath)
	dbgen.NekoKV("总耗时", elapsed.Round(time.Millisecond).String())
	dbgen.NekoKV("输出路径", dbPath)
	dbgen.NekoFooter()
}