.PHONY: build test clean install lint

BINARY := pi-go
VERSION := 0.1.0

build:
	go build -ldflags="-s -w -X main.version=$(VERSION)" -o $(BINARY) .

test:
	go test -v ./...

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	go vet ./...
	@if command -v golint >/dev/null 2>&1; then golint ./...; fi

fmt:
	go fmt ./...

clean:
	rm -f $(BINARY)
	rm -f coverage.out coverage.html

install: build
	mv $(BINARY) $(GOPATH)/bin/

run:
	go run main.go

all: fmt lint test build
