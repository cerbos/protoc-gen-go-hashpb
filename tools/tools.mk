XDG_CACHE_HOME ?= $(HOME)/.cache
TOOLS_BIN_DIR := $(abspath $(XDG_CACHE_HOME)/protoc-gen-go-hashpb/bin)
TOOLS_MOD := tools/go.mod

BUF := $(TOOLS_BIN_DIR)/buf
GORELEASER := $(TOOLS_BIN_DIR)/goreleaser
PROTOC_GEN_GO := $(TOOLS_BIN_DIR)/protoc-gen-go
PROTOC_GEN_GO_HASHPB := $(TOOLS_BIN_DIR)/protoc-gen-go-hashpb


define BUF_GEN_TEMPLATE
{\
  "version": "v1",\
  "plugins": [\
    {\
      "name": "go",\
      "opt": "paths=source_relative",\
      "out": ".",\
      "path": "$(PROTOC_GEN_GO)"\
    },\
    {\
      "name": "hashpb",\
      "opt": "paths=source_relative",\
      "out": ".",\
      "path": "$(PROTOC_GEN_GO_HASHPB)"\
    },\
  ]\
}
endef

$(TOOLS_BIN_DIR):
	@ mkdir -p $(TOOLS_BIN_DIR)

$(BUF): $(TOOLS_BIN_DIR) 
	@ GOBIN=$(TOOLS_BIN_DIR) go install -modfile=$(TOOLS_MOD) github.com/bufbuild/buf/cmd/buf

$(GORELEASER): $(TOOLS_BIN_DIR) 
	@ GOBIN=$(TOOLS_BIN_DIR) go install -modfile=$(TOOLS_MOD) github.com/goreleaser/goreleaser

$(PROTOC_GEN_GO): $(TOOLS_BIN_DIR) 
	@ GOBIN=$(TOOLS_BIN_DIR) go install -modfile=$(TOOLS_MOD) google.golang.org/protobuf/cmd/protoc-gen-go

