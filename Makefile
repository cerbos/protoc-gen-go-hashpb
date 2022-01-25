include tools/tools.mk

.PHONY: protoc-gen-go-hashpb
protoc-gen-go-hashpb: 
	@ go build -o $(PROTOC_GEN_GO_HASHPB) .

.PHONY: generate
generate: $(BUF) $(PROTOC_GEN_GO) protoc-gen-go-hashpb
	@ $(BUF) generate --template '$(BUF_GEN_TEMPLATE)' .

.PHONY: test
test: generate 
	@ go test -v -count=1 ./...

.PHONY: benchmark
benchmark: generate
	@ go test -v -run=ignore -count=10 -bench=. ./...

.PHONY: build
build: $(GORELEASER) test
	@ $(GORELEASER) release --config=.goreleaser.yml --snapshot --skip-publish --rm-dist
