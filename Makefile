.PHONY: build clean install

BINARY_NAME=hollow
VERSION=$(shell git describe --tags --always || echo "dev")

build:
	@echo "Compilation de Hollow..."
	go build -ldflags="-X main.Version=$(VERSION)" -o $(BINARY_NAME) ./cmd/hollow

clean:
	@echo "Nettoyage..."
	rm -f $(BINARY_NAME)
	rm -rf dist/

install: build
	@echo "Installation dans /usr/local/bin..."
	sudo cp $(BINARY_NAME) /usr/local/bin/