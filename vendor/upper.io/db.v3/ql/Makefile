SHELL := bash

DB_NAME ?= ql-test.db

export DB_NAME

build:
	go build && go install

require-client:
	@if [ -z "$$(which ql)" ]; then \
		echo 'Missing "ql" command. Please install the QL cli tool and try again.' && \
		exit 1; \
	fi

generate:
	go generate && \
	go get -d -t -v ./...

reset-db: require-client
	rm -f $(DB_NAME)

test: reset-db generate
	#go test -tags generated -v -race # race: limit on 8192 simultaneously alive goroutines is exceeded, dying
	go test -tags generated -timeout 30m -v
