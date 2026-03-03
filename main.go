package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

// 定义完全匹配前端需求的数据结构
type IPInfo struct {
	IP        string `json:"ip"`
	Country   string `json:"country"`
	Province  string `json:"province"`
	City      string `json:"city"`
	ISP       string `json:"isp"`
	Latitude  string `json:"latitude"`
	Longitude string `json:"longitude"`
}

// 定义标准的 API 响应格式
type APIResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"` // 用 interface{} 以便在出错时返回 nil
}

// 辅助函数：获取客户端真实 IP
func getClientIP(r *http.Request) string {
	ip := r.Header.Get("X-Real-IP")
	if ip == "" {
		ip = r.Header.Get("X-Forwarded-For")
	}
	if ip == "" {
		ip, _, _ = net.SplitHostPort(r.RemoteAddr)
	}
	// X-Forwarded-For 可能是逗号分隔的多个IP，取第一个真实的
	ip = strings.Split(ip, ",")[0]
	// 清理可能包含的空格，提升健壮性
	return strings.TrimSpace(ip)
}

func apiHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 提取查询参数并去除两端空白字符
	queryIPStr := strings.TrimSpace(r.URL.Query().Get("ip"))

	targetIP := queryIPStr
	if targetIP == "" {
		targetIP = getClientIP(r)
	}

	// 【安全防线 1】：严格限制为合法的 IPv4 格式。任何注入代码都会在这里被直接拦截。
	parsedIP := net.ParseIP(targetIP)
	if parsedIP == nil || parsedIP.To4() == nil {
		json.NewEncoder(w).Encode(APIResponse{Code: 400, Msg: "非法的 IPv4 地址喵！", Data: nil})
		return
	}

	// 【安全防线 2】：转为 uint32 整型。彻底杜绝字符型注入。
	ipInt := uint32(parsedIP.To4()[0])<<24 | uint32(parsedIP.To4()[1])<<16 | uint32(parsedIP.To4()[2])<<8 | uint32(parsedIP.To4()[3])

	var infoJSON string
	var networkEnd uint32 // 🌟 记得提前声明这个变量喵

	// 【性能优化核心】：利用索引极速向下查找最近的一条记录
	err := db.QueryRow(`
	SELECT network_end, ip_info_json
	FROM ip_info
	WHERE network_start <= ?
	ORDER BY network_start DESC
	LIMIT 1`, ipInt).Scan(&networkEnd, &infoJSON)

	if err != nil {
		if err == sql.ErrNoRows {
			json.NewEncoder(w).Encode(APIResponse{Code: 404, Msg: "数据库里没有找到这个 IP 喵~", Data: nil})
		} else {
			// 避免将底层的数据库错误直接暴露给前端（防止信息泄露）
			log.Printf("数据库查询错误: %v\n", err)
			json.NewEncoder(w).Encode(APIResponse{Code: 500, Msg: "数据库查询出错了喵", Data: nil})
		}
		return
	}

	// 【关键逻辑校验】：虽然找到了最近的起始 IP，但必须确认目标 IP 是否在这个网段的覆盖范围内
	if ipInt > networkEnd {
		json.NewEncoder(w).Encode(APIResponse{Code: 404, Msg: "数据库里没有找到这个 IP 喵~", Data: nil})
		return
	}

	var rawData map[string]string
	if err := json.Unmarshal([]byte(infoJSON), &rawData); err != nil {
		log.Printf("JSON 解析错误: %v\n", err)
		json.NewEncoder(w).Encode(APIResponse{Code: 500, Msg: "数据解析失败喵", Data: nil})
		return
	}

	result := IPInfo{
		IP:        targetIP,
		Country:   rawData["country"],
		Province:  rawData["province"],
		City:      rawData["city"],
		ISP:       rawData["isp"],
		Latitude:  rawData["latitude"],
		Longitude: rawData["longitude"],
	}

	json.NewEncoder(w).Encode(APIResponse{
		Code: 200,
		Msg:  "success",
		Data: result,
	})
}

func main() {
	// ================= 定义启动参数 =================
	// flag.String("参数名", "默认值", "说明文字")
	dbPath := flag.String("db", "ip_info.db", "SQLite 数据库文件路径")
	port := flag.String("port", "8080", "API 服务监听端口")

	// 解析命令行参数
	flag.Parse()

	var err error
	// 使用参数指定的数据库路径
	db, err = sql.Open("sqlite3", *dbPath)
	if err != nil {
		log.Fatal("无法打开数据库: ", err)
	}
	defer db.Close()

	http.HandleFunc("/ipinfo", apiHandler)

	// 拼接监听地址
	addr := fmt.Sprintf(":%s", *port)
	fmt.Printf("猫娘纯净 API 服务启动于 %s 喵... 🐾\n", addr)
	fmt.Printf("当前使用的数据库文件: %s\n", *dbPath)

	log.Fatal(http.ListenAndServe(addr, nil))
}
