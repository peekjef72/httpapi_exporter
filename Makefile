# Copyright 2015 The Prometheus Authors
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

GO     := go
GOPATH := $(firstword $(subst :, ,$(shell $(GO) env GOPATH)))
PROMU  := $(GOPATH)/bin/promu
PASSWD_ENCRYPT := $(GOPATH)/bin/passwd_encrypt
pkgs    = $(shell $(GO) list ./... | grep -v /vendor/)

PREFIX              ?= $(shell pwd)
BIN_DIR             ?= $(shell pwd)
DOCKER_IMAGE_NAME   ?= httpapi_exporter
DOCKER_IMAGE_TAG    ?= latest


all: promu build test

style:
	@echo ">> checking code style"
	@! gofmt -d $(shell find . -path ./vendor -prune -o -name '*.go' -print) | grep '^'

test:
	@echo ">> running tests"
	@$(GO) test -short -race $(pkgs)

format:
	@echo ">> formatting code"
	@$(GO) fmt $(pkgs)

vet:
	@echo ">> vetting code"
	@$(GO) vet $(pkgs)

build: promu passwd_encrypt
	@echo ">> building binaries"
	@$(PROMU) build --prefix $(PREFIX)

tarball: promu passwd_encrypt
	@echo ">> building release tarball"
	@cp $(PASSWD_ENCRYPT) $(BIN_DIR)
	@$(PROMU) tarball --prefix $(PREFIX) $(BIN_DIR)
	@rm $(BIN_DIR)/passwd_encrypt

docker:
	@echo ">> building docker image"
	@docker build -t "$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)" .

promu:
	$(GO) install github.com/prometheus/promu@latest

passwd_encrypt:
	$(GO) install github.com/peekjef72/passwd_encrypt@latest

.PHONY: all style format build test vet tarball docker promu
