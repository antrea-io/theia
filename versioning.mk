# check if git is available
ifeq ($(shell which git),)
        VERSION_SUFFIX := unknown
else
        # Tree state is "dirty" if there are uncommitted changes, untracked files are ignored
        GIT_TREE_STATE := $(shell test -n "`git status --porcelain --untracked-files=no`" && echo "dirty" || echo "clean")
        ifeq ($(GIT_TREE_STATE),dirty)
                VERSION_SUFFIX := $(GIT_SHA).dirty
        else
                VERSION_SUFFIX := $(GIT_SHA)
        endif
endif

ifndef VERSION
        VERSION := $(shell head -n 1 VERSION)
        DOCKER_IMG_VERSION := $(VERSION)-$(VERSION_SUFFIX)
else
        DOCKER_IMG_VERSION := $(VERSION)
endif

version-info:
	@echo "===> Version information <==="
	@echo "VERSION: $(VERSION)"
	@echo "DOCKER_IMG_VERSION: $(DOCKER_IMG_VERSION)"
