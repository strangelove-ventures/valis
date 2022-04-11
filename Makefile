VERSION := $(shell echo $(shell git describe --tags) | sed 's/^v//')

LD_FLAGS = -X github.com/strangelove-ventures/valis/cmd.Version=$(VERSION)

BUILD_FLAGS := -ldflags '$(LD_FLAGS)'

build: go.sum
ifeq ($(OS),Windows_NT)
	@echo "building valis binary..."
	@go build -mod=readonly $(BUILD_FLAGS) -o build/valis.exe main.go
else
	@echo "building valis binary..."
	@go build -mod=readonly $(BUILD_FLAGS) -o build/valis main.go
endif

install: go.sum
	@echo "installing valis binary..."
	@go build -mod=readonly $(BUILD_FLAGS) -o $(GOBIN)/valis main.go

test:
	@go test -mod=readonly -race ./...