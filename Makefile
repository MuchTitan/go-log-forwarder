run: build
	@./bin/main.out

build:
	@go build -o bin/main.out cmd/main.go
