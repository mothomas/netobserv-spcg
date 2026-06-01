.PHONY: proto run-engine run-ui build docker

PROTO_DIR=api/proto/capture/v1
GEN_DIR=api/proto/capture/v1

proto:
	@command -v protoc >/dev/null || (echo "install protoc"; exit 1)
	@command -v protoc-gen-go >/dev/null || go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@command -v protoc-gen-go-grpc >/dev/null || go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	protoc -I api/proto \
		--go_out=. --go_opt=module=github.com/netobserv/spcg \
		--go-grpc_out=. --go-grpc_opt=module=github.com/netobserv/spcg \
		$(PROTO_DIR)/capture.proto

build:
	go build -o bin/backend-engine ./cmd/backend-engine
	go build -o bin/ui-portal ./cmd/ui-portal

run-engine:
	go run ./cmd/backend-engine

run-ui:
	go run ./cmd/ui-portal

docker:
	docker build -f deploy/Dockerfile.engine -t spcg-backend-engine:latest .
	docker build -f deploy/Dockerfile.ui -t spcg-ui-portal:latest .
	docker build -f deploy/Dockerfile.frontend -t spcg-frontend:latest .
