.DEFAULT: build

build:
	go build main.go

test:
	go test ./...
