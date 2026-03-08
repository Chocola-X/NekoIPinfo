# ============================================
# NekoIPinfo Makefile
# ============================================

# 检测操作系统
ifeq ($(OS),Windows_NT)
  HOST_OS  := Windows
  SHELL    := cmd.exe
  .SHELLFLAGS := /C
  RM_CMD    = if exist $(B) rmdir /s /q $(B)
  MKDIR_CMD = if not exist $(subst /,\,$(1)) mkdir $(subst /,\,$(1))
else
  HOST_OS  := Unix
  SHELL    := /bin/sh
  RM_CMD    = rm -rf $(B)
  MKDIR_CMD = mkdir -p $(1)
endif

# 项目配置
APP   = nekoipinfo
BENCH = nekoipinfo-bench
DBGEN = nekoipinfo-dbgen
MB2DB = nekoipinfo-mb2db

CMD_MAIN  = ./cmd
CMD_BENCH = ./cmd/bench
CMD_DBGEN = ./cmd/dbgen
CMD_MB2DB = ./cmd/mb2db

B       = build
LDFLAGS = -s -w
BF      = -trimpath -ldflags "$(LDFLAGS)"

.PHONY: clean all auto help deps test fmt vet linux darwin windows upx

# ============================================
# 全平台编译
# ============================================
all: deps linux darwin windows
	@echo ==============================================
	@echo   All platforms compiled. Output: $(B)/
	@echo ==============================================

# ============================================
# Linux
# ============================================

LINUX_PAIRS = amd64:x86_64 386:x86 arm64:arm64 arm:arm \
              mips:mips mipsle:mipsle mips64:mips64 mips64le:mips64le \
              loong64:loong64 riscv64:riscv64 ppc64:ppc64 ppc64le:ppc64le s390x:s390x

ifeq ($(HOST_OS),Windows)
linux: deps
	@echo ==============================================
	@echo   Compiling Linux
	@echo ==============================================
	@if not exist $(B)\Linux mkdir $(B)\Linux
	@for %%p in ($(LINUX_PAIRS)) do @( \
		for /f "tokens=1,2 delims=:" %%a in ("%%p") do @( \
			echo   %%b & \
			cmd /C "set CGO_ENABLED=0&& set GOOS=linux&& set GOARCH=%%a&& go build $(BF) -o $(B)/Linux/$(APP)_%%b $(CMD_MAIN)" && \
			cmd /C "set CGO_ENABLED=0&& set GOOS=linux&& set GOARCH=%%a&& go build $(BF) -o $(B)/Linux/$(BENCH)_%%b $(CMD_BENCH)" && \
			cmd /C "set CGO_ENABLED=0&& set GOOS=linux&& set GOARCH=%%a&& go build $(BF) -o $(B)/Linux/$(DBGEN)_%%b $(CMD_DBGEN)" \
		) \
	)
	@echo   Linux done
else
linux: deps
	@echo "=============================================="
	@echo "  Compiling Linux"
	@echo "=============================================="
	@mkdir -p $(B)/Linux
	@for pair in $(LINUX_PAIRS); do \
		goarch=$${pair%%:*}; \
		suffix=$${pair##*:}; \
		echo "  $$suffix"; \
		CGO_ENABLED=0 GOOS=linux GOARCH=$$goarch go build $(BF) -o $(B)/Linux/$(APP)_$$suffix $(CMD_MAIN) && \
		CGO_ENABLED=0 GOOS=linux GOARCH=$$goarch go build $(BF) -o $(B)/Linux/$(BENCH)_$$suffix $(CMD_BENCH) && \
		CGO_ENABLED=0 GOOS=linux GOARCH=$$goarch go build $(BF) -o $(B)/Linux/$(DBGEN)_$$suffix $(CMD_DBGEN) || exit 1; \
	done
	@echo "  Linux done"
endif

# ============================================
# macOS
# ============================================

DARWIN_PAIRS = amd64:x86_64 arm64:arm64

ifeq ($(HOST_OS),Windows)
darwin: deps
	@echo ==============================================
	@echo   Compiling macOS
	@echo ==============================================
	@if not exist $(B)\macOS mkdir $(B)\macOS
	@for %%p in ($(DARWIN_PAIRS)) do @( \
		for /f "tokens=1,2 delims=:" %%a in ("%%p") do @( \
			echo   %%b & \
			cmd /C "set CGO_ENABLED=0&& set GOOS=darwin&& set GOARCH=%%a&& go build $(BF) -o $(B)/macOS/$(APP)_%%b $(CMD_MAIN)" && \
			cmd /C "set CGO_ENABLED=0&& set GOOS=darwin&& set GOARCH=%%a&& go build $(BF) -o $(B)/macOS/$(BENCH)_%%b $(CMD_BENCH)" && \
			cmd /C "set CGO_ENABLED=0&& set GOOS=darwin&& set GOARCH=%%a&& go build $(BF) -o $(B)/macOS/$(DBGEN)_%%b $(CMD_DBGEN)" \
		) \
	)
	@echo   macOS done
else
darwin: deps
	@echo "=============================================="
	@echo "  Compiling macOS"
	@echo "=============================================="
	@mkdir -p $(B)/macOS
	@for pair in $(DARWIN_PAIRS); do \
		goarch=$${pair%%:*}; \
		suffix=$${pair##*:}; \
		echo "  $$suffix"; \
		CGO_ENABLED=0 GOOS=darwin GOARCH=$$goarch go build $(BF) -o $(B)/macOS/$(APP)_$$suffix $(CMD_MAIN) && \
		CGO_ENABLED=0 GOOS=darwin GOARCH=$$goarch go build $(BF) -o $(B)/macOS/$(BENCH)_$$suffix $(CMD_BENCH) && \
		CGO_ENABLED=0 GOOS=darwin GOARCH=$$goarch go build $(BF) -o $(B)/macOS/$(DBGEN)_$$suffix $(CMD_DBGEN) || exit 1; \
	done
	@echo "  macOS done"
endif

# ============================================
# Windows
# ============================================

WINDOWS_PAIRS = amd64:x86_64 386:x86 arm64:arm64

ifeq ($(HOST_OS),Windows)
windows: deps
	@echo ==============================================
	@echo   Compiling Windows
	@echo ==============================================
	@if not exist $(B)\Windows mkdir $(B)\Windows
	@for %%p in ($(WINDOWS_PAIRS)) do @( \
		for /f "tokens=1,2 delims=:" %%a in ("%%p") do @( \
			echo   %%b & \
			cmd /C "set CGO_ENABLED=0&& set GOOS=windows&& set GOARCH=%%a&& go build $(BF) -o $(B)/Windows/$(APP)_%%b.exe $(CMD_MAIN)" && \
			cmd /C "set CGO_ENABLED=0&& set GOOS=windows&& set GOARCH=%%a&& go build $(BF) -o $(B)/Windows/$(BENCH)_%%b.exe $(CMD_BENCH)" && \
			cmd /C "set CGO_ENABLED=0&& set GOOS=windows&& set GOARCH=%%a&& go build $(BF) -o $(B)/Windows/$(DBGEN)_%%b.exe $(CMD_DBGEN)" \
		) \
	)
	@echo   Windows done
else
windows: deps
	@echo "=============================================="
	@echo "  Compiling Windows"
	@echo "=============================================="
	@mkdir -p $(B)/Windows
	@for pair in $(WINDOWS_PAIRS); do \
		goarch=$${pair%%:*}; \
		suffix=$${pair##*:}; \
		echo "  $$suffix"; \
		CGO_ENABLED=0 GOOS=windows GOARCH=$$goarch go build $(BF) -o $(B)/Windows/$(APP)_$$suffix.exe $(CMD_MAIN) && \
		CGO_ENABLED=0 GOOS=windows GOARCH=$$goarch go build $(BF) -o $(B)/Windows/$(BENCH)_$$suffix.exe $(CMD_BENCH) && \
		CGO_ENABLED=0 GOOS=windows GOARCH=$$goarch go build $(BF) -o $(B)/Windows/$(DBGEN)_$$suffix.exe $(CMD_DBGEN) || exit 1; \
	done
	@echo "  Windows done"
endif

# ============================================
# 当前平台编译
# ============================================

ifeq ($(HOST_OS),Windows)
auto: deps
	@echo ==============================================
	@echo   Current platform build
	@echo ==============================================
	@if not exist $(B) mkdir $(B)
	@echo   $(APP)
	@cmd /C "set CGO_ENABLED=0&& go build $(BF) -o $(B)/$(APP).exe $(CMD_MAIN)"
	@echo   $(BENCH)
	@cmd /C "set CGO_ENABLED=0&& go build $(BF) -o $(B)/$(BENCH).exe $(CMD_BENCH)"
	@echo   $(DBGEN)
	@cmd /C "set CGO_ENABLED=0&& go build $(BF) -o $(B)/$(DBGEN).exe $(CMD_DBGEN)"
	@echo   $(MB2DB)
	@cmd /C "set CGO_ENABLED=1&& go build $(BF) -o $(B)/$(MB2DB).exe $(CMD_MB2DB)"
	@echo   Done: $(B)/
else
auto: deps
	@echo "=============================================="
	@echo "  Current platform build"
	@echo "=============================================="
	@mkdir -p $(B)
	@echo "  $(APP)"
	@CGO_ENABLED=0 go build $(BF) -o $(B)/$(APP) $(CMD_MAIN)
	@echo "  $(BENCH)"
	@CGO_ENABLED=0 go build $(BF) -o $(B)/$(BENCH) $(CMD_BENCH)
	@echo "  $(DBGEN)"
	@CGO_ENABLED=0 go build $(BF) -o $(B)/$(DBGEN) $(CMD_DBGEN)
	@echo "  $(MB2DB)"
	@CGO_ENABLED=1 go build $(BF) -o $(B)/$(MB2DB) $(CMD_MB2DB)
	@echo "  Done: $(B)/"
endif

# ============================================
# UPX 压缩
# ============================================

ifeq ($(HOST_OS),Windows)
upx:
	@echo ==============================================
	@echo   UPX Compression
	@echo ==============================================
	@where upx >nul 2>&1 && ( \
		for /r $(B) %%f in (*.exe *.) do ( \
			echo   Compressing: %%f & \
			upx --best --lzma "%%f" 2>nul || echo   Skipped: %%f \
		) & echo   UPX done \
	) || echo   UPX not found, skipping compression
else
upx:
	@echo "=============================================="
	@echo "  UPX Compression"
	@echo "=============================================="
	@if command -v upx >/dev/null 2>&1; then \
		find $(B) -type f \( -perm +111 -o -name "*.exe" \) | while read f; do \
			echo "  Compressing: $$f"; \
			upx --best --lzma "$$f" 2>/dev/null || echo "  Skipped: $$f"; \
		done; \
		echo "  UPX done"; \
	else \
		echo "  UPX not found, skipping compression"; \
	fi
endif

# ============================================
# 通用目标
# ============================================

deps:
	@go mod tidy
	@go mod download

test:
	@go test ./... -v -count=1

fmt:
	@gofmt -s -w .

vet:
	@go vet ./...

ifeq ($(HOST_OS),Windows)
clean:
	@if exist $(B) rmdir /s /q $(B)
	@echo   Cleaned $(B)/
else
clean:
	@rm -rf $(B)
	@echo "  Cleaned $(B)/"
endif

help:
	@echo ==============================================
	@echo   NekoIPinfo Makefile
	@echo ==============================================
	@echo   make              Full cross-compile
	@echo   make auto         Build current platform
	@echo   make linux        Linux all arch
	@echo   make darwin       macOS all arch
	@echo   make windows      Windows all arch
	@echo   make upx          Compress with UPX
	@echo   make auto upx     Build then compress
	@echo   make clean        Clean build dir
	@echo   make deps         Download deps
	@echo   make test         Run tests
	@echo   make fmt          Format code
	@echo   make vet          Static analysis
	@echo   make help         This help
	@echo ==============================================
	@echo   Output:
	@echo     $(B)/Linux/       13 arch
	@echo     $(B)/macOS/       2 arch
	@echo     $(B)/Windows/     3 arch
	@echo ==============================================