# Version information
include Makefile.version

SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
BUILD_DIR := ${PROJECT_DIR}/build

REGISTRY ?= ghcr.io/mellanox
IMAGE_TAG_BASE ?= $(REGISTRY)/nvidia-k8s-ipam
IMAGE_TAG ?= latest

# Image URL to use all building/pushing image targets
IMG ?= $(IMAGE_TAG_BASE):$(IMAGE_TAG)
DOCKERFILE ?= Dockerfile

# which container engine to use for image build/push
DOCKER_CMD ?= docker

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.27.1

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

TARGET_OS ?= $(shell go env GOOS)
TARGET_ARCH ?= $(shell go env GOARCH)

# Options for go build command
GO_BUILD_OPTS ?= CGO_ENABLED=0 GOOS=$(TARGET_OS) GOARCH=$(TARGET_ARCH)
# Linker flags for go build command
GO_LDFLAGS = $(VERSION_LDFLAGS)


PKGS = $(or $(PKG),$(shell cd $(PROJECT_DIR) && go list ./... | grep -v "^nvidia-k8s-ipam/vendor/" | grep -v ".*/mocks"))

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: clean
clean: ## Remove downloaded tools and compiled binaries
	@rm -rf $(LOCALBIN)
	@rm -rf $(BUILD_DIR)
	@rm -rf $(GRPC_TMP_DIR)

##@ Development

.PHONY: lint
lint: golangci-lint ## Lint code.
	$(GOLANGCILINT) run --timeout 10m

COVERAGE_MODE = atomic
COVER_PROFILE = $(PROJECT_DIR)/cover.out
LCOV_PATH =  $(PROJECT_DIR)/lcov.info

.PHONY: unit-test
unit-test: envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test -covermode=$(COVERAGE_MODE) -coverprofile=$(COVER_PROFILE) $(PKGS)

.PHONY: test
test: lint unit-test

.PHONY: cov-report
cov-report: gcov2lcov unit-test  ## Build test coverage report in lcov format
	$(GCOV2LCOV) -infile $(COVER_PROFILE) -outfile $(LCOV_PATH)

##@ Build

.PHONY: build-controller
build-controller: ## build IPAM controller
	$(GO_BUILD_OPTS) go build -ldflags $(GO_LDFLAGS) -o $(BUILD_DIR)/ipam-controller ./cmd/ipam-controller/main.go

.PHONY: build-node
build-node: ## build IPAM node
	$(GO_BUILD_OPTS) go build -ldflags $(GO_LDFLAGS) -o $(BUILD_DIR)/ipam-node ./cmd/ipam-node/main.go

.PHONY: build-cni
build-cni: ## build IPAM cni
	$(GO_BUILD_OPTS) go build -ldflags $(GO_LDFLAGS) -o $(BUILD_DIR)/nv-ipam ./cmd/nv-ipam/main.go

.PHONY: build
build: build-controller build-node build-cni ## Build project binaries
	

.PHONY: docker-build
docker-build:  ## Build docker image with ipam binaries
	$(DOCKER_CMD) build -t $(IMG) -f $(DOCKERFILE) .

.PHONY: docker-push
docker-push: ## Push docker image with ipam binaries
	$(DOCKER_CMD) push $(IMG)


KIND_CLUSTER ?= kind
.PHONY: kind-load-image
kind-load-image:  ## Load ipam image to kind cluster
	kind load docker-image --name $(KIND_CLUSTER) $(IMG)

.PHONY: generate-mocks
generate-mocks: mockery ## generate mock objects
	PATH=$(LOCALBIN):$(PATH) go generate ./...

## Location to install dependencies to
LOCALBIN ?= $(PROJECT_DIR)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Temporary location for GRPC files
GRPC_TMP_DIR  ?= $(CURDIR)/_tmp
$(GRPC_TMP_DIR):
	@mkdir -p $@

##@ Tools

## Tool Binaries
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCILINT ?= $(LOCALBIN)/golangci-lint
GCOV2LCOV ?= $(LOCALBIN)/gcov2lcov
MOCKERY ?= $(LOCALBIN)/mockery
PROTOC ?= $(LOCALBIN)/protoc/bin/protoc
PROTOC_GEN_GO ?= $(LOCALBIN)/protoc-gen-go
PROTOC_GEN_GO_GRPC ?= $(LOCALBIN)/protoc-gen-go-grpc
BUF ?= $(LOCALBIN)/buf

## Tool Versions
GOLANGCILINT_VERSION ?= v1.52.2
GCOV2LCOV_VERSION ?= v1.0.5
MOCKERY_VERSION ?= v2.27.1
PROTOC_VER ?= 23.4
PROTOC_GEN_GO_VER ?= 1.31.0
PROTOC_GEN_GO_GRPC_VER ?= 1.3.0
BUF_VERSION ?= 1.23.1


.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): | $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: golangci-lint
golangci-lint: $(GOLANGCILINT) ## Download golangci-lint locally if necessary.
$(GOLANGCILINT): | $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCILINT_VERSION)

.PHONY: gcov2lcov
gcov2lcov: $(GCOV2LCOV) ## Download gcov2lcov locally if necessary.
$(GCOV2LCOV): | $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/jandelgado/gcov2lcov@$(GCOV2LCOV_VERSION)

.PHONY: mockery
mockery: $(MOCKERY) ## Download mockery locally if necessary.
$(MOCKERY): | $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/vektra/mockery/v2@$(MOCKERY_VERSION)

.PHONY: protoc
PROTOC_REL ?= https://github.com/protocolbuffers/protobuf/releases
protoc: $(PROTOC) ## Download protoc locally if necessary.
$(PROTOC): | $(LOCALBIN)
	cd $(LOCALBIN) && \
	curl -L --output tmp.zip $(PROTOC_REL)/download/v$(PROTOC_VER)/protoc-$(PROTOC_VER)-linux-x86_64.zip && \
	unzip tmp.zip -d protoc && rm tmp.zip

.PHONY: protoc-gen-go
protoc-gen-go: $(PROTOC_GEN_GO) ## Download protoc-gen-go locally if necessary.
$(PROTOC_GEN_GO): | $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install google.golang.org/protobuf/cmd/protoc-gen-go@v$(PROTOC_GEN_GO_VER)

.PHONY: protoc-gen-go-grpc
protoc-gen-go-grpc: $(PROTOC_GEN_GO_GRPC) ## Download protoc-gen-go locally if necessary.
$(PROTOC_GEN_GO_GRPC): | $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v$(PROTOC_GEN_GO_GRPC_VER)

.PHONY: buf
buf: $(BUF) ## Download buf locally if necessary
$(BUF): | $(LOCALBIN)
	cd $(LOCALBIN) && \
	curl -sSL "https://github.com/bufbuild/buf/releases/download/v$(BUF_VERSION)/buf-Linux-x86_64" -o "$(LOCALBIN)/buf" && \
	chmod +x "$(LOCALBIN)/buf"

##@ GRPC
# go package for generated code
API_PKG_GO_MOD ?= github.com/Mellanox/nvidia-k8s-ipam/api/grpc

# GRPC DIRs
GRPC_DIR ?= $(PROJECT_DIR)/api/grpc
PROTO_DIR ?= $(GRPC_DIR)/proto
GENERATED_CODE_DIR ?= $(GRPC_DIR)

grpc-generate: protoc protoc-gen-go protoc-gen-go-grpc ## Generate GO client and server GRPC code
	@echo "generate GRPC API"; \
	echo "   go module: $(API_PKG_GO_MOD)"; \
	echo "   output dir: $(GENERATED_CODE_DIR) "; \
	echo "   proto dir: $(PROTO_DIR) "; \
	cd $(PROTO_DIR) && \
	TARGET_FILES=""; \
	PROTOC_OPTIONS="--plugin=protoc-gen-go=$(PROTOC_GEN_GO) \
					--plugin=protoc-gen-go-grpc=$(PROTOC_GEN_GO_GRPC) \
					--go_out=$(GENERATED_CODE_DIR) \
					--go_opt=module=$(API_PKG_GO_MOD) \
					--proto_path=$(PROTO_DIR) \
					--go-grpc_out=$(GENERATED_CODE_DIR) \
					--go-grpc_opt=module=$(API_PKG_GO_MOD)"; \
	echo "discovered proto files:"; \
	for proto_file in $$(find . -name "*.proto"); do \
		proto_file=$$(echo $$proto_file | cut -d'/' -f2-); \
		proto_dir=$$(dirname $$proto_file); \
		pkg_name=M$$proto_file=$(API_PKG_GO_MOD)/$$proto_dir; \
		echo "    $$proto_file"; \
		TARGET_FILES="$$TARGET_FILES $$proto_file"; \
		PROTOC_OPTIONS="$$PROTOC_OPTIONS \
						--go_opt=$$pkg_name \
						--go-grpc_opt=$$pkg_name" ; \
	done; \
	$(PROTOC)  $$PROTOC_OPTIONS $$TARGET_FILES

grpc-check: grpc-format grpc-lint protoc protoc-gen-go protoc-gen-go-grpc $(GRPC_TMP_DIR)  ## Check that generated GO client code match proto files
	@rm -rf $(GRPC_TMP_DIR)/nvidia/
	@$(MAKE) GENERATED_CODE_DIR=$(GRPC_TMP_DIR) grpc-generate
	@diff -Naur $(GRPC_TMP_DIR)/nvidia/ $(GENERATED_CODE_DIR)/nvidia/ || \
		(printf "\n\nOutdated files detected!\nPlease, run 'make generate' to regenerate GO code\n\n" && exit 1)
	@echo "generated files are up to date"

grpc-lint: buf  ## Lint GRPC files
	@echo "lint protobuf files";
	cd $(PROTO_DIR) && \
	$(BUF) lint --config ../buf.yaml

grpc-format: buf  ## Format GRPC files
	@echo "format protobuf files";
	cd $(PROTO_DIR) && \
	$(BUF) format -w --exit-code
