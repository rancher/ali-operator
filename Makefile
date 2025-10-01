GIT_BRANCH?=$(shell git branch --show-current)
GIT_COMMIT?=$(shell git rev-parse HEAD)
GIT_COMMIT_SHORT?=$(shell git rev-parse --short HEAD)
GIT_TAG?=v0.0.0
ifneq ($(GIT_BRANCH), main)
GIT_TAG?=$(shell git describe --abbrev=0 --tags 2>/dev/null || echo "v0.0.0" )
endif
TAG?=${GIT_TAG}-${GIT_COMMIT_SHORT}
REPO?=rancher
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

MOCKGEN_VER := v1.6.0
MOCKGEN_BIN := mockgen
MOCKGEN := $(BIN_DIR)/$(MOCKGEN_BIN)-$(MOCKGEN_VER)
GINKGO_VER := v2.21.0
GINKGO_BIN := ginkgo
GINKGO := $(BIN_DIR)/$(GINKGO_BIN)-$(GINKGO_VER)
SETUP_ENVTEST_VER := v0.0.0-20230905084151-2a553d6f910d
SETUP_ENVTEST_BIN := setup-envtest
SETUP_ENVTEST := $(BIN_DIR)/$(SETUP_ENVTEST_BIN)-$(SETUP_ENVTEST_VER)

ifeq ($(shell go env GOOS),darwin) # Use the darwin/amd64 binary until an arm64 version is available
	KUBEBUILDER_ASSETS ?= $(shell $(SETUP_ENVTEST) use --use-env -p path --arch amd64 $(KUBEBUILDER_ENVTEST_KUBERNETES_VERSION))
else
	KUBEBUILDER_ASSETS ?= $(shell $(SETUP_ENVTEST) use --use-env -p path $(KUBEBUILDER_ENVTEST_KUBERNETES_VERSION))
endif

default: operator

$(MOCKGEN):
	GOBIN=$(BIN_DIR) $(GO_INSTALL) github.com/golang/mock/mockgen $(MOCKGEN_BIN) $(MOCKGEN_VER)

$(GINKGO):
	GOBIN=$(BIN_DIR) $(GO_INSTALL) github.com/onsi/ginkgo/v2/ginkgo $(GINKGO_BIN) $(GINKGO_VER)

$(SETUP_ENVTEST): 
	GOBIN=$(BIN_DIR) $(GO_INSTALL) sigs.k8s.io/controller-runtime/tools/setup-envtest $(SETUP_ENVTEST_BIN) $(SETUP_ENVTEST_VER)

.PHONY: generate-crd
generate-crd:
	go generate main.go

.PHONY: generate
generate:
	$(MAKE) generate-crd
	$(MAKE) generate-go

.PHONY: generate-go
generate-go: $(MOCKGEN)
	go generate ./pkg/alibaba/...

.PHONY: operator
operator:
	CGO_ENABLED=0 go build -ldflags \
            "-X github.com/rancher/ali-operator/pkg/version.GitCommit=$(GIT_COMMIT) \
             -X github.com/rancher/ali-operator/pkg/version.Version=$(TAG)" \
        -o bin/ali-operator .

$(GO_APIDIFF):
	GOBIN=$(BIN_DIR) $(GO_INSTALL) github.com/joelanford/go-apidiff $(GO_APIDIFF_BIN) $(GO_APIDIFF_VER)

ALL_VERIFY_CHECKS = generate

.PHONY: verify
verify: $(addprefix verify-,$(ALL_VERIFY_CHECKS))

.PHONY: verify-generate
verify-generate: generate
	@if !(git diff --quiet HEAD); then \
		git diff; \
		echo "generated files are out of date, run make generate"; exit 1; \
	fi

.PHONY: buildx-machine
buildx-machine: ## create rancher dockerbuildx machine targeting platform defined by DEFAULT_PLATFORMS
	@docker buildx ls | grep $(MACHINE) || \
		docker buildx create --name=$(MACHINE) --platform=$(DEFAULT_PLATFORMS)

.PHONY: image-build
image-build: buildx-machine ## build (and load) the container image targeting the current platform.
	docker buildx build -f package/Dockerfile \
    	--builder $(MACHINE) --build-arg COMMIT=$(GIT_COMMIT) --build-arg VERSION=$(TAG) \
		--output=type=docker \
    	-t "$(IMAGE)" $(BUILD_ACTION) .
	@echo "Built $(IMAGE)"

APIDIFF_OLD_COMMIT ?= $(shell git rev-parse origin/main)

.PHONY: apidiff
apidiff: $(GO_APIDIFF) ## Check for API differences
	$(GO_APIDIFF) $(APIDIFF_OLD_COMMIT) --print-compatible

.PHONY: operator-chart
operator-chart:
	mkdir -p $(BIN_DIR)
	cp -rf $(ROOT_DIR)/charts/ali-operator $(BIN_DIR)/chart
	sed -i -e 's/tag:.*/tag: '${TAG}'/' $(BIN_DIR)/chart/values.yaml
	sed -i -e 's|repository:.*|repository: '${REPO}/ali-operator'|' $(BIN_DIR)/chart/values.yaml
	helm package --version ${CHART_VERSION} --app-version ${GIT_TAG} -d $(BIN_DIR)/ $(BIN_DIR)/chart
	rm -Rf $(BIN_DIR)/chart
	
.PHONY: crd-chart
crd-chart:
	mkdir -p $(BIN_DIR)
	helm package --version ${CHART_VERSION} --app-version ${GIT_TAG} -d $(BIN_DIR)/ $(ROOT_DIR)/charts/ali-operator-crd
	rm -Rf $(BIN_DIR)/chart

.PHONY: charts
charts:
	$(MAKE) operator-chart
	$(MAKE) crd-chart

.PHONY: test
test: $(SETUP_ENVTEST) $(GINKGO)
	KUBEBUILDER_ASSETS="$(KUBEBUILDER_ASSETS)" $(GINKGO) -v -r -p --trace ./pkg/... ./controller/...
