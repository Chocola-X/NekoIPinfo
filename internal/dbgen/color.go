package dbgen

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	ColorReset   = "\033[0m"
	ColorPink    = "\033[38;5;213m"
	ColorMagenta = "\033[38;5;199m"
	ColorRose    = "\033[38;5;211m"
	ColorHotPink = "\033[38;5;206m"
	ColorFuchsia = "\033[38;5;201m"
	ColorLavend  = "\033[38;5;183m"
	ColorBold    = "\033[1m"
	ColorDim     = "\033[2m"
	ColorWhite   = "\033[37m"
)

var (
	colorEnabled = true
	colorMu      sync.RWMutex
)

func SetColorEnabled(enabled bool) {
	colorMu.Lock()
	defer colorMu.Unlock()
	colorEnabled = enabled
}

func IsColorEnabled() bool {
	colorMu.RLock()
	defer colorMu.RUnlock()
	return colorEnabled
}

func colorize(text, color string) string {
	if !IsColorEnabled() {
		return text
	}
	return color + text + ColorReset
}

func Neko(text string, color string) {
	fmt.Println(colorize(text, color))
}

func Nekof(format string, color string, args ...interface{}) {
	text := fmt.Sprintf(format, args...)
	fmt.Println(colorize(text, color))
}

func NekoHeader(title string) {
	line := colorize("========================================", ColorPink)
	titleStr := colorize(fmt.Sprintf(" 🐾 %s", title), ColorBold+ColorMagenta)
	fmt.Println(line)
	fmt.Println(titleStr)
	fmt.Println(line)
}

func NekoFooter() {
	fmt.Println(colorize("========================================", ColorPink))
}

func NekoSection(title string) {
	fmt.Println()
	fmt.Println(colorize(fmt.Sprintf("── %s ──", title), ColorMagenta))
}

func NekoKV(key, value string) {
	k := colorize(fmt.Sprintf(" %s:", key), ColorLavend)
	v := colorize(value, ColorWhite)
	fmt.Printf("%s %s\n", k, v)
}

func NekoSuccess(text string) {
	Neko(fmt.Sprintf(" ✅ %s", text), ColorPink)
}

func NekoWarn(text string) {
	Neko(fmt.Sprintf(" ⚠️  %s", text), ColorRose)
}

func NekoError(text string) {
	Neko(fmt.Sprintf(" ❌ %s", text), ColorFuchsia)
}

type NekoProgress struct {
	title    string
	total    int64
	current  int64
	start    time.Time
	mu       sync.Mutex
	done     bool
	stopCh   chan struct{}
	frames   []string
	frameIdx int
	barWidth int
}

func NewNekoProgress(title string, total int64) *NekoProgress {
	return &NekoProgress{
		title:   title,
		total:   total,
		frames:  []string{"🐱", "😺", "😸", "😻", "🐱"},
		barWidth: 30,
		stopCh:  make(chan struct{}),
	}
}

func (p *NekoProgress) Start() {
	p.start = time.Now()
	go p.renderLoop()
}

func (p *NekoProgress) SetCurrent(n int64) {
	p.mu.Lock()
	p.current = n
	p.mu.Unlock()
}

func (p *NekoProgress) Finish() {
	p.mu.Lock()
	p.done = true
	p.current = p.total
	p.mu.Unlock()
	close(p.stopCh)
	time.Sleep(50 * time.Millisecond)
	p.render()
	fmt.Println()
}

func (p *NekoProgress) renderLoop() {
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.render()
		}
	}
}

func (p *NekoProgress) render() {
	p.mu.Lock()
	current := p.current
	total := p.total
	done := p.done
	p.mu.Unlock()

	if total <= 0 {
		total = 1
	}

	pct := float64(current) / float64(total)
	if pct > 1 {
		pct = 1
	}

	elapsed := time.Since(p.start)
	var eta time.Duration
	var speed float64
	if elapsed.Seconds() > 0 {
		speed = float64(current) / elapsed.Seconds()
	}
	if speed > 0 && current < total {
		remaining := float64(total-current) / speed
		eta = time.Duration(remaining * float64(time.Second))
	}

	filled := int(pct * float64(p.barWidth))
	if filled > p.barWidth {
		filled = p.barWidth
	}

	frame := p.frames[p.frameIdx%len(p.frames)]
	p.frameIdx++

	var bar strings.Builder
	if IsColorEnabled() {
		bar.WriteString(ColorPink)
	}

	for i := 0; i < p.barWidth; i++ {
		if i < filled {
			bar.WriteString("█")
		} else if i == filled && !done {
			bar.WriteString("▓")
		} else {
			bar.WriteString("░")
		}
	}

	if IsColorEnabled() {
		bar.WriteString(ColorReset)
	}

	pctStr := fmt.Sprintf("%.1f%%", pct*100)

	var statusLine string
	if done {
		statusLine = fmt.Sprintf("\r %s %s %s [%s] %s | %s | 完成喵！",
			colorize(frame, ColorMagenta),
			colorize(p.title, ColorLavend),
			bar.String(),
			colorize(pctStr, ColorHotPink),
			colorize(fmtCount(current), ColorWhite),
			colorize(elapsed.Round(time.Millisecond).String(), ColorDim),
		)
	} else {
		etaStr := "计算中..."
		if eta > 0 {
			etaStr = eta.Round(time.Second).String()
		}
		speedStr := fmtSpeed(speed)
		statusLine = fmt.Sprintf("\r %s %s %s [%s] %s/%s | %s | ETA %s",
			colorize(frame, ColorMagenta),
			colorize(p.title, ColorLavend),
			bar.String(),
			colorize(pctStr, ColorHotPink),
			colorize(fmtCount(current), ColorWhite),
			colorize(fmtCount(total), ColorDim),
			colorize(speedStr, ColorRose),
			colorize(etaStr, ColorDim),
		)
	}

	fmt.Fprint(os.Stderr, statusLine)
}

func fmtCount(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%.2fM", float64(n)/1000000)
}

func fmtSpeed(s float64) string {
	if s <= 0 {
		return "-- /s"
	}
	if s < 1000 {
		return fmt.Sprintf("%.0f/s", s)
	}
	if s < 1000000 {
		return fmt.Sprintf("%.1fK/s", s/1000)
	}
	return fmt.Sprintf("%.2fM/s", s/1000000)
}