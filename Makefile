include tools/tools.mk

.PHONY: generate
generate: $(BUF) $(PROTOC_GEN_GO) 
	@ $(BUF) generate --template '$(BUF_GEN_TEMPLATE)' .

.PHONY: test
test: 
	@ go test -v -cover -coverprofile=cover.out ./...
