test: build-with-coverage
	@rm -fr .coverdata
	@mkdir -p .coverdata
	@go test ./...
	@go tool covdata percent -i=.coverdata

check-coverage: test
	@go tool covdata textfmt -i=.coverdata -o profile.txt
	@go tool cover -html=profile.txt

build:
	@go build

build-with-coverage:
	@go build -cover -o notecata-coverage

.DEFAULT_GOAL := build
