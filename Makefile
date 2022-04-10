VERSION := $(shell echo $(shell git describe --tags) | sed 's/^v//')

LD_FLAGS = -X github.com/jtieri/atlas/cmd.Version=$(VERSION)

BUILD_FLAGS := -ldflags '$(LD_FLAGS)'

build: go.sum
ifeq ($(OS),Windows_NT)
	@echo "building atlas binary..."
	@go build -mod=readonly $(BUILD_FLAGS) -o build/atlas.exe main.go
else
	@echo "building atlas binary..."
	@go build -mod=readonly $(BUILD_FLAGS) -o build/atlas main.go
endif

install: go.sum
	@echo "installing atlas binary..."
	@go build -mod=readonly $(BUILD_FLAGS) -o $(GOBIN)/atlas main.go

test:
	@go test -mod=readonly -race ./...