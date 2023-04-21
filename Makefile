#################
##
## AUTOGENERATED makefile.
##
#################

APP_NAME              = go-commons
VERSION               ?= $(shell ./scripts/version.sh)
REGISTRY              ?="armory-docker-local.jfrog.io"
REGISTRY_ORG          ?="armory"
GOARCH                ?= $(shell go env GOARCH)
GOOS                  ?= $(shell go env GOOS)
PWD                   =  $(shell pwd)
IMAGE_TAG             ?= $(VERSION)
LOCAL_KUBECTL_CONTEXT ?= "kind-armory-cloud-dev"
IMAGE                 := $(subst $\",,$(REGISTRY)/$(REGISTRY_ORG)/${APP_NAME}:${VERSION})
BUILD_DIR             := ${PWD}/build
DIST_DIR              := ${BUILD_DIR}/bin/$(GOOS)_$(GOARCH)
GEN_DIR               := ${PWD}/generated

#####################
## Build steps
#####################
.PHONY: all
all: clean build-dirs run-before-tools build check run-after-tools

# clean all work directories
.PHONY: clean
clean:
	@rm -fr $(BUILD_DIR)
	@rm -fr $(GEN_DIR)

.PHONY: build-dirs
build-dirs:
	@mkdir -p $(BUILD_DIR) $(DIST_DIR) "$(BUILD_DIR)/reports" $(GEN_DIR)

.PHONY: build
build:
	@echo Build of binary not enabled for this project

#####################
## Run tests and generate coverage report
#####################
.PHONY: check
check: export APP_NAME:=$(APP_NAME)
check: export BUILD_DIR:=$(BUILD_DIR)
check: build-dirs generate_mocks
	@go run scripts/check.go

#####################
## install all tools step
#####################
.PHONY: install-tools
install-tools:
	@test ! -f "$(GEN_DIR)/.tools" && \
	echo installing tools.... && \
	go install github.com/vakenbolt/go-test-report@v0.9.3 && \
	echo installing mockgen... && \
	go install github.com/golang/mock/mockgen@v1.6.0 && \
	echo installing static check... && \
	go install honnef.co/go/tools/cmd/staticcheck@latest && \
	mkdir -p $(GEN_DIR) && touch $(GEN_DIR)/.tools || echo tools are installed


#####################
## Local generate_mocks
#####################
.PHONY: generate_mocks
generate_mocks: install-tools
	@echo generating mock files...
	@go generate -run mockgen ./...


#####################
## Local static-check
#####################
.PHONY: static-check
static-check: install-tools
	@echo "Running static check in ${PWD}..."
	@staticcheck ./...


.PHONY: run-before-tools
run-before-tools: install-tools generate_mocks 
	@go mod tidy
	@echo run before tools DONE

.PHONY: run-after-tools
run-after-tools: static-check
	@echo run after tools DONE

#####################
## Custom build steps
#####################

###################
## Trigger project bootstrap with latest version
###################
.PHONY: bootstrap-project
bootstrap-project: clean project.yaml
	@echo setting up project...
	@docker pull armory-docker-local.jfrog.io/armory/go-makefile:latest
	@docker run -v "${PWD}":/root/templates/data armory-docker-local.jfrog.io/armory/go-makefile:latest

