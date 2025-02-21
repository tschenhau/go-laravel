## test: runs all tests
test:
	@go test -v ./...

## cover: opens coverage in browser
cover:
	@go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out

## coverage: displays test coverage
coverage:
	@go test -cover ./...

## build_cli: builds the command line tool celeritas and copies it to myapp
build_cli:
	@go build -o ../myapp/celeritas ./cmd/cli

## install_cli: builds the command line tool celeritas and installs it to ~/go/bin
install_cli:
	@go build -o ~/go/bin/celeritas -ldflags '-s -w' ./cmd/cli

build:
	@go build -o ./dist/celeritas ./cmd/cli

## help: displays help
help: Makefile
	@echo " Choose a command:"
	@sed -n 's/^##//p' $< | column -t -s ':' |  sed -e 's/^/ /'