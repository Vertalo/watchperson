PLATFORM=$(shell uname -s | tr '[:upper:]' '[:lower:]')
VERSION := $(shell grep -Eo '(v[0-9]+[\.][0-9]+[\.][0-9]+(-[a-zA-Z0-9]*)?)' version.go)
TEST_ITERATIONS := "first" "second"

.PHONY: build clean

build: clean
	@rm -rf ./bin ./data/*.txt ./data/*.json
	@CGO_ENABLED=1 go build -o ./bin/server github.com/moov-io/watchman/cmd/server

start: build
	@./bin/server

test: export input="input.tsv"
test: export output="result.json"
test: export data_directory="./tmp/data"
test: export limit=100
test: build
	@for word in $(TEST_ITERATIONS); do \
		mkdir -p "${data_directory}" && \
		cp "./data/$(input)" "./tmp/$(input)" && \
		./bin/server --input-file="./tmp/${input}" --output-file="./data/$$word-${output}" --data-directory="${data_directory}" --limit-file-rows=$(limit) && \
		cat "./data/$$word-${output}" | jq '.[].hash' | sort -u > "./data/$$word-hashes.txt"; \
	done
	
.PHONY: clean
clean:
ifeq ($(OS),Windows_NT)
	@echo "Skipping cleanup on Windows, currently unsupported."
else
	@rm -rf ./bin
endif
