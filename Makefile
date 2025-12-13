APP := tk
PKG := ./cmd/$(APP)
GOBIN ?= $(HOME)/.local/bin

.PHONY: build run test fmt vet install clean

build:
	go build $(PKG)

run: build
	./$(APP) --help

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

install:
	GOBIN=$(GOBIN) go install $(PKG)

clean:
	rm -f $(APP)
