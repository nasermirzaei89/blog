#@IgnoreInspection BashAddShebang
ROOT=$(realpath $(dir $(lastword $(MAKEFILE_LIST))))
CGO_ENABLED?=0

APP_NAME?=blog

IMAGE_REPOSITORY?=ghcr.io/nasermirzaei89/blog
IMAGE_TAG?=latest

.DEFAULT_GOAL := .default

.default: format build test

.PHONY: help
help: ## Show help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

### Go

.which-go:
	@which go > /dev/null || (echo "Install Go from https://go.dev/doc/install" & exit 1)

.PHONY: build
build: .which-go ## Build binary
	CGO_ENABLED=1 go build -v -o $(ROOT)/bin/$(APP_NAME) $(ROOT)

.PHONY: format
format: .which-go ## Format files
	go mod tidy
	gofmt -s -w $(ROOT)

.PHONY: test
test: .which-go ## Run tests
	CGO_ENABLED=1 go test -race -cover $(ROOT)/...

### Node

.which-npm:
	@which npm > /dev/null || (echo "Install NodeJS from https://nodejs.org/en/download" & exit 1)

.PHONY: npm-build
npm-build: .which-npm ## Build JS and CSS
	npm run build:js
	npm run build:css

### Docker

.which-docker:
	@which docker > /dev/null || (echo "Install Docker from https://www.docker.com/get-started/" & exit 1)

.PHONY: docker-build
docker-build: .which-docker ## Build docker image
	docker build -t $(IMAGE_REPOSITORY):$(IMAGE_TAG) $(ROOT)

.PHONY: docker-push
docker-push: .which-docker ## Push docker image
	docker push $(IMAGE_REPOSITORY):$(IMAGE_TAG)
