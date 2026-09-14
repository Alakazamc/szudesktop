BINARY  := szunet
DIST    := dist
LDFLAGS := -s -w

.PHONY: all build test vet cross clean check

all: check build

## 编译当前平台的程序
build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY) ./cmd/szunet

## 跑测试和静态检查
check: vet test

test:
	go test ./...

vet:
	go vet ./...

## 一次性交叉编译出三端产物，命名带平台和架构
cross: clean
	mkdir -p $(DIST)
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)-linux-amd64        ./cmd/szunet
	GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)-linux-arm64        ./cmd/szunet
	GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)-darwin-amd64       ./cmd/szunet
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)-darwin-arm64       ./cmd/szunet
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)-windows-amd64.exe  ./cmd/szunet

clean:
	rm -rf $(DIST)
