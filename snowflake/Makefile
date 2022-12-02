GO                 ?= go
BINDIR := $(CURDIR)/bin

all: bin

.PHONY: bin
bin:
	make -C udfs/udfs/
	$(GO) build -o $(BINDIR)/theia-sf antrea.io/theia/snowflake

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
	MOCKGEN_COPYRIGHT_FILE=$(CURDIR)/hack/boilerplate/license_header.raw.txt \
	$(GO) generate ./...

.PHONY: clean
clean:
	rm -rf bin
