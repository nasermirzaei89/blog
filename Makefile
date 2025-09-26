#@IgnoreInspection BashAddShebang
ROOT=$(realpath $(dir $(lastword $(MAKEFILE_LIST))))
CGO_ENABLED?=0
GO_CMD?=go

APP_NAME?=fullstackgo

IMAGE_REPOSITORY?=ghcr.io/nasermirzaei89/fullstackgo
IMAGE_TAG?=latest

# Install by `go get -tool github.com/golangci/golangci-lint/v2/cmd/golangci-lint@<SET VERSION>`
GOLANGCI_LINT_CMD=$(GO_CMD) tool golangci-lint

NPM_CMD?=npm
DOCKER_CMD?=docker

.DEFAULT_GOAL := .default

.default: format lint build test

.PHONY: help
help: ## Show help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: dep
dep: npm-install .which-go ## Install all dependencies
	$(GO_CMD) mod download

### Go

.which-go:
	@which $(GO_CMD) > /dev/null || (echo "Install Go from https://go.dev/doc/install" & exit 1)

.PHONY: run
run: build ## Run application
	$(ROOT)/bin/$(APP_NAME)

.PHONY: build
build: npm-build .which-go ## Build binary
	CGO_ENABLED=1 $(GO_CMD) build -v -o $(ROOT)/bin/$(APP_NAME) $(ROOT)/cmd/$(APP_NAME)

.PHONY: format
format: .which-go ## Format files
	$(GO_CMD) mod tidy
	$(GOLANGCI_LINT_CMD) fmt $(ROOT)/...

.PHONY: lint
lint: .which-go ## Lint files
	$(GOLANGCI_LINT_CMD) run $(ROOT)/...

.PHONY: test
test: .which-go ## Run tests
	CGO_ENABLED=1 $(GO_CMD) test -race -cover -coverprofile=coverage.out -covermode=atomic $(ROOT)/...

### Node

.which-npm:
	@which $(NPM_CMD) > /dev/null || (echo "Install NodeJS from https://nodejs.org/en/download" & exit 1)

.PHONY: npm-install
npm-install: .which-npm ## Install JS dependencies
	$(NPM_CMD) --prefix $(ROOT)/web install

.PHONY: npm-build
npm-build: .which-npm ## Build JS and CSS
	$(NPM_CMD) --prefix $(ROOT)/web run build:js
	$(NPM_CMD) --prefix $(ROOT)/web run build:css

### Docker

.which-docker:
	@which $(DOCKER_CMD) > /dev/null || (echo "Install Docker from https://www.docker.com/get-started/" & exit 1)

.PHONY: docker-build
docker-build: .which-docker ## Build docker image
	$(DOCKER_CMD) build -t $(IMAGE_REPOSITORY):$(IMAGE_TAG) $(ROOT)

.PHONY: docker-push
docker-push: .which-docker ## Push docker image
	$(DOCKER_CMD) push $(IMAGE_REPOSITORY):$(IMAGE_TAG)
