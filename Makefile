GOBIN=$(shell pwd)/bin
GOFILES=$(wildcard *.go)
GONAME=dex-k8s-authenticator
TAG=latest
ARCH?=$(shell go env GOARCH)
PACKAGES_DIR=packages

all: build

.PHONY: build
build:
	@echo "Building $(GOFILES) to ./bin"
	GOBIN=$(GOBIN) go build -o bin/$(GONAME) .

.PHONY: melange-keygen
melange-keygen:
	melange keygen

.PHONY: melange-build
melange-build: melange-keygen
	melange build melange.yaml \
		--arch $(ARCH) \
		--signing-key melange.rsa \
		--out-dir $(PACKAGES_DIR)

.PHONY: image
image: melange-build
	apko build apko.yaml \
		$(GONAME):$(TAG) $(GONAME).tar \
		--arch $(ARCH) \
		--keyring-append melange.rsa.pub \
		--repository-append $(shell pwd)/$(PACKAGES_DIR)

.PHONY: image-load
image-load: image
	docker load < $(GONAME).tar

.PHONY: clean
clean:
	@echo "Cleaning"
	GOBIN=$(GOBIN) go clean
	rm -rf ./bin $(PACKAGES_DIR) melange.rsa melange.rsa.pub $(GONAME).tar

.PHONY: lint
lint:
	golangci-lint run

.PHONY: lint-fix
lint-fix: lint
	golangci-lint run --fix
