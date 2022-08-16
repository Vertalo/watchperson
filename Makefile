PLATFORM=$(shell uname -s | tr '[:upper:]' '[:lower:]')
VERSION := $(shell grep -Eo '(v[0-9]+[\.][0-9]+[\.][0-9]+(-[a-zA-Z0-9]*)?)' version.go)

.PHONY: build clean

build: clean
	@go build -o ./bin/server github.com/moov-io/watchman/cmd/server

start: build
	@./bin/server

export input=sample.tsv
export output=sample.json
export limit=100
test: build
	@mkdir -p ./tmp && cp ./data/$(input) ./tmp/$(input)
	@./bin/server --input-file=./tmp/$(input) --limit-file-rows=$(limit) --output-file=./data/$(output)
	@cat ./data/$(output) | jq '.[].hash' | sort -u | wc -l
	@cat ./data/$(output) | jq '.[].hash' | sort -u | wc -l

.PHONY: clean
clean:
ifeq ($(OS),Windows_NT)
	@echo "Skipping cleanup on Windows, currently unsupported."
else
	@rm -rf ./bin
endif
