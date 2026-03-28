.PHONY: run connect test check clean

# Start server locally (builds first)
run:
	go build -o bin/tavrn-admin ./cmd/tavrn-admin
	./bin/tavrn-admin

# Connect to local server with audio (builds first)
connect:
	go build -o bin/tavrn ./cmd/tavrn
	./bin/tavrn --dev

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
