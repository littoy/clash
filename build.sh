CGO_ENABLED=0 go build -trimpath  -ldflags '-w -s -X google.golang.org/protobuf/reflect/protoregistry.conflictPolicy=warn'