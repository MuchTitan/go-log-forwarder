run: build
	@./bin/forwarder-client.out

build: 
	@go build -o bin/forwarder-client.out

dbUp:
	@migrate -database sqlite3://./your_database.db -path ./migrations up

dbDown:
	@migrate -database sqlite3://./your_database.db -path ./migrations down

dbVersion:
	@migrate -database sqlite3://./your_database.db -path ./migrations version
