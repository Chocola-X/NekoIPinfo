package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
)

type roundResult struct {
	Concurrency int
	Total       int64
	Success     int64
	Fail        int64
	QPS         float64
	AvgLatNs    int64
	MinLatNs    int64
	MaxLatNs    int64
	FirstErr    string
}

func fmtNum(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var buf []byte
	mod := len(s) % 3
	if mod > 0 {
		buf = append(buf, s[:mod]...)
	}
	for i := mod; i < len(s); i += 3 {
		if len(buf) > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, s[i:i+3]...)
	}
	return string(buf)
}

func fmtDur(ns int64) string {
	if ns <= 0 {
		return "0"
	}
	us := float64(ns) / 1000
	if us < 1000 {
		return fmt.Sprintf("%.0fµs", us)
	}
	ms := us / 1000
	if ms < 1000 {
		return fmt.Sprintf("%.2fms", ms)
	}
	return fmt.Sprintf("%.2fs", ms/1000)
}

func runRound(host string, apiPath string, concurrency int, dur time.Duration, useRandom bool, fixedIP string, showLive bool) roundResult {
	client := &fasthttp.Client{
		MaxConnsPerHost:     concurrency * 2,
		MaxIdleConnDuration: 30 * time.Second,
		ReadTimeout:         5 * time.Second,
		WriteTimeout:        5 * time.Second,
	}

	var totalReqs, succReqs, failReqs, totalLatNs int64
	var minLatNs int64 = int64(time.Hour)
	var maxLatNs int64
	var firstError sync.Once
	var firstErrMsg string

	baseURLBytes := []byte(host + apiPath + "?ip=")
	fixedIPBytes := []byte(fixedIP)

	stopCh := make(chan struct{})
	var wg sync.WaitGroup
	startTime := time.Now()

	go func() {
		time.Sleep(dur)
		close(stopCh)
	}()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			localRng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id)*7919))
			urlBuf := make([]byte, 0, len(baseURLBytes)+16)
			var fixedURLBuf []byte
			if !useRandom {
				fixedURLBuf = append(fixedURLBuf, baseURLBytes...)
				fixedURLBuf = append(fixedURLBuf, fixedIPBytes...)
			}

			req := fasthttp.AcquireRequest()
			resp := fasthttp.AcquireResponse()
			defer fasthttp.ReleaseRequest(req)
			defer fasthttp.ReleaseResponse(resp)

			for {
				select {
				case <-stopCh:
					return
				default:
				}

				req.Reset()
				resp.Reset()

				if useRandom {
					a := localRng.Intn(223) + 1
					b := localRng.Intn(256)
					c := localRng.Intn(256)
					d := localRng.Intn(256)
					urlBuf = append(urlBuf[:0], baseURLBytes...)
					urlBuf = strconv.AppendInt(urlBuf, int64(a), 10)
					urlBuf = append(urlBuf, '.')
					urlBuf = strconv.AppendInt(urlBuf, int64(b), 10)
					urlBuf = append(urlBuf, '.')
					urlBuf = strconv.AppendInt(urlBuf, int64(c), 10)
					urlBuf = append(urlBuf, '.')
					urlBuf = strconv.AppendInt(urlBuf, int64(d), 10)
					req.SetRequestURIBytes(urlBuf)
				} else {
					req.SetRequestURIBytes(fixedURLBuf)
				}

				req.Header.SetMethod(fasthttp.MethodGet)

				reqStart := time.Now()
				doErr := client.Do(req, resp)
				latNs := int64(time.Since(reqStart))

				atomic.AddInt64(&totalReqs, 1)
				if doErr != nil {
					atomic.AddInt64(&failReqs, 1)
					firstError.Do(func() {
						firstErrMsg = fmt.Sprintf("连接错误: %v", doErr)
					})
					continue
				}

				httpCode := resp.StatusCode()
				if httpCode != fasthttp.StatusOK {
					atomic.AddInt64(&failReqs, 1)
					firstError.Do(func() {
						firstErrMsg = fmt.Sprintf("HTTP %d: %s", httpCode, string(resp.Body()))
					})
					continue
				}

				atomic.AddInt64(&succReqs, 1)
				atomic.AddInt64(&totalLatNs, latNs)

				for {
					old := atomic.LoadInt64(&minLatNs)
					if latNs >= old || atomic.CompareAndSwapInt64(&minLatNs, old, latNs) {
						break
					}
				}
				for {
					old := atomic.LoadInt64(&maxLatNs)
					if latNs <= old || atomic.CompareAndSwapInt64(&maxLatNs, old, latNs) {
						break
					}
				}
			}
		}(i)
	}

	if showLive {
		ticker := time.NewTicker(1 * time.Second)
		go func() {
			lastTotal := int64(0)
			for {
				select {
				case <-stopCh:
					ticker.Stop()
					return
				case <-ticker.C:
					current := atomic.LoadInt64(&totalReqs)
					diff := current - lastTotal
					lastTotal = current
					elapsed := time.Since(startTime).Seconds()
					fmt.Printf("  [%.0fs] 总请求: %s | 瞬时QPS: %s | 成功: %s | 失败: %s\n",
						elapsed, fmtNum(current), fmtNum(diff),
						fmtNum(atomic.LoadInt64(&succReqs)),
						fmtNum(atomic.LoadInt64(&failReqs)))
				}
			}
		}()
	}

	wg.Wait()
	actualDur := time.Since(startTime)

	total := atomic.LoadInt64(&totalReqs)
	success := atomic.LoadInt64(&succReqs)
	fail := atomic.LoadInt64(&failReqs)
	totalLat := atomic.LoadInt64(&totalLatNs)
	minL := atomic.LoadInt64(&minLatNs)
	maxL := atomic.LoadInt64(&maxLatNs)

	qps := float64(0)
	if actualDur.Seconds() > 0 {
		qps = float64(total) / actualDur.Seconds()
	}
	var avgL int64
	if success > 0 {
		avgL = totalLat / success
	}
	if minL == int64(time.Hour) {
		minL = 0
	}

	return roundResult{
		Concurrency: concurrency,
		Total:       total,
		Success:     success,
		Fail:        fail,
		QPS:         qps,
		AvgLatNs:    avgL,
		MinLatNs:    minL,
		MaxLatNs:    maxL,
		FirstErr:    firstErrMsg,
	}
}

func preflight(host string, apiPath string) {
	url := fmt.Sprintf("%s%s?ip=8.8.8.8", host, apiPath)
	fmt.Println("正在进行预检请求...")
	fmt.Printf("  URL: %s\n", url)

	statusCode, body, err := fasthttp.Get(nil, url)
	if err != nil {
		fmt.Printf("  预检失败: %v\n", err)
		fmt.Println("  请确认 API 服务是否已启动")
		os.Exit(1)
	}

	fmt.Printf("  预检响应 [HTTP %d]: %s\n", statusCode, string(body))
	fmt.Println()
}

func runAutoMode(host string, apiPath string, useRandom bool, fixedIP string, cpus int) {
	startC := cpus * 4
	if startC < 16 {
		startC = 16
	}
	maxC := cpus * 512
	if maxC < 4096 {
		maxC = 4096
	}
	roundDur := 3 * time.Second

	fmt.Println("预热中（1秒）...")
	runRound(host, apiPath, startC, 1*time.Second, useRandom, fixedIP, false)
	time.Sleep(200 * time.Millisecond)

	fmt.Println()
	fmt.Println("══════════════════════════════════════════════════════════════════════════")
	fmt.Println("  阶段一：粗粒度扫描（倍增）")
	fmt.Println("══════════════════════════════════════════════════════════════════════════")
	fmt.Printf("  %-6s %-8s %-14s %-12s %-12s %s\n",
		"轮次", "并发数", "QPS", "平均延迟", "最大延迟", "趋势")
	fmt.Println("  " + strings.Repeat("─", 68))

	type cResult struct {
		c   int
		res roundResult
	}
	var coarseResults []cResult
	bestQPS := float64(0)
	bestIdx := 0
	dropCount := 0
	round := 0

	for c := startC; c <= maxC; c *= 2 {
		round++
		result := runRound(host, apiPath, c, roundDur, useRandom, fixedIP, false)
		coarseResults = append(coarseResults, cResult{c, result})

		status := ""
		if result.QPS > bestQPS {
			bestQPS = result.QPS
			bestIdx = len(coarseResults) - 1
			dropCount = 0
			status = "↑"
		} else {
			dropCount++
			status = "↓"
		}

		fmt.Printf("  %-6d %-8d %-14s %-12s %-12s %s\n",
			round, c,
			fmtNum(int64(result.QPS)),
			fmtDur(result.AvgLatNs),
			fmtDur(result.MaxLatNs),
			status)

		if dropCount >= 2 {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	peakC := coarseResults[bestIdx].c
	lowerBound := peakC / 4
	upperBound := peakC * 2
	if bestIdx > 0 {
		lowerBound = coarseResults[bestIdx-1].c
	}
	if bestIdx < len(coarseResults)-1 {
		upperBound = coarseResults[bestIdx+1].c
	}
	if lowerBound < startC {
		lowerBound = startC
	}
	if upperBound > maxC {
		upperBound = maxC
	}

	span := upperBound - lowerBound
	steps := 8
	step := span / steps
	if step < 4 {
		step = 4
	}
	step = (step + 3) / 4 * 4

	fmt.Println()
	fmt.Println("══════════════════════════════════════════════════════════════════════════")
	fmt.Printf("  阶段二：精细扫描（%d ~ %d，步长 %d）\n", lowerBound, upperBound, step)
	fmt.Println("══════════════════════════════════════════════════════════════════════════")
	fmt.Printf("  %-6s %-8s %-14s %-12s %-12s %s\n",
		"轮次", "并发数", "QPS", "平均延迟", "最大延迟", "趋势")
	fmt.Println("  " + strings.Repeat("─", 68))

	var fineResults []cResult
	fineBestQPS := float64(0)
	fineBestIdx := 0
	fineRound := 0

	for c := lowerBound; c <= upperBound; c += step {
		fineRound++
		result := runRound(host, apiPath, c, roundDur, useRandom, fixedIP, false)
		fineResults = append(fineResults, cResult{c, result})

		status := ""
		if result.QPS > fineBestQPS {
			fineBestQPS = result.QPS
			fineBestIdx = len(fineResults) - 1
			status = "↑"
		} else {
			status = "↓"
		}

		fmt.Printf("  %-6d %-8d %-14s %-12s %-12s %s\n",
			fineRound, c,
			fmtNum(int64(result.QPS)),
			fmtDur(result.AvgLatNs),
			fmtDur(result.MaxLatNs),
			status)

		time.Sleep(300 * time.Millisecond)
	}

	var best roundResult
	var bestC int
	if fineBestQPS > bestQPS {
		best = fineResults[fineBestIdx].res
		bestC = fineResults[fineBestIdx].c
	} else {
		best = coarseResults[bestIdx].res
		bestC = coarseResults[bestIdx].c
	}

	fmt.Println()
	fmt.Println("══════════════════════════════════════════════════════════════════════════")
	fmt.Printf("  阶段三：最优并发验证（并发 %d，持续 10 秒）\n", bestC)
	fmt.Println("══════════════════════════════════════════════════════════════════════════")
	fmt.Println()

	time.Sleep(500 * time.Millisecond)
	finalResult := runRound(host, apiPath, bestC, 11*time.Second, useRandom, fixedIP, true)

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("  寻峰结果")
	fmt.Println("========================================")
	fmt.Printf("  最优并发: %d\n", bestC)
	fmt.Printf("  峰值 QPS: %s（扫描阶段）\n", fmtNum(int64(best.QPS)))
	fmt.Printf("  验证 QPS: %s（10秒持续）\n", fmtNum(int64(finalResult.QPS)))
	fmt.Printf("  平均延迟: %s\n", fmtDur(finalResult.AvgLatNs))
	fmt.Printf("  最大延迟: %s\n", fmtDur(finalResult.MaxLatNs))
	if finalResult.Total > 0 {
		fmt.Printf("  成功率: %.2f%%\n", float64(finalResult.Success)/float64(finalResult.Total)*100)
	}
	if finalResult.FirstErr != "" {
		fmt.Printf("  首条错误: %s\n", finalResult.FirstErr)
	}
	fmt.Println("========================================")
}

func runManualMode(host string, apiPath string, concurrency, duration int, useRandom bool, fixedIP string) {
	fmt.Println("预热连接池中（2秒）...")
	runRound(host, apiPath, concurrency, 2*time.Second, useRandom, fixedIP, false)
	time.Sleep(200 * time.Millisecond)

	fmt.Println("测试开始...")
	result := runRound(host, apiPath, concurrency, time.Duration(duration)*time.Second, useRandom, fixedIP, true)

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println(" 测试结果")
	fmt.Println("========================================")
	fmt.Printf(" 总请求数: %s\n", fmtNum(result.Total))
	fmt.Printf(" 成功请求: %s\n", fmtNum(result.Success))
	fmt.Printf(" 失败请求: %s\n", fmtNum(result.Fail))
	fmt.Printf(" 平均 QPS: %s\n", fmtNum(int64(result.QPS)))
	if result.Success > 0 {
		fmt.Printf(" 最小延迟: %s\n", fmtDur(result.MinLatNs))
		fmt.Printf(" 最大延迟: %s\n", fmtDur(result.MaxLatNs))
		fmt.Printf(" 平均延迟: %s\n", fmtDur(result.AvgLatNs))
	}
	if result.Total > 0 {
		fmt.Printf(" 成功率: %.2f%%\n", float64(result.Success)/float64(result.Total)*100)
	}
	if result.FirstErr != "" {
		fmt.Printf(" 首条错误: %s\n", result.FirstErr)
	}
	fmt.Println("========================================")
}

func buildHostURL(scheme, ip, port string) string {
	return fmt.Sprintf("%s://%s:%s", scheme, ip, port)
}

func main() {
	cpus := runtime.NumCPU()
	defaultC := cpus * 32
	if defaultC < 50 {
		defaultC = 50
	}

	host := flag.String("host", "", "API 服务完整地址（优先级最高，如 http://127.0.0.1:8080）")
	serverIP := flag.String("ip", "127.0.0.1", "目标服务器 IP 或域名")
	serverPort := flag.String("port", "8080", "目标服务器端口")
	scheme := flag.String("scheme", "http", "请求协议（http 或 https）")
	apiPath := flag.String("path", "/ipinfo", "API 接口路径")
	concurrency := flag.Int("c", defaultC, fmt.Sprintf("并发协程数（自动检测: CPU×32=%d）", defaultC))
	duration := flag.Int("d", 10, "测试持续时间（秒）")
	randomIP := flag.Bool("random", true, "是否使用随机 IP 查询")
	queryIP := flag.String("query-ip", "8.8.8.8", "固定查询 IP（random=false 时生效）")
	autoMode := flag.Bool("auto", false, "自动寻峰模式：逐步提高并发直到找到最高 QPS")
	flag.Parse()

	targetHost := *host
	if targetHost == "" {
		targetHost = buildHostURL(*scheme, *serverIP, *serverPort)
	}

	fmt.Println("========================================")
	fmt.Println("  NekoIPinfo 压力测试工具")
	fmt.Println("========================================")
	fmt.Printf("  目标地址: %s\n", targetHost)
	fmt.Printf("  API 路径: %s\n", *apiPath)
	fmt.Printf("  CPU 核心: %d\n", cpus)
	if *autoMode {
		fmt.Println("  测试模式: 自动寻峰（逐步提高并发至设备极限）")
	} else {
		fmt.Printf("  并发数: %d\n", *concurrency)
		fmt.Printf("  持续时间: %ds\n", *duration)
	}
	if *randomIP {
		fmt.Println("  查询模式: 随机 IP")
	} else {
		fmt.Printf("  查询模式: 固定 IP (%s)\n", *queryIP)
	}
	fmt.Println("========================================")
	fmt.Println()

	preflight(targetHost, *apiPath)

	if *autoMode {
		runAutoMode(targetHost, *apiPath, *randomIP, *queryIP, cpus)
	} else {
		runManualMode(targetHost, *apiPath, *concurrency, *duration, *randomIP, *queryIP)
	}
	os.Exit(0)
}