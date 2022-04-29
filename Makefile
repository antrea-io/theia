SHELL              := /bin/bash
# go options
GO                 ?= go
LDFLAGS            :=
GOFLAGS            :=
BINDIR             ?= $(CURDIR)/bin
GO_FILES           := $(shell find . -type d -name '.cache' -prune -o -type f -name '*.go' -print)
GOPATH             ?= $$($(GO) env GOPATH)
DOCKER_CACHE       := $(CURDIR)/.cache
GO_VERSION         := $(shell head -n 1 build/images/deps/go-version)

DOCKER_BUILD_ARGS += --build-arg GO_VERSION=$(GO_VERSION)

.PHONY: all
all: build

UNAME_S := $(shell uname -s)

.PHONY: bin
bin:
	@mkdir -p $(BINDIR)
	GOOS=linux $(GO) build -o $(BINDIR) $(GOFLAGS) -ldflags '$(LDFLAGS)' antrea.io/theia/plugins/...

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
		golang:1.17

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
.linux-test-unit:
	@echo
	@echo "==> Running unit tests <=="
	$(GO) test -race -covermode=atomic -cover antrea.io/theia/cmd/... antrea.io/theia/pkg/... antrea.io/theia/plugins/... 

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
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $@ v1.41.1

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

.PHONY: manifest
manifest:
	@echo "===> Generating dev manifest for Theia <==="
	$(CURDIR)/hack/generate-manifest.sh --mode dev > build/yamls/flow-visibility.yml

.PHONY: verify
verify:
	@echo "===> Verifying spellings <==="
	GO=$(GO) $(CURDIR)/hack/verify-spelling.sh
	@echo "===> Verifying documentation formatting for website <==="
	$(CURDIR)/hack/verify-docs-for-website.sh

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
