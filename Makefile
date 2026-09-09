BINARY_NAME=dockeretior
BUILD_DIR=bin
INSTALL_PATH=/usr/local/bin

.PHONY: all build clean install run tidy test

all: build

tidy:
	go mod tidy

build: tidy
	@mkdir -p $(BUILD_DIR)
	@echo "==> Compilando $(BINARY_NAME)..."
	go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/dockeretior

install: build
	@echo "==> Instalando $(BINARY_NAME) en $(INSTALL_PATH)..."
	sudo install -m 755 $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)

run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

test:
	go test -v ./...

clean:
	@echo "==> Limpiando binarios y artefactos..."
	rm -rf $(BUILD_DIR)
