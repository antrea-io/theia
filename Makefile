GO                 ?= go
LDFLAGS            :=
GOFLAGS            :=
BINDIR             ?= $(CURDIR)/bin
GO_VERSION         := $(shell head -n 1 build/images/deps/go-version)

DOCKER_BUILD_ARGS = --build-arg GO_VERSION=$(GO_VERSION)

include versioning.mk

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
