.PHONY: run test check clean

# Build both binaries, start server, connect with audio
run:
	@go build -o bin/tavrn-admin ./cmd/tavrn-admin
	@go build -o bin/tavrn ./cmd/tavrn
	@./bin/tavrn-admin & sleep 1 && ./bin/tavrn --dev; kill %1 2>/dev/null

# Run all tests with race detector
test:
	go test -race ./internal/... ./ui/...

# Run before push — lint + build + test (mirrors CI)
check:
	gofmt -w .
	go vet ./...
	go build -o bin/tavrn-admin ./cmd/tavrn-admin
	go build -o bin/tavrn ./cmd/tavrn
	go test -race ./internal/... ./ui/...
	@echo "All good."

# Remove binaries and db
clean:
	rm -rf bin/ tavrn.db
