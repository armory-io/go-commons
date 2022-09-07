APP_NAME="go-commons"
GOARCH ?= $(shell go env GOARCH)
GOOS ?= $(shell go env GOOS)
PWD = $(shell pwd)
BUILD_DIR := ${PWD}/build
DIST_DIR := ${BUILD_DIR}/bin/$(GOOS)_$(GOARCH)

.PHONY: build-dirs
build-dirs:
	@mkdir -p $(BUILD_DIR) $(DIST_DIR) "$(BUILD_DIR)/reports"

.PHONY: check
check: export APP_NAME:=$(APP_NAME)
check: export BUILD_DIR:=$(BUILD_DIR)
check: build-dirs
	@go install github.com/vakenbolt/go-test-report@v0.9.3
	@go run scripts/check.go
