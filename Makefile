BUILD_DIR := build

GO_SRCS := $(shell find . -name '*.go' -o -name 'go.mod' -o -name 'go.sum')
GOOSs := darwin linux
GOARCHs := amd64 arm64

UPDATER_BINARIES := $(foreach GOOS,$(GOOSs),$(foreach GOARCH,$(GOARCHs),$(BUILD_DIR)/photon-db-updater-$(GOOS)-$(GOARCH)))
AGENT_BINARIES := $(foreach GOOS,$(GOOSs),$(foreach GOARCH,$(GOARCHs),$(BUILD_DIR)/photon-agent-$(GOOS)-$(GOARCH)))

.PHONY: all
all: $(UPDATER_BINARIES) $(AGENT_BINARIES)

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
$(foreach GOOS,$(GOOSs),$(foreach GOARCH,$(GOARCHs),$(eval $(call go-cross-build,photon-agent,$(GOOS),$(GOARCH)))))

.PHONY: archive
archive: build.tar

build.tar: $(UPDATER_BINARIES) $(AGENT_BINARIES)
	tar cf $@ $^

.PHONY: image
image: all
	. ./.env && \
	docker build \
		-t ghcr.io/pddg/photon:latest \
		--build-arg PHOTON_VERSION=$${PHOTON_VERSION} \
		--build-arg PHOTON_SHA256SUM=$${PHOTON_SHA256SUM} \
		--build-arg GIT_SHA=$(shell git rev-parse HEAD) \
		.

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)

.PHONY: lint
lint:
	golangci-lint run --timeout 5m ./...

.PHONY: lint-fix
lint-fix:
	golangci-lint run --fix --timeout 5m ./...

.PHONY: fmt
fmt:
	golangci-lint fmt ./...

.PHONY: test
test: lint
	go test -race -vet=off ./internal/...

.PHONY: test-e2e
test-e2e: image $(UPDATER_BINARIES)
	go test -vet=off ./e2e/...
