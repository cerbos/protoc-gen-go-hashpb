# hashpb

Library for hashing protobuf messages.

Hashing messages encoded using Protocol Buffers is tricky because there is [no guarantee that the serialized form is stable](https://developers.google.com/protocol-buffers/docs/encoding) between different implementations, architectures, or even library versions. 
This library attempts to get around this by implementing a serializer that encodes a protobuf message using a canonical traversal order. The encoded values are fed directly into a hash function to produce a single hash value at the end. 

NOTE: This library is still under development and does not provide any guarantees about cross-arch or cross-implementation stability. Use at your own risk. 

## Usage

Calculate a 64-bit hash using xxhash (the default)

```go
h, err := hashpb.Sum64(myProtoMsg)
```

Calculate the SHA256

```go
digest, err := hashpb.Sum(nil, myProtoMsg, hashpb.WithHasher(sha256.New()))
```

Ignore some fields when calculating the hash

```go
h, err := hashpb.Sum64(myProtoMsg, hashpb.WithIgnoreFields("fully.qualified.Message.field_name1", "fully.qualified.Message.field_name2"))
```
