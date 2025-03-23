BUILD_DIR := build

GO_SRCS := $(shell find . -name '*.go' -o -name 'go.mod' -o -name 'go.sum')

.PHONY: all
all: $(BUILD_DIR)/photon-wrapper $(BUILD_DIR)/photon-db-updater

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

$(BUILD_DIR)/%: $(GO_SRCS) | $(BUILD_DIR)
	CGO_ENABLED=0 go build -trimpath -o $@ ./cmd/$*

.PHONY: image
image:
	source .env && \
	docker build \
		-t photon-wrapper:latest \
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
test-e2e:
	./e2e/test.sh
