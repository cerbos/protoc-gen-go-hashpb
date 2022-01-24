# protoc-gen-go-hashpb

A protobuf plugin to generate hash functions for messages.

Hashing messages encoded using Protocol Buffers is tricky because there is [no guarantee that the serialized form is stable](https://developers.google.com/protocol-buffers/docs/encoding) between different implementations, architectures, or even library versions. 
This plugin generates a hash function that does a depth-first traversal of the populated values (including default values) of the message in field number order and feeds it to the provided `hash.Hash` implementation. Map values are accessed in key order as well. Because of this deterministic traversal order, the hash generated for two identical protobuf messages should be the same.

NOTE: While we have tested this plugin quite extensively, some edge cases may remain. Use at your own risk.

## Install

```shell
go install github.com/cerbos/protoc-gen-go-hashpb@latest
```

## Usage

### Generate code

With `protoc`:

```shell
protoc --plugin protoc-gen-go-hashpb=${GOBIN}/protoc-gen-go-hashpb --go_out=. --go-hashpb_out=. *.proto 
```

With [`buf`](https://github.com/bufbuild/buf): 

```shell
buf generate --template='{"version":"v1","plugins":[{"name":"go","out":"."},{"name":"go-hashpb","out":"."}]}'
```

### Calculate hashes using generated code

```go
func hashMyProto(m *mypb.MyMsg) uint64 {
    digest := xxhash.New() // any hash.Hash implementation would work
    m.HashPB(digest, nil)
    return digest.Sum64()
}
```

You can exclude certain fields from being included in the hash. The field name must be fully qualified.

```go
ignore := map[string]struct{}{"fully.qualified.package.Message.field_name1": {}, "fully.qualified.package.Message.field_name2":{}}
digest := xxhash.New() // any hash.Hash implementation would work
m.HashPB(digest, ignore)
```
