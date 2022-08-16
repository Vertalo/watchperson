PLATFORM=$(shell uname -s | tr '[:upper:]' '[:lower:]')
VERSION := $(shell grep -Eo '(v[0-9]+[\.][0-9]+[\.][0-9]+(-[a-zA-Z0-9]*)?)' version.go)

.PHONY: build clean

build: clean
	@go mod download
	@CGO_ENABLED=1 go build -o ./bin/server github.com/moov-io/watchman/cmd/server

start: build
	@./bin/server

export input=input.tsv
export output=result.json
export limit=100
test: build
	@for i in first second; do mkdir -p ./tmp && cp ./data/$(input) ./tmp/$(input) && ./bin/server --input-file=./tmp/$(input) --limit-file-rows=$(limit) --output-file=./data/$(i)-$(output) && cat ./data/$(i)-$(output) | jq '.[].hash' | sort -u > ./data/$(i)-hashes.txt; done
	@diff ./data/first-hashes.txt ./data/second-hashes.txt
	
.PHONY: clean
clean:
ifeq ($(OS),Windows_NT)
	@echo "Skipping cleanup on Windows, currently unsupported."
else
	@rm -rf ./bin
endif
