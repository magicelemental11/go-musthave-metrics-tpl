// Package proto holds the Go code generated from api/proto/metrics.proto.
//
// Regenerating needs protoc and the plugins pinned below:
//
//	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12
//	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2
//	go generate ./internal/proto
package proto

//go:generate protoc --proto_path=../../api/proto --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative metrics.proto
