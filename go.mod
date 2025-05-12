module github.com/cerbos/protoc-gen-go-hashpb

go 1.24

toolchain go1.24.1

require (
	github.com/cespare/xxhash/v2 v2.3.0
	google.golang.org/protobuf v1.36.6
)

require github.com/google/go-cmp v0.7.0 // indirect

tool google.golang.org/protobuf/cmd/protoc-gen-go
