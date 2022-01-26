FROM scratch

LABEL "build.buf.plugins.runtime_library_versions.0.name"="google.golang.org/protobuf"
LABEL "build.buf.plugins.runtime_library_versions.0.version"="v1.27.1"
ENTRYPOINT ["/protoc-gen-go-hashpb"]
COPY protoc-gen-go-hashpb /
