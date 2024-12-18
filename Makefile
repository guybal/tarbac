# Paths and binaries
CONTROLLER_GEN = $(shell which controller-gen)

# Default target
all: generate

# Generate code
generate:
	$(CONTROLLER_GEN) crd paths=./api/v1 output:crd:artifacts:config=./config/crd/bases

# Tidy Go modules (optional)
tidy:
	go mod tidy

# Build the manager binary
build:
	go build -o bin/manager main.go

# Run the manager locally
run: generate
	go run ./main.go

# Test (optional, for testing controllers)
test: generate
	go test ./... -coverprofile cover.out
