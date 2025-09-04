GIT_BRANCH?=$(shell git branch --show-current)
GIT_COMMIT?=$(shell git rev-parse HEAD)
GIT_COMMIT_SHORT?=$(shell git rev-parse --short HEAD)
GIT_TAG?=v0.0.0
ifneq ($(GIT_BRANCH), main)
GIT_TAG?=$(shell git describe --abbrev=0 --tags 2>/dev/null || echo "v0.0.0" )
endif
TAG?=${GIT_TAG}-${GIT_COMMIT_SHORT}
REPO?=docker.io/rancher
IMAGE = $(REPO)/ali-operator:$(TAG)
MACHINE := rancher
# Define the target platforms that can be used across the ecosystem.
# Note that what would actually be used for a given project will be
# defined in TARGET_PLATFORMS, and must be a subset of the below:
DEFAULT_PLATFORMS := linux/amd64,linux/arm64,darwin/arm64,darwin/amd64
TARGET_PLATFORMS := linux/amd64,linux/arm64
BUILDX_ARGS ?= --sbom=true --attest type=provenance,mode=max

ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
BIN_DIR := $(abspath $(ROOT_DIR)/bin)
GO_INSTALL = ./scripts/go_install.sh

GO_APIDIFF_VER := v0.8.2
GO_APIDIFF_BIN := go-apidiff
GO_APIDIFF := $(BIN_DIR)/$(GO_APIDIFF_BIN)-$(GO_APIDIFF_VER)

default: operator

.PHONY: generate-crd
generate-crd:
	go generate main.go

.PHONY: generate
generate:
	$(MAKE) generate-crd

.PHONY: operator
operator:
	CGO_ENABLED=0 go build -ldflags \
            "-X github.com/rancher/ali-operator/pkg/version.GitCommit=$(GIT_COMMIT) \
             -X github.com/rancher/ali-operator/pkg/version.Version=$(TAG)" \
        -o bin/ali-operator .

$(GO_APIDIFF):
	GOBIN=$(BIN_DIR) $(GO_INSTALL) github.com/joelanford/go-apidiff $(GO_APIDIFF_BIN) $(GO_APIDIFF_VER)

.PHONY: buildx-machine
buildx-machine: ## create rancher dockerbuildx machine targeting platform defined by DEFAULT_PLATFORMS
	@docker buildx ls | grep $(MACHINE) || \
		docker buildx create --name=$(MACHINE) --platform=$(DEFAULT_PLATFORMS)

.PHONY: image-build
image-build: buildx-machine ## build (and load) the container image targeting the current platform.
	docker buildx build -f package/Dockerfile \
    	--builder $(MACHINE) --build-arg COMMIT=$(GIT_COMMIT) --build-arg VERSION=$(TAG) \
		--output=type=cacheonly \
    	-t "$(IMAGE)" $(BUILD_ACTION) .
	@echo "Built $(IMAGE)"

APIDIFF_OLD_COMMIT ?= $(shell git rev-parse origin/main)

.PHONY: apidiff
apidiff: $(GO_APIDIFF) ## Check for API differences
	$(GO_APIDIFF) $(APIDIFF_OLD_COMMIT) --print-compatible
