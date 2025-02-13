run: build
	@./bin/main.out --cfg "./cfg/cfg.yaml"

build:
	@go build -o bin/main.out cmd/main.go
