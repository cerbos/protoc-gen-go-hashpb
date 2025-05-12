.PHONY: protoc-gen-go-hashpb
protoc-gen-go-hashpb:
	@ go build -o protoc-gen-go-hashpb -ldflags '-X github.com/cerbos/protoc-gen-go-hashpb/internal/generator.Version=(devel)' .

.PHONY: generate
generate:
	@ go run github.com/bufbuild/buf/cmd/buf@latest generate .

.PHONY: test
test: generate
	@ go test -v -count=1 ./...

.PHONY: benchmark
benchmark: generate
	@ go test -v -run=ignore -count=10 -bench=. ./...

.PHONY: build
build: test
	@ go run github.com/goreleaser/goreleaser@latest release --config=.goreleaser.yml --snapshot --skip=publish --clean
