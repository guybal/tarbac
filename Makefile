run: generate fmt vet
	go run ./main.go

build:
	docker build -t ghcr.io/your-org/temporary-rbac-controller:latest .

deploy:
	kubectl apply -k config/default
