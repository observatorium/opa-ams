SHELL=/usr/bin/env bash -o pipefail
TMP_DIR := $(shell pwd)/tmp
BIN_DIR ?= $(TMP_DIR)/bin
LIB_DIR ?= $(TMP_DIR)/lib
FIRST_GOPATH := $(firstword $(subst :, ,$(shell go env GOPATH)))
OS ?= $(shell uname -s | tr '[A-Z]' '[a-z]')
ARCH ?= $(shell uname -m)

VERSION := $(strip $(shell [ -d .git ] && git describe --always --tags --dirty))
BUILD_DATE := $(shell date -u +"%Y-%m-%d")
BUILD_TIMESTAMP := $(shell date -u +"%Y-%m-%dT%H:%M:%S%Z")
VCS_BRANCH := $(strip $(shell git rev-parse --abbrev-ref HEAD))
VCS_REF := $(strip $(shell [ -d .git ] && git rev-parse --short HEAD))
DOCKER_REPO ?= quay.io/observatorium/opa-ams

THANOS ?= $(BIN_DIR)/thanos
THANOS_VERSION ?= 0.13.0
OBSERVATORIUM ?= $(BIN_DIR)/observatorium
UP ?= $(BIN_DIR)/up
HYDRA ?= $(BIN_DIR)/hydra
GOLANGCILINT ?= $(FIRST_GOPATH)/bin/golangci-lint
GOLANGCILINT_VERSION ?= v1.21.0
EMBEDMD ?= $(BIN_DIR)/embedmd
SHELLCHECK ?= $(BIN_DIR)/shellcheck
MEMCACHED ?= $(BIN_DIR)/memcached
AMS ?= $(BIN_DIR)/ams

default: opa-ams
all: clean lint test opa-ams

tmp/help.txt: opa-ams
	./opa-ams --help &> tmp/help.txt || true

tmp/load_help.txt:
	-./test/load.sh -h > tmp/load_help.txt 2&>1

README.md: $(EMBEDMD) tmp/help.txt
	$(EMBEDMD) -w README.md

benchmark.md: $(EMBEDMD) tmp/load_help.txt
	-rm -rf ./docs/loadtests
	PATH=$$PATH:$$(pwd)/$(BIN_DIR):$(FIRST_GOPATH)/bin ./test/load.sh -r 300 -c 1000 -m 3 -q 10 -o gnuplot
	$(EMBEDMD) -w docs/benchmark.md

opa-ams: vendor main.go $(wildcard *.go) $(wildcard */*.go)
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=amd64 GO111MODULE=on GOPROXY=https://proxy.golang.org go build -mod vendor -a -ldflags '-s -w' -o $@ .

.PHONY: build
build: opa-ams

.PHONY: vendor
vendor: go.mod go.sum
	go mod tidy
	go mod vendor

.PHONY: format
format: $(GOLANGCILINT)
	$(GOLANGCILINT) run --fix --enable-all -c .golangci.yml

.PHONY: go-fmt
go-fmt:
	@fmt_res=$$(gofmt -d -s $$(find . -type f -name '*.go' -not -path './vendor/*' -not -path './jsonnet/vendor/*' -not -path '${TMP_DIR}/*')); if [ -n "$$fmt_res" ]; then printf '\nGofmt found style issues. Please check the reported issues\nand fix them if necessary before submitting the code for review:\n\n%s' "$$fmt_res"; exit 1; fi

.PHONY: shellcheck
shellcheck: $(SHELLCHECK)
	$(SHELLCHECK) $(shell find . -type f -name "*.sh" -not -path "*vendor*" -not -path "${TMP_DIR}/*")

.PHONY: lint
lint: $(GOLANGCILINT) vendor go-fmt shellcheck
	$(GOLANGCILINT) run -v --enable-all -c .golangci.yml

.PHONY: test
test: build test-unit test-integration

.PHONY: test-unit
test-unit:
	CGO_ENABLED=1 GO111MODULE=on go test -mod vendor -v -race -short ./...

.PHONY: test-integration
test-integration: build integration-test-dependencies
	PATH=$(BIN_DIR):$(FIRST_GOPATH)/bin:$$PATH LD_LIBRARY_PATH=$$LD_LIBRARY_PATH:$(LIB_DIR) ./test/integration.sh

.PHONY: clean
clean:
	-rm tmp/help.txt
	-rm -rf tmp/bin
	-rm opa-ams

.PHONY: container
container: Dockerfile
	@docker build --build-arg BUILD_DATE="$(BUILD_TIMESTAMP)" \
		--build-arg VERSION="$(VERSION)" \
		--build-arg VCS_REF="$(VCS_REF)" \
		--build-arg VCS_BRANCH="$(VCS_BRANCH)" \
		--build-arg DOCKERFILE_PATH="/Dockerfile" \
		-t $(DOCKER_REPO):$(VCS_BRANCH)-$(BUILD_DATE)-$(VERSION) .
	@docker tag $(DOCKER_REPO):$(VCS_BRANCH)-$(BUILD_DATE)-$(VERSION) $(DOCKER_REPO):latest

.PHONY: container-push
container-push: container
	docker push $(DOCKER_REPO):$(VCS_BRANCH)-$(BUILD_DATE)-$(VERSION)
	docker push $(DOCKER_REPO):latest

.PHONY: container-release
container-release: VERSION_TAG = $(strip $(shell [ -d .git ] && git tag --points-at HEAD))
container-release: container
	# https://git-scm.com/docs/git-tag#Documentation/git-tag.txt---points-atltobjectgt
	@docker tag $(DOCKER_REPO):$(VCS_BRANCH)-$(BUILD_DATE)-$(VERSION) $(DOCKER_REPO):$(VERSION_TAG)
	docker push $(DOCKER_REPO):$(VERSION_TAG)
	docker push $(DOCKER_REPO):latest

.PHONY: integration-test-dependencies
integration-test-dependencies: $(THANOS) $(UP) $(HYDRA) $(OBSERVATORIUM) $(MEMCACHED) $(AMS)

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

$(LIB_DIR):
	mkdir -p $@

$(THANOS): | $(BIN_DIR)
	@echo "Downloading Thanos"
	curl -L "https://github.com/thanos-io/thanos/releases/download/v$(THANOS_VERSION)/thanos-$(THANOS_VERSION).$$(go env GOOS)-$$(go env GOARCH).tar.gz" | tar --strip-components=1 -xzf - -C $(BIN_DIR)

$(OBSERVATORIUM): | vendor $(BIN_DIR)
	go build -mod=vendor -o $@ github.com/observatorium/observatorium

$(UP): | vendor $(BIN_DIR)
	go build -mod=vendor -o $@ github.com/observatorium/up/cmd/up

$(HYDRA): | vendor $(BIN_DIR)
	@echo "Downloading Hydra"
	curl -L "https://github.com/ory/hydra/releases/download/v1.7.4/hydra_1.7.4_linux_64-bit.tar.gz" | tar -xzf - -C $(BIN_DIR) hydra

$(EMBEDMD): | vendor $(BIN_DIR)
	go build -mod=vendor -o $@ github.com/campoy/embedmd

$(AMS): | vendor $(BIN_DIR)
	go build -mod=vendor -o $@ github.com/observatorium/opa-ams/test/mock

$(GOLANGCILINT):
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/$(GOLANGCILINT_VERSION)/install.sh \
		| sed -e '/install -d/d' \
		| sh -s -- -b $(FIRST_GOPATH)/bin $(GOLANGCILINT_VERSION)

$(SHELLCHECK): | $(BIN_DIR)
	curl -sNL "https://github.com/koalaman/shellcheck/releases/download/stable/shellcheck-stable.$(OS).$(ARCH).tar.xz" | tar --strip-components=1 -xJf - -C $(BIN_DIR)

$(MEMCACHED): | $(BIN_DIR) $(LIB_DIR)
	@echo "Downloading Memcached"
	curl -L https://archive.org/download/archlinux_pkg_libevent/libevent-2.1.10-1-x86_64.pkg.tar.xz | tar --strip-components=2 --xz -xf - -C $(LIB_DIR) usr/lib
	curl -L https://archive.org/download/archlinux_pkg_memcached/memcached-1.5.8-1-x86_64.pkg.tar.xz | tar --strip-components=2 --xz -xf - -C $(BIN_DIR) usr/bin/memcached
