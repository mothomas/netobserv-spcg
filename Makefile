.PHONY: proto run-engine run-ui build docker

PROTO_DIR=api/proto/capture/v1
GEN_DIR=api/proto/capture/v1

proto:
	@command -v protoc >/dev/null || (echo "install protoc"; exit 1)
	protoc -I api/proto \
		--go_out=$(GEN_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_DIR) --go-grpc_opt=paths=source_relative \
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
