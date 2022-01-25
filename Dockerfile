FROM scratch
ENTRYPOINT ["/protoc-gen-go-hashpb"]
COPY protoc-gen-go-hashpb /
