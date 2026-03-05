.PHONY: gen-go gen-rust gen-all

gen-go:
	export PATH=$(PATH):$(shell go env GOPATH)/bin && \
	cd proto && protoc --go_out=../server/proto --go_opt=paths=source_relative --go-grpc_out=../server/proto --go-grpc_opt=paths=source_relative cloudguardian.proto

gen-rust:
	cd agent && cargo build

gen-all: gen-go gen-rust
