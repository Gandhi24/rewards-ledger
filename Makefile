.PHONY: run build test test-cover fmt vet tidy docker-up docker-down clean

# Run the server locally (defaults to :8080, override with PORT=xxxx).
run:
	go run ./cmd/server

# Compile the binary into ./bin.
build:
	go build -o bin/server ./cmd/server

# Run the full test suite (race detector on).
test:
	go test -race ./...

# Run tests with a coverage summary.
test-cover:
	go test -race -cover ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

tidy:
	go mod tidy

# Build and start via docker-compose.
docker-up:
	docker compose up --build

docker-down:
	docker compose down

clean:
	rm -rf bin
