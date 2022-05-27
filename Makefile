gomod:
	go mod tidy
	go mod vendor

build-echo:
	go build -o echo ./examples/echo.go

run-echo: build-echo
	./echo

test:
	go test ./...