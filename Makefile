.PHONY: build test race vet check mock serve generate conformance bench-e2e
conformance:
	go run ./tools/conformance
build:
	go run ./tools/build
test:
	go test ./...
race:
	go test -race ./...
vet:
	go vet ./...
check: test race vet
mock:
	go run ./cmd/mock-upstream
serve:
	go run ./cmd/moduapi serve --config config.yaml
generate:
	protoc -I proto --go_out=. --go_opt=module=github.com/huey1in/moduapi --go-grpc_out=. --go-grpc_opt=module=github.com/huey1in/moduapi proto/moduapi/plugin/v1/plugin.proto
bench-e2e:
	go run ./tools/bench-e2e
