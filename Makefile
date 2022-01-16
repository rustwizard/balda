MAKEFILE_PATH := $(abspath $(dir $(abspath $(lastword $(MAKEFILE_LIST)))))

export GOBIN = $(MAKEFILE_PATH)/bin

BUILD_TARGET=$(MAKEFILE_PATH)/bin/balda

GREEN_COLOR   = "\033[0;32m"
DEFAULT_COLOR = "\033[m"

.PHONY: build

build:
	@echo -e $(GREEN_COLOR)[building balda to $(BUILD_TARGET)]$(DEFAULT_COLOR)
	@go generate ./... && go build -o $(BUILD_TARGET)

code-gen:
	@echo -e $(GREEN_COLOR)[generating models, client and server...]$(DEFAULT_COLOR)
	@swagger generate model -f api/swagger/http-api.yaml -m internal/server/models
