BINARY := tot-mcp
VERSION := 0.5.1
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build clean all

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

# Cross-compile for all major platforms
all: clean
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64 .
	GOOS=linux   GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64 .
	GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe .

clean:
	rm -rf dist/ $(BINARY)
