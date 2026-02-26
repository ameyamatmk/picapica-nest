APP_NAME    := picapica-nest
CMD_PATH    := ./cmd/picapica-nest
BUILD_DIR   := build

VERSION     := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME  := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

LDFLAGS     := -s -w \
  -X main.version=$(VERSION) \
  -X main.gitCommit=$(GIT_COMMIT) \
  -X main.buildTime=$(BUILD_TIME)

PLATFORMS   := linux/amd64 linux/arm64

.PHONY: build build-all clean

## build: ローカル環境向けビルド
build:
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME) $(CMD_PATH)

## build-all: クロスコンパイル (linux/amd64, linux/arm64)
build-all:
	@mkdir -p $(BUILD_DIR)
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		output=$(BUILD_DIR)/$(APP_NAME)-$${os}-$${arch}; \
		echo ">> Building $${output}..."; \
		GOOS=$${os} GOARCH=$${arch} go build -ldflags "$(LDFLAGS)" -o $${output} $(CMD_PATH); \
	done
	@echo ">> Build complete"
	@ls -lh $(BUILD_DIR)/

## clean: ビルド成果物を削除
clean:
	rm -rf $(BUILD_DIR)
