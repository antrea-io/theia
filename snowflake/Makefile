GO                 ?= go
BINDIR := $(CURDIR)/bin

all: bin

.PHONY: bin
bin:
	$(GO) build -o $(BINDIR)/theia-sf antrea.io/theia/snowflake

.PHONY: test
test:
	$(GO) test -v ./...

.PHONY: clean
clean:
	rm -rf bin
