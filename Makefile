# 提取git commit id
COMMIT_ID=$(shell git rev-parse --short HEAD || echo "<unknown-commit-id>")
$(info curl-go commit id: ${COMMIT_ID})

# 以iso8601格式输出UTC日期时间
BUILD_TIME=$(shell date -Iseconds || echo "<unknown-build-time>")
$(info curl-go build time: ${BUILD_TIME})

# 提取CHANGLOG.md中的第一行作为版本号
VERSION=$(shell head -n 1 CHANGELOG.md | sed -e 's/^\# v//')
$(info curl-go version: ${VERSION})

# 传递给go build的参数
LD_FLAGS=" \
	-X github.com/zhangzqs/curl-go/internal/version.COMMIT_ID=$(COMMIT_ID) \
	-X github.com/zhangzqs/curl-go/internal/version.BUILD_TIME=$(BUILD_TIME) \
	-X github.com/zhangzqs/curl-go/internal/version.VERSION=$(VERSION)"

build:
	@go build -ldflags $(LD_FLAGS) -o curl-go

install:
	@go install -ldflags $(LD_FLAGS)