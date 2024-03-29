MAKEFILE_PATH := $(abspath $(dir $(abspath $(lastword $(MAKEFILE_LIST)))))

export GOBIN = $(MAKEFILE_PATH)/bin

BUILD_TARGET=$(MAKEFILE_PATH)/bin/balda

GREEN_COLOR   = "\033[0;32m"
DEFAULT_COLOR = "\033[m"

.PHONY: build code-gen test

build:
	@echo -e $(GREEN_COLOR)[building balda to $(BUILD_TARGET)]$(DEFAULT_COLOR)
	@go build -o $(BUILD_TARGET)

code-gen:
	@echo -e $(GREEN_COLOR)[generating models and server...]$(DEFAULT_COLOR)
	@swagger generate server -f api/swagger/http-api.yaml -s internal/server/restapi --exclude-main -m internal/server/models

test:
	@echo -e $(GREEN_COLOR)[running tests]$(DEFAULT_COLOR)
	@go generate ./... && go test -v `go list ./... | grep -v integration`

docker:
	@docker build -f ./build/Dockerfile ./

