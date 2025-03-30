BUILD_DIR := build

GO_SRCS := $(shell find . -name '*.go' -o -name 'go.mod' -o -name 'go.sum')
GOOSs := darwin linux
GOARCHs := amd64 arm64

UPDATER_BINARIES := $(foreach GOOS,$(GOOSs),$(foreach GOARCH,$(GOARCHs),$(BUILD_DIR)/photon-db-updater-$(GOOS)-$(GOARCH)))
WRAPPER_BINARIES := $(foreach GOOS,$(GOOSs),$(foreach GOARCH,$(GOARCHs),$(BUILD_DIR)/photon-wrapper-$(GOOS)-$(GOARCH)))

.PHONY: all
all: $(UPDATER_BINARIES) $(WRAPPER_BINARIES)

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

$(BUILD_DIR)/%: $(GO_SRCS) | $(BUILD_DIR)
	CGO_ENABLED=0 go build -trimpath -o $@ ./cmd/$*

define go-cross-build
$(BUILD_DIR)/$(1)-$(2)-$(3): $(GO_SRCS) | $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=$(2) GOARCH=$(3) \
		go build -trimpath -o $$@ ./cmd/$(1)
endef
$(foreach GOOS,$(GOOSs),$(foreach GOARCH,$(GOARCHs),$(eval $(call go-cross-build,photon-db-updater,$(GOOS),$(GOARCH)))))
$(foreach GOOS,$(GOOSs),$(foreach GOARCH,$(GOARCHs),$(eval $(call go-cross-build,photon-wrapper,$(GOOS),$(GOARCH)))))

.PHONY: archive
archive: build.tar

build.tar: $(UPDATER_BINARIES) $(WRAPPER_BINARIES)
	tar cf $@ \
		$(UPDATER_BINARIES) \
		$(WRAPPER_BINARIES)

.PHONY: image
image: all
	source .env && \
	docker build \
		-t ghcr.io/pddg/photon:latest \
		--build-arg PHOTON_VERSION=$${PHOTON_VERSION} \
		--build-arg PHOTON_SHA256SUM=$${PHOTON_SHA256SUM} \
		--build-arg GIT_SHA=$(shell git rev-parse HEAD) \
		.

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)

.PHONY: test
test:
	go test -race -vet=all ./internal/...

.PHONY: test-e2e
test-e2e: $(UPDATER_BINARIES)
	go test -vet=all ./e2e/...
