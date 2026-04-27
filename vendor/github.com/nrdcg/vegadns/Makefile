.PHONY: all
all: build test check

.PHONY: build
build:
	go build -trimpath

.PHONY: test
test:
	go test -count=1 -cover -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

.PHONY: check
check:
	golangci-lint run
