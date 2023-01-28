GO                 ?= go
LDFLAGS            :=
GOFLAGS            :=
BINDIR             ?= $(CURDIR)/bin
CLI_BINARY_NAME    ?= theia-sf

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
	rm -rf bin
