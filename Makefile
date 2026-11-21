GOOS    := linux
GOARCH  := arm64
BUILD   := build
WEBHOOK := $(BUILD)/webhook

.PHONY: all clean build webhook zips run-local invoke-local test lint fmt vet

test:
	go test -race -shuffle=on -count=1 ./...

lint:
	golangci-lint run ./...

fmt:
	golangci-lint fmt ./...

vet:
	go vet ./...

LOCAL      := $(BUILD)/local
LOCAL_PORT ?= 9000
CMD        ?= webhook
EVENT      ?= events/appeeky-alert.json
RIE_IMAGE  := public.ecr.aws/lambda/provided:al2023
LOCAL_ARCH := $(shell uname -m)
ifeq ($(LOCAL_ARCH),x86_64)
  LOCAL_GOARCH := amd64
else
  LOCAL_GOARCH := arm64
endif

COMMON_SRCS  := $(shell find ./internal -name '*.go') go.mod go.sum
WEBHOOK_SRCS := $(shell find ./cmd/webhook -name '*.go') $(COMMON_SRCS)
all: zips

zips: $(BUILD)/webhook.zip

$(BUILD)/webhook.zip: $(WEBHOOK)/bootstrap
	cd $(WEBHOOK) && zip -q ../webhook.zip bootstrap

$(WEBHOOK)/bootstrap: $(WEBHOOK_SRCS)
	mkdir -p $(WEBHOOK)
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags="-s -w" -o $(WEBHOOK)/bootstrap ./cmd/webhook

build: $(WEBHOOK)/bootstrap
webhook: $(WEBHOOK)/bootstrap

$(LOCAL)/$(CMD)/bootstrap: $(shell find ./cmd/$(CMD) -name '*.go') $(COMMON_SRCS)
	mkdir -p $(LOCAL)/$(CMD)
	GOOS=linux GOARCH=$(LOCAL_GOARCH) go build -o $(LOCAL)/$(CMD)/bootstrap ./cmd/$(CMD)

run-local: $(LOCAL)/$(CMD)/bootstrap
	docker run --rm -p $(LOCAL_PORT):8080 \
		$(if $(wildcard .env.local),--env-file .env.local,) \
		-v "$(CURDIR)/$(LOCAL)/$(CMD)":/var/task \
		--entrypoint /usr/local/bin/aws-lambda-rie \
		$(RIE_IMAGE) /var/task/bootstrap

invoke-local:
	curl -s http://localhost:$(LOCAL_PORT)/2015-03-31/functions/function/invocations \
		-d @$(EVENT) | (jq . 2>/dev/null || cat)

clean:
	rm -rf $(BUILD)
