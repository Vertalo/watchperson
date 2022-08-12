PLATFORM=$(shell uname -s | tr '[:upper:]' '[:lower:]')
VERSION := $(shell grep -Eo '(v[0-9]+[\.][0-9]+[\.][0-9]+(-[a-zA-Z0-9]*)?)' version.go)

.PHONY: build docker release check clean

build:
	@CGO_ENABLED=1 go build -o ./bin/server github.com/moov-io/watchman/cmd/server

start: build
	@./bin/server

.PHONY: check
check:
ifeq ($(OS),Windows_NT)
	@echo "Skipping checks on Windows, currently unsupported."
else
	@wget -O lint-project.sh https://raw.githubusercontent.com/moov-io/infra/master/go/lint-project.sh
	@chmod +x ./lint-project.sh
	COVER_THRESHOLD=70.0 DISABLE_GITLEAKS=true ./lint-project.sh
endif

.PHONY: clean
clean:
ifeq ($(OS),Windows_NT)
	@echo "Skipping cleanup on Windows, currently unsupported."
else
	@rm -rf ./bin/ cover.out coverage.txt lint-project.sh misspell* staticcheck* openapi-generator-cli-*.jar ./watchman.db
endif

docker: clean docker-hub docker-openshift docker-static docker-watchmantest docker-webhook

docker-hub:
	docker build --pull -t moov/watchman:$(VERSION) -f Dockerfile .
	docker tag moov/watchman:$(VERSION) moov/watchman:latest

docker-openshift:
	docker build --pull -t quay.io/moov/watchman:$(VERSION) -f Dockerfile-openshift --build-arg VERSION=$(VERSION) .
	docker tag quay.io/moov/watchman:$(VERSION) quay.io/moov/watchman:latest

docker-static:
	docker build --pull -t moov/watchman:static -f Dockerfile-static .

docker-watchmantest:
	docker build --pull -t moov/watchmantest:$(VERSION) -f ./cmd/watchmantest/Dockerfile .
	docker tag moov/watchmantest:$(VERSION) moov/watchmantest:latest

docker-webhook:
	docker build --pull -t moov/watchman-webhook-example:$(VERSION) -f ./examples/webhook/Dockerfile .
	docker tag moov/watchman-webhook-example:$(VERSION) moov/watchman-webhook-example:latest

release: docker AUTHORS
	go vet ./...
	go test -coverprofile=cover-$(VERSION).out ./...
	git tag -f $(VERSION)

release-push:
	docker push moov/watchman:$(VERSION)
	docker push moov/watchman:latest
	docker push moov/watchman:static
	docker push moov/watchmantest:$(VERSION)
	docker push moov/watchman-webhook-example:$(VERSION)

quay-push:
	docker push quay.io/moov/watchman:$(VERSION)
	docker push quay.io/moov/watchman:latest

.PHONY: cover-test cover-web
cover-test:
	go test -coverprofile=cover.out ./...
cover-web:
	go tool cover -html=cover.out

clean-integration:
	docker-compose kill && docker-compose rm -v -f

test-integration: clean-integration
	docker-compose up -d
	sleep 30
	curl -v http://localhost:9094/data/refresh # hangs until download and parsing completes
	./bin/batchsearch -local -threshold 0.95

# From https://github.com/genuinetools/img
.PHONY: AUTHORS
AUTHORS:
	@$(file >$@,# This file lists all individuals having contributed content to the repository.)
	@$(file >>$@,# For how it is generated, see `make AUTHORS`.)
	@echo "$(shell git log --format='\n%aN <%aE>' | LC_ALL=C.UTF-8 sort -uf)" >> $@
