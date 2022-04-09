.PHONY: test
test:
	MallocNanoZone=0 go test -race ./...

fmt:
	go fmt ./...

lint:
	go vet ./...

