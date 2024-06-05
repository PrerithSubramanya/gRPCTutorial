# User Service

This repository contains the implementation of the User Service using gRPC.

## Generating Go Code from Proto Files

To generate the Go code from the `user_service.proto` file, use the following command:

```sh
protoc -I . \
    --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    --grpc-gateway_out=. --grpc-gateway_opt=paths=source_relative \
    --grpc-gateway_opt=logtostderr=true \
    user_service.proto
