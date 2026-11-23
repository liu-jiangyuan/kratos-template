# GNU Make falls back to cmd.exe on Windows when sh.exe is not in PATH, which
# breaks POSIX recipes (find/unzip/2>/dev/null/...). When running from cmd.exe,
# derive Git Bash from `where git` and use it for every recipe.
#
# Detect the actual shell by echoing $0: cmd.exe prints "$0" literally, while a
# POSIX shell prints /usr/bin/sh or similar. Using `ifeq ($(SHELL),sh.exe)` here
# would also match Git Bash (Make's default value) and create a stray NUL file.
NULL_DEVICE:=/dev/null
ifeq ($(OS),Windows_NT)
	MAKE_SHELL_ID:=$(shell echo $$0)
	ifeq ($(MAKE_SHELL_ID),$$0)
		NULL_DEVICE:=NUL
		GIT_CMD:=$(shell where git 2>NUL | findstr /I /C:"\cmd\git.exe")
		ifeq ($(strip $(GIT_CMD)),)
			GIT_CMD:=$(lastword $(shell where git 2>NUL))
		endif
		GIT_BASH:=$(subst \cmd\git.exe,\bin\bash.exe,$(GIT_CMD))
		GIT_BASH:=$(subst \ucrt64\bin\git.exe,\bin\bash.exe,$(GIT_BASH))
		ifneq ($(strip $(GIT_BASH)),)
			SHELL:=$(GIT_BASH)
			GIT_USR_BIN:=$(subst \bin\bash.exe,\usr\bin,$(GIT_BASH))
			export PATH:=$(subst \,/,$(GIT_USR_BIN)):$(PATH)
			NULL_DEVICE:=/dev/null
		endif
	endif
endif

GOHOSTOS:=$(shell go env GOHOSTOS)
GOHOSTARCH:=$(shell go env GOHOSTARCH)
ifeq ($(GOHOSTOS),windows)
	EXE:=.exe
endif

VERSION?=$(shell git describe --tags --always 2>$(NULL_DEVICE))
APP?=$(notdir $(firstword $(wildcard ./cmd/*)))
# Ent v0.14.6's generator needs the Go version declared in go.mod; newer
# toolchains (Go 1.27+) currently fail inside its go/packages loader.
GO_VERSION?=$(shell go list -m -f '{{.GoVersion}}' 2>$(NULL_DEVICE))

# Local toolchain fallback. Missing tools are installed under .tools/ so the
# template does not touch the caller's global GOPATH/bin; PATH tools are reused.
TOOLS_DIR:=$(subst \,/,$(CURDIR))/.tools
TOOLS_BIN:=$(TOOLS_DIR)/bin
PROTOC_VERSION?=36.2
PROTOC_DIR?=$(TOOLS_DIR)/protoc

# Base URL written into openapi.yaml. Keep this in sync with
# configs/config.yaml server.http.addr when testing from another host/port.
OPENAPI_SERVER_URL?=http://127.0.0.1:8000
OPENAPI_SERVER_DESC?=Local demo server

# google/api and google/protobuf/descriptor.proto come from the official Kratos
# module instead of a checked-in third_party copy.
KRATOS_MOD:=$(subst \,/,$(shell go list -m -f '{{.Dir}}' github.com/go-kratos/kratos/v3 2>$(NULL_DEVICE)))
KRATOS_THIRD_PARTY:=$(KRATOS_MOD)/third_party

ifeq ($(GOHOSTOS),windows)
	PROTOC_ASSET:=protoc-$(PROTOC_VERSION)-win64.zip
	PROTOC_BIN:=$(PROTOC_DIR)/bin/protoc.exe
else ifeq ($(GOHOSTOS),darwin)
	ifeq ($(GOHOSTARCH),arm64)
		PROTOC_ASSET:=protoc-$(PROTOC_VERSION)-osx-aarch_64.zip
	else
		PROTOC_ASSET:=protoc-$(PROTOC_VERSION)-osx-x86_64.zip
	endif
	PROTOC_BIN:=$(PROTOC_DIR)/bin/protoc
else
	ifeq ($(GOHOSTARCH),arm64)
		PROTOC_ASSET:=protoc-$(PROTOC_VERSION)-linux-aarch_64.zip
	else
		PROTOC_ASSET:=protoc-$(PROTOC_VERSION)-linux-x86_64.zip
	endif
	PROTOC_BIN:=$(PROTOC_DIR)/bin/protoc
endif

PROTOC_URL:=https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/$(PROTOC_ASSET)

# Tool resolution order:
#   1. explicit environment / command-line variable (PROTOC, PROTOC_INCLUDE, ...)
#   2. an executable already in PATH
#   3. the local .tools fallback installed by `make init`
# Nothing is downloaded automatically except by `make init`.
PATH_PROTOC:=$(shell command -v protoc 2>$(NULL_DEVICE))
ifeq ($(origin PROTOC),undefined)
	ifneq ($(strip $(PATH_PROTOC)),)
		PROTOC:=$(PATH_PROTOC)
		PROTOC_EXTERNAL:=1
	else
		PROTOC:=$(PROTOC_BIN)
	endif
else
	PROTOC_EXTERNAL:=1
endif

PROTOC_RESOLVED:=$(shell command -v "$(PROTOC)" 2>$(NULL_DEVICE))
ifeq ($(strip $(PROTOC_RESOLVED)),)
	PROTOC_RESOLVED:=$(PROTOC)
endif
ifeq ($(GOHOSTOS),windows)
	# Native protoc.exe does not understand Git Bash /c/... paths.
	PROTOC_WIN:=$(shell cygpath -w "$(PROTOC_RESOLVED)" 2>$(NULL_DEVICE))
	ifneq ($(strip $(PROTOC_WIN)),)
		PROTOC:=$(PROTOC_WIN)
	endif
endif

ifeq ($(origin PROTOC_INCLUDE),undefined)
	ifeq ($(PROTOC_EXTERNAL),1)
		PROTOC_FWD:=$(subst \,/,$(PROTOC))
		PROTOC_ROOT:=$(patsubst %/bin/protoc.exe,%,$(PROTOC_FWD))
		ifeq ($(PROTOC_ROOT),$(PROTOC_FWD))
			PROTOC_INCLUDE:=$(abspath $(dir $(PROTOC_FWD))../include)
		else
			PROTOC_INCLUDE:=$(PROTOC_ROOT)/include
		endif
	else
		PROTOC_INCLUDE:=$(PROTOC_DIR)/include
	endif
endif

PATH_PROTOC_GEN_GO:=$(shell command -v protoc-gen-go 2>$(NULL_DEVICE))
ifeq ($(origin PROTOC_GEN_GO),undefined)
	PROTOC_GEN_GO:=$(if $(strip $(PATH_PROTOC_GEN_GO)),$(PATH_PROTOC_GEN_GO),$(TOOLS_BIN)/protoc-gen-go$(EXE))
endif
PATH_PROTOC_GEN_GO_HTTP:=$(shell command -v protoc-gen-go-http 2>$(NULL_DEVICE))
ifeq ($(origin PROTOC_GEN_GO_HTTP),undefined)
	PROTOC_GEN_GO_HTTP:=$(if $(strip $(PATH_PROTOC_GEN_GO_HTTP)),$(PATH_PROTOC_GEN_GO_HTTP),$(TOOLS_BIN)/protoc-gen-go-http$(EXE))
endif
PATH_PROTOC_GEN_OPENAPI:=$(shell command -v protoc-gen-openapi 2>$(NULL_DEVICE))
ifeq ($(origin PROTOC_GEN_OPENAPI),undefined)
	PROTOC_GEN_OPENAPI:=$(if $(strip $(PATH_PROTOC_GEN_OPENAPI)),$(PATH_PROTOC_GEN_OPENAPI),$(TOOLS_BIN)/protoc-gen-openapi$(EXE))
endif

ifeq ($(GOHOSTOS),windows)
	PROTOC_GEN_GO_WIN:=$(shell cygpath -w "$(PROTOC_GEN_GO)" 2>$(NULL_DEVICE))
	ifneq ($(strip $(PROTOC_GEN_GO_WIN)),)
		PROTOC_GEN_GO:=$(PROTOC_GEN_GO_WIN)
	endif
	PROTOC_GEN_GO_HTTP_WIN:=$(shell cygpath -w "$(PROTOC_GEN_GO_HTTP)" 2>$(NULL_DEVICE))
	ifneq ($(strip $(PROTOC_GEN_GO_HTTP_WIN)),)
		PROTOC_GEN_GO_HTTP:=$(PROTOC_GEN_GO_HTTP_WIN)
	endif
	PROTOC_GEN_OPENAPI_WIN:=$(shell cygpath -w "$(PROTOC_GEN_OPENAPI)" 2>$(NULL_DEVICE))
	ifneq ($(strip $(PROTOC_GEN_OPENAPI_WIN)),)
		PROTOC_GEN_OPENAPI:=$(PROTOC_GEN_OPENAPI_WIN)
	endif
endif

# Pass --plugin only when the resolved value is a path; a bare command name
# must be left for protoc to find in PATH.
PROTOC_GEN_GO_FLAG:=$(if $(or $(findstring /,$(PROTOC_GEN_GO)),$(findstring \,$(PROTOC_GEN_GO))),--plugin=protoc-gen-go="$(PROTOC_GEN_GO)",)
PROTOC_GEN_GO_HTTP_FLAG:=$(if $(or $(findstring /,$(PROTOC_GEN_GO_HTTP)),$(findstring \,$(PROTOC_GEN_GO_HTTP))),--plugin=protoc-gen-go-http="$(PROTOC_GEN_GO_HTTP)",)
PROTOC_GEN_OPENAPI_FLAG:=$(if $(or $(findstring /,$(PROTOC_GEN_OPENAPI)),$(findstring \,$(PROTOC_GEN_OPENAPI))),--plugin=protoc-gen-openapi="$(PROTOC_GEN_OPENAPI)",)

CONFIG_PROTO_FILES=$(shell find core/conf -name '*.proto')
API_PROTO_FILES=$(shell find api -name '*.proto')

.PHONY: init
# ensure protoc and generators (PATH tools win; missing ones go into .tools/)
init: tools protoc

.PHONY: tools
# install only missing Go-based generators into .tools/bin; PATH tools win
tools:
	@mkdir -p $(TOOLS_BIN)
	@install_tool() { \
		name=$$1; pkg=$$2; \
		if command -v $$name >/dev/null 2>&1; then \
			echo "using existing $$name: $$(command -v $$name)"; \
		elif [ -x "$(TOOLS_BIN)/$$name$(EXE)" ]; then \
			echo "using existing $(TOOLS_BIN)/$$name$(EXE)"; \
		else \
			echo "installing $$name into $(TOOLS_BIN)"; \
			GOBIN=$(TOOLS_BIN) go install $$pkg || exit 1; \
		fi; \
	}; \
	install_tool protoc-gen-go google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11; \
	install_tool protoc-gen-go-http github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v3@latest; \
	install_tool protoc-gen-openapi github.com/google/gnostic/cmd/protoc-gen-openapi@latest; \
	install_tool kratos github.com/go-kratos/kratos/cmd/kratos/v3@latest; \
	install_tool wire github.com/google/wire/cmd/wire@v0.7.0; \
	install_tool ent entgo.io/ent/cmd/ent@v0.14.6

.PHONY: protoc
# download the official protoc release into .tools/protoc when no system protoc
protoc:
	@if [ "$(PROTOC_EXTERNAL)" = "1" ]; then \
		echo "using existing protoc: $(PROTOC)"; \
	elif [ -x "$(PROTOC_BIN)" ]; then \
		echo "protoc already installed: $$($(PROTOC_BIN) --version)"; \
	else \
		echo "downloading $(PROTOC_URL)"; \
		mkdir -p "$(PROTOC_DIR)"; \
		curl -fsSL -o "$(PROTOC_DIR)/protoc.zip" "$(PROTOC_URL)"; \
		if command -v unzip >/dev/null 2>&1; then \
			unzip -o -q "$(PROTOC_DIR)/protoc.zip" -d "$(PROTOC_DIR)"; \
		else \
			powershell -NoProfile -Command "Expand-Archive -Force '$(PROTOC_DIR)/protoc.zip' '$(PROTOC_DIR)'"; \
		fi; \
		rm -f "$(PROTOC_DIR)/protoc.zip"; \
		"$(PROTOC_BIN)" --version; \
	fi

.PHONY: check-tools
check-tools:
	@command -v "$(PROTOC)" >/dev/null 2>&1 || [ -x "$(PROTOC)" ] || { \
		echo "protoc not found: $(PROTOC)"; \
		echo "run 'make init' or set PROTOC/PROTOC_INCLUDE"; exit 1; }
	@test -f "$(PROTOC_INCLUDE)/google/protobuf/duration.proto" || { \
		echo "protoc include not found or incomplete: $(PROTOC_INCLUDE)"; \
		echo "set PROTOC_INCLUDE to the directory containing google/protobuf/*.proto"; exit 1; }
	@for tool in "$(PROTOC_GEN_GO)" "$(PROTOC_GEN_GO_HTTP)"; do \
		command -v "$$tool" >/dev/null 2>&1 || [ -x "$$tool" ] || { \
			echo "protoc plugin not found: $$tool"; \
			echo "run 'make init' or set PROTOC_GEN_GO/PROTOC_GEN_GO_HTTP"; exit 1; }; \
	done
	@test -d "$(KRATOS_THIRD_PARTY)" || { echo "kratos third_party not found: run 'go mod download'"; exit 1; }

.PHONY: check-openapi-tools
check-openapi-tools: check-tools
	@command -v "$(PROTOC_GEN_OPENAPI)" >/dev/null 2>&1 || [ -x "$(PROTOC_GEN_OPENAPI)" ] || { \
		echo "protoc-gen-openapi not found: $(PROTOC_GEN_OPENAPI)"; \
		echo "run 'make init' or set PROTOC_GEN_OPENAPI"; exit 1; }

.PHONY: tools-env
# print the resolved protoc/plugin paths and their source
tools-env:
	@echo "PROTOC=$(PROTOC)"
	@echo "PROTOC_INCLUDE=$(PROTOC_INCLUDE)"
	@echo "PROTOC_GEN_GO=$(PROTOC_GEN_GO)"
	@echo "PROTOC_GEN_GO_HTTP=$(PROTOC_GEN_GO_HTTP)"
	@echo "PROTOC_GEN_OPENAPI=$(PROTOC_GEN_OPENAPI)"
	@echo "TOOLS_BIN=$(TOOLS_BIN)"

.PHONY: config
# generate core/conf proto
config: check-tools
	"$(PROTOC)" --proto_path=./core \
	       --proto_path="$(PROTOC_INCLUDE)" \
	       $(PROTOC_GEN_GO_FLAG) \
	       --go_out=paths=source_relative:./core \
	       $(CONFIG_PROTO_FILES)

.PHONY: api
# generate api proto
api: check-tools
	"$(PROTOC)" --proto_path=./api \
	       --proto_path="$(KRATOS_THIRD_PARTY)" \
	       --proto_path="$(PROTOC_INCLUDE)" \
	       $(PROTOC_GEN_GO_FLAG) \
	       $(PROTOC_GEN_GO_HTTP_FLAG) \
	       --go_out=paths=source_relative:./api \
	       --go-http_out=paths=source_relative:./api \
	       $(API_PROTO_FILES)

.PHONY: openapi
# generate openapi.yaml with servers + envelope (optional)
openapi: check-openapi-tools
	@echo "generating openapi.yaml (server: $(OPENAPI_SERVER_URL))"
	"$(PROTOC)" --proto_path=./api \
	       --proto_path="$(KRATOS_THIRD_PARTY)" \
	       --proto_path="$(PROTOC_INCLUDE)" \
	       $(PROTOC_GEN_OPENAPI_FLAG) \
	       --openapi_out=fq_schema_naming=true,default_response=false:. \
	       $(API_PROTO_FILES)
	@go run ./hack/openapi -file openapi.yaml \
		-server "$(OPENAPI_SERVER_URL)" \
		-server-desc "$(OPENAPI_SERVER_DESC)"

.PHONY: ent
# generate ent code (uses the Go toolchain declared in go.mod)
ent:
	GOTOOLCHAIN=go$(GO_VERSION) go generate ./ent/...

.PHONY: wire
# generate wire dependency injection
wire:
	go generate ./cmd/...

.PHONY: generate
# generate ent/wire and tidy go.mod
generate: ent wire
	go mod tidy

.PHONY: build
# build application binaries into ./bin/ (tools under hack/ are excluded)
build:
	mkdir -p bin/ && go build -ldflags "-X main.Version=$(VERSION) -X main.Name=$(APP)" -o ./bin/ ./cmd/...

.PHONY: run
# run server locally (works both before and after kratos new renames cmd/server)
run:
	go run -ldflags "-X main.Name=$(APP) -X main.Version=$(VERSION)" $(firstword $(wildcard ./cmd/*)) -conf ./configs

.PHONY: test
# run unit, architecture, and HTTP integration tests
test:
	go test ./...

.PHONY: test-race
# run all tests with Go race detector enabled
test-race:
	go test -race ./...

.PHONY: fmt
# format handwritten Go code (generated ent code is intentionally excluded)
fmt:
	gofmt -w ./cmd ./core ./hack ./internal ./pkg ./test

.PHONY: fmt-check
fmt-check:
	@out="$$(gofmt -l ./cmd ./core ./hack ./internal ./pkg ./test)"; \
	if [ -n "$$out" ]; then echo "gofmt required:"; echo "$$out"; exit 1; fi

.PHONY: check
# run go vet, gofmt check and architecture boundary tests
check: fmt-check
	go vet ./...
	go test -v ./test/arch/...

.PHONY: all
# generate api, config, ent, wire and run check
all: api config generate check

# show help
help:
	@echo ''
	@echo 'Usage:'
	@echo ' make [target]'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-\_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m%-22s\033[0m %s\n", helpCommand,helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
