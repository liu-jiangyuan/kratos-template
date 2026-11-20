GOHOSTOS:=$(shell go env GOHOSTOS)
GOPATH:=$(shell go env GOPATH)
VERSION=$(shell git describe --tags --always)

ifeq ($(GOHOSTOS), windows)
	# the `find.exe` is different from `find` in bash/shell.
	# to see https://docs.microsoft.com/en-us/windows-server/administration/windows-commands/find.
	# changed to use git-bash.exe to run find cli or other cli friendly, caused of every developer has a Git.
	Git_Bash=$(subst \,/,$(subst cmd\,bin\bash.exe,$(dir $(shell where git))))
	CONFIG_PROTO_FILES=$(shell $(Git_Bash) -c "find core/conf -name *.proto")
	API_PROTO_FILES=$(shell $(Git_Bash) -c "find api -name *.proto")
else
	CONFIG_PROTO_FILES=$(shell find core/conf -name *.proto)
	API_PROTO_FILES=$(shell find api -name *.proto)
endif

.PHONY: init
# init env (install protoc plugins, wire, ent)
init:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/go-kratos/kratos/cmd/kratos/v3@latest
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v3@latest
	go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest
	go install github.com/google/wire/cmd/wire@latest
	go install entgo.io/ent/cmd/ent@latest

.PHONY: config
# generate core/conf proto
config:
	protoc --proto_path=./core/conf \
	       --go_out=paths=source_relative:./core/conf \
	       $(CONFIG_PROTO_FILES)

.PHONY: api
# generate api proto
api:
	protoc --proto_path=./api \
	       --proto_path=./third_party \
	       --go_out=paths=source_relative:./api \
	       --go-http_out=paths=source_relative:./api \
	       --go-grpc_out=paths=source_relative:./api \
	       --openapi_out=fq_schema_naming=true,default_response=false:. \
	       $(API_PROTO_FILES)

.PHONY: ent
# generate ent code
ent:
	go generate ./ent/...

.PHONY: wire
# generate wire dependency injection
wire:
	go generate ./cmd/...

.PHONY: generate
# generate all (ent, wire) and tidy go.mod
generate:
	go generate ./...
	go mod tidy

.PHONY: build
# build binaries into ./bin/
build:
	mkdir -p bin/ && go build -ldflags "-X main.Version=$(VERSION)" -o ./bin/ ./...

.PHONY: run
# run server locally (works both before and after kratos new renames cmd/server)
run:
	go run $(firstword $(wildcard ./cmd/*)) -conf ./configs

.PHONY: test
# run all unit and architecture tests
test:
	go test ./...

.PHONY: check
# run go vet and architecture boundary check
check:
	go vet ./...
	go test -v ./test/arch/...

.PHONY: all
# generate api, config, ent, wire and run check
all:
	make api
	make config
	make generate
	make check

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
