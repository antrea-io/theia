SHELL                 := /bin/bash
# go options
GO                    ?= go
LDFLAGS               :=
GOFLAGS               :=
BINDIR                ?= $(CURDIR)/bin
GO_FILES              := $(shell find . -type d -name '.cache' -prune -o -type f -name '*.go' -print)
GOPATH                ?= $$($(GO) env GOPATH)
DOCKER_CACHE          := $(CURDIR)/.cache
THEIA_BINARY_NAME     ?= theia
GO_VERSION            := $(shell head -n 1 build/images/deps/go-version)

DOCKER_BUILD_ARGS = --build-arg GO_VERSION=$(GO_VERSION)

.PHONY: all
all: theia

include versioning.mk

VERSION_LDFLAGS = -X antrea.io/theia/pkg/version.Version=$(VERSION)
VERSION_LDFLAGS += -X antrea.io/theia/pkg/version.GitSHA=$(GIT_SHA)
VERSION_LDFLAGS += -X antrea.io/theia/pkg/version.GitTreeState=$(GIT_TREE_STATE)
VERSION_LDFLAGS += -X antrea.io/theia/pkg/version.ReleaseStatus=$(RELEASE_STATUS)

LDFLAGS += $(VERSION_LDFLAGS)

UNAME_S := $(shell uname -s)

.PHONY: bin
bin:
	@mkdir -p $(BINDIR)
	GOOS=linux $(GO) build -o $(BINDIR) $(GOFLAGS) -ldflags '$(LDFLAGS)' antrea.io/theia/plugins/...

.PHONY: .coverage
.coverage:
	mkdir -p $(CURDIR)/.coverage

.PHONY: test-unit
ifeq ($(UNAME_S),Linux)
test-unit: .linux-test-unit
else
test-unit:
	$(error Cannot use target 'test-unit' on OS $(UNAME_S), but you can run unit tests with 'docker-test-unit')
endif

.PHONY: test
test: golangci
test: docker-test-unit

$(DOCKER_CACHE):
	@mkdir -p $@/gopath
	@mkdir -p $@/gocache

# Since the WORKDIR is mounted from host, the $(id -u):$(id -g) user can access it.
# Inside the docker, the user is nameless and does not have a home directory. This is ok for our use case.
DOCKER_ENV := \
	@docker run --rm -u $$(id -u):$$(id -g) \
		-e "GOCACHE=/tmp/gocache" \
		-e "GOPATH=/tmp/gopath" \
		-w /usr/src/antrea.io/theia \
		-v $(DOCKER_CACHE)/gopath:/tmp/gopath \
		-v $(DOCKER_CACHE)/gocache:/tmp/gocache \
		-v $(CURDIR):/usr/src/antrea.io/theia \
		golang:1.19

.PHONY: docker-test-unit
docker-test-unit: $(DOCKER_CACHE)
	@$(DOCKER_ENV) make test-unit
	@chmod -R 0755 $<

.PHONY: docker-tidy
docker-tidy: $(DOCKER_CACHE)
	@rm -f go.sum
	@$(DOCKER_ENV) $(GO) mod tidy

.PHONY: check-copyright
check-copyright: 
	@GO=$(GO) $(CURDIR)/hack/add-license.sh

.PHONY: add-copyright
add-copyright: 
	@GO=$(GO) $(CURDIR)/hack/add-license.sh --add

.PHONY: .linux-test-unit
.linux-test-unit: .coverage
	@echo
	@echo "==> Running unit tests <=="
	$(GO) test -race -coverpkg=antrea.io/theia/plugins/...,antrea.io/theia/pkg/...  \
	  -coverprofile=.coverage/coverage-unit.txt -covermode=atomic \
	  antrea.io/theia/plugins/... antrea.io/theia/pkg/... 

.PHONY: tidy
tidy:
	@rm -f go.sum
	@$(GO) mod tidy

test-tidy:
	@echo
	@echo "===> Checking go.mod tidiness <==="
	@GO=$(GO) $(CURDIR)/hack/tidy-check.sh

.PHONY: fmt
fmt:
	@echo
	@echo "===> Formatting Go files <==="
	@gofmt -s -l -w $(GO_FILES)

.golangci-bin:
	@echo "===> Installing Golangci-lint <==="
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $@ v1.50.0

.PHONY: golangci
golangci: .golangci-bin
	@echo "===> Running golangci (linux) <==="
	@GOOS=linux $(CURDIR)/.golangci-bin/golangci-lint run -c $(CURDIR)/.golangci.yml

.PHONY: golangci-fix
golangci-fix: .golangci-bin
	@echo "===> Running golangci (linux) <==="
	@GOOS=linux $(CURDIR)/.golangci-bin/golangci-lint run -c $(CURDIR)/.golangci.yml --fix

.PHONY: clean
clean:
	@rm -rf $(BINDIR)
	@rm -rf $(DOCKER_CACHE)
	@rm -rf .golangci-bin

.PHONY: codegen
codegen:
	@echo "===> Updating generated code <==="
	$(CURDIR)/hack/update-codegen.sh

.PHONY: manifest
manifest:
	@echo "===> Generating dev manifest for Theia <==="
	$(CURDIR)/hack/generate-manifest.sh --mode dev > build/yamls/flow-visibility.yml

.PHONY: verify
verify:
	@echo "===> Verifying spellings <==="
	GO=$(GO) $(CURDIR)/hack/verify-spelling.sh
	@echo "===> Verifying Table of Contents <==="
	GO=$(GO) $(CURDIR)/hack/verify-toc.sh
	@echo "===> Verifying documentation formatting for website <==="
	$(CURDIR)/hack/verify-docs-for-website.sh

.PHONY: toc
toc:
	@echo "===> Generating Table of Contents for Theia docs <==="
	GO=$(GO) $(CURDIR)/hack/update-toc.sh

.PHONE: markdownlint
markdownlint:
	@echo "===> Running markdownlint <==="
	markdownlint -c .markdownlint-config.yml -i CHANGELOG/ -i CHANGELOG.md -i CODE_OF_CONDUCT.md .

.PHONE: markdownlint-fix
markdownlint-fix:
	@echo "===> Running markdownlint <==="
	markdownlint --fix -c .markdownlint-config.yml -i CHANGELOG/ -i CHANGELOG.md -i CODE_OF_CONDUCT.md .

.PHONY: spelling-fix
spelling-fix:
	@echo "===> Updating incorrect spellings <==="
	$(CURDIR)/hack/update-spelling.sh

.PHONY: clickhouse-monitor
clickhouse-monitor:
	@echo "===> Building antrea/theia-clickhouse-monitor Docker image <==="
	docker build --pull -t antrea/theia-clickhouse-monitor:$(DOCKER_IMG_VERSION) -f build/images/Dockerfile.clickhouse-monitor.ubuntu $(DOCKER_BUILD_ARGS) .
	docker tag antrea/theia-clickhouse-monitor:$(DOCKER_IMG_VERSION) antrea/theia-clickhouse-monitor
	docker tag antrea/theia-clickhouse-monitor:$(DOCKER_IMG_VERSION) projects.registry.vmware.com/antrea/theia-clickhouse-monitor
	docker tag antrea/theia-clickhouse-monitor:$(DOCKER_IMG_VERSION) projects.registry.vmware.com/antrea/theia-clickhouse-monitor:$(DOCKER_IMG_VERSION)

.PHONY: clickhouse-monitor-plugin
clickhouse-monitor-plugin:
	@mkdir -p $(BINDIR)
	GOOS=linux $(GO) build -o $(BINDIR) $(GOFLAGS) -ldflags '$(LDFLAGS)' antrea.io/theia/plugins/clickhouse-monitor

.PHONY: theia-manager
theia-manager:
	@echo "===> Building antrea/theia-manager Docker image <==="
	docker build --pull -t antrea/theia-manager:$(DOCKER_IMG_VERSION) -f build/images/Dockerfile.theia-manager.ubuntu $(DOCKER_BUILD_ARGS) .
	docker tag antrea/theia-manager:$(DOCKER_IMG_VERSION) antrea/theia-manager
	docker tag antrea/theia-manager:$(DOCKER_IMG_VERSION) projects.registry.vmware.com/antrea/theia-manager
	docker tag antrea/theia-manager:$(DOCKER_IMG_VERSION) projects.registry.vmware.com/antrea/theia-manager:$(DOCKER_IMG_VERSION)

.PHONY: theia-manager-bin
theia-manager-bin:
	@mkdir -p $(BINDIR)
	GOOS=linux $(GO) build -o $(BINDIR) $(GOFLAGS) -ldflags '$(LDFLAGS)' antrea.io/theia/cmd/theia-manager

.PHONY: clickhouse-server
clickhouse-server:
	@echo "===> Building antrea/theia-clickhouse-server Docker image <==="
	docker build --pull -t antrea/theia-clickhouse-server:$(DOCKER_IMG_VERSION) -f build/images/Dockerfile.clickhouse-server.ubuntu $(DOCKER_BUILD_ARGS) .
	docker tag antrea/theia-clickhouse-server:$(DOCKER_IMG_VERSION) antrea/theia-clickhouse-server
	docker tag antrea/theia-clickhouse-server:$(DOCKER_IMG_VERSION) projects.registry.vmware.com/antrea/theia-clickhouse-server
	docker tag antrea/theia-clickhouse-server:$(DOCKER_IMG_VERSION) projects.registry.vmware.com/antrea/theia-clickhouse-server:$(DOCKER_IMG_VERSION)

.PHONY: clickhouse-server-multi-arch
clickhouse-server-multi-arch:
	@echo "===> Building antrea/theia-clickhouse-server Docker image <==="
	docker buildx build --platform=linux/amd64,linux/arm64 --push --pull -t antrea/theia-clickhouse-server:$(VERSION) -f build/images/Dockerfile.clickhouse-server.ubuntu $(DOCKER_BUILD_ARGS) .

.PHONY: clickhouse-schema-management-plugin
clickhouse-schema-management-plugin:
	@mkdir -p $(BINDIR)
	GOOS=linux $(GO) build -o $(BINDIR) $(GOFLAGS) -ldflags '$(LDFLAGS)' antrea.io/theia/plugins/clickhouse-schema-management

# Theia currently supports two spark jobs, Throughput Anomaly Detection and Policy Recommendation.
# This Dockerfile helps create unified dockerfile for both the spark jobs.
.PHONY: spark-jobs
spark-jobs:
	@echo "===> Building antrea/theia-spark-jobs Docker image <==="
	docker build --pull -t antrea/theia-spark-jobs:$(DOCKER_IMG_VERSION) -f build/images/Dockerfile.spark-jobs.ubuntu .
	docker tag antrea/theia-spark-jobs:$(DOCKER_IMG_VERSION) antrea/theia-spark-jobs
	docker tag antrea/theia-spark-jobs:$(DOCKER_IMG_VERSION) projects.registry.vmware.com/antrea/theia-spark-jobs
	docker tag antrea/theia-spark-jobs:$(DOCKER_IMG_VERSION) projects.registry.vmware.com/antrea/theia-spark-jobs:$(DOCKER_IMG_VERSION)

THEIA_BINARIES := theia-darwin theia-linux theia-windows
$(THEIA_BINARIES): theia-%:
	@GOOS=$* $(GO) build -o $(BINDIR)/$@ $(GOFLAGS) -ldflags '$(LDFLAGS)' antrea.io/theia/pkg/theia
	@if [[ $@ != *windows ]]; then \
	  chmod 0755 $(BINDIR)/$@; \
	else \
	  mv $(BINDIR)/$@ $(BINDIR)/$@.exe; \
	fi

.PHONY: theia
theia: $(THEIA_BINARIES)

.PHONY: theia-release
theia-release:
	@$(GO) build -o $(BINDIR)/$(THEIA_BINARY_NAME) $(GOFLAGS) -ldflags '-s -w $(LDFLAGS)' antrea.io/theia/pkg/theia

