# Paths and binaries
CONTROLLER_GEN = $(shell which controller-gen)
GOIMPORTS = $(shell which goimports)
GOLANGCI_LINT = $(shell which golangci-lint)

# Default target
all: generate tidy build run

# Generate code
generate:
	$(CONTROLLER_GEN) crd paths=./api/v1 output:crd:artifacts:config=./config/crd/bases
	$(GOIMPORTS) -w .

# Tidy Go modules
tidy:
	go mod tidy

# Build the manager binary
build:
	go build -o bin/manager main.go

# Run the manager locally
run: 
	go run ./main.go

# Lint the code
lint:
	$(GOLANGCI_LINT) run

# Test (optional, for testing controllers)
# test: generate
# 	go test ./... -coverprofile cover.out
