GO                 ?= go
LDFLAGS            :=
GOFLAGS            :=
BINDIR             ?= $(CURDIR)/bin
GO_FILES           := $(shell find . -type d -name '.cache' -prune -o -type f -name '*.go' -print)
CLI_BINARY_NAME    ?= theia-sf

GOLANGCI_LINT_VERSION := v1.54.2
GOLANGCI_LINT_BINDIR  := $(CURDIR)/.golangci-bin
GOLANGCI_LINT_BIN     := $(GOLANGCI_LINT_BINDIR)/$(GOLANGCI_LINT_VERSION)/golangci-lint

.PHONY: all
all: udfs bin

VERSION_FILE := ../VERSION
include ../versioning.mk

VERSION_LDFLAGS = -X antrea.io/theia/snowflake/pkg/version.Version=$(VERSION)
VERSION_LDFLAGS += -X antrea.io/theia/snowflake/pkg/version.GitSHA=$(GIT_SHA)
VERSION_LDFLAGS += -X antrea.io/theia/snowflake/pkg/version.GitTreeState=$(GIT_TREE_STATE)
VERSION_LDFLAGS += -X antrea.io/theia/snowflake/pkg/version.ReleaseStatus=$(RELEASE_STATUS)

LDFLAGS += $(VERSION_LDFLAGS)

.PHONY: udfs
udfs:
	make -C udfs/udfs/

.PHONY: bin
bin:
	$(GO) build -o $(BINDIR)/$(CLI_BINARY_NAME) $(GOFLAGS) -ldflags '-s -w $(LDFLAGS)' antrea.io/theia/snowflake

.PHONY: .coverage
.coverage:
	mkdir -p $(CURDIR)/.coverage

.PHONY: test-unit
test-unit: .coverage
	@echo
	@echo "==> Running unit tests <=="
	$(GO) test -race -coverpkg=antrea.io/theia/snowflake/cmd/...,antrea.io/theia/snowflake/pkg/... \
	  -coverprofile=.coverage/coverage-unit.txt -covermode=atomic \
	  antrea.io/theia/snowflake/cmd/... antrea.io/theia/snowflake/pkg/... 

.PHONY: generate
generate:
        MOCKGEN_COPYRIGHT_FILE=$(CURDIR)/hack/boilerplate/license_header.raw.txt $(GO) generate ./...

.PHONY: clean
clean:
	@rm -rf $(BINDIR)
	@rm -rf $(GOLANGCI_LINT_BINDIR)

.PHONY: tidy
tidy:
	@rm -f go.sum
	@$(GO) mod tidy

test-tidy:
	@echo "===> Checking go.mod tidiness <==="
	@GO=$(GO) $(CURDIR)/hack/tidy-check.sh

.PHONY: docker-tidy
docker-tidy: $(DOCKER_CACHE)
	@rm -f go.sum
	@$(DOCKER_ENV) $(GO) mod tidy

.PHONY: fmt
fmt:
	@echo "===> Formatting Go files <==="
	@gofmt -s -l -w $(GO_FILES)

$(GOLANGCI_LINT_BIN):
	@rm -rf $(GOLANGCI_LINT_BINDIR)/* # remove old versions
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOLANGCI_LINT_BINDIR)/$(GOLANGCI_LINT_VERSION) $(GOLANGCI_LINT_VERSION)

.PHONY: golangci
golangci: $(GOLANGCI_LINT_BIN)
	@echo "===> Running golangci (linux) <==="
	@GOOS=linux $(GOLANGCI_LINT_BIN) run -c $(CURDIR)/.golangci.yml

.PHONY: golangci-fix
golangci-fix: $(GOLANGCI_LINT_BIN)
	@echo "===> Running golangci (linux) <==="
	@GOOS=linux $(GOLANGCI_LINT_BIN) run -c $(CURDIR)/.golangci.yml --fix
