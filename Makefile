.PHONY: all build-all install clean

# Configuration
SYSTEMD_DIR := $(HOME)/.config/systemd/user

all: build-all

install-deps:
	git submodule update --init --recursive
	go mod download

build-all:
	@echo "Building binaries..."
	go build -o bin/wh-engine cmd/wh-engine/main.go
	cd modules/godbledger && go build -o ../../bin/godbledger .
	cd modules/godbledger-web && go build -o ../../bin/godbledger-web .

install:
	@echo "Installing binaries to /usr/local/bin..."
	sudo cp bin/* /usr/local/bin/
	
	@echo "Installing systemd units to $(SYSTEMD_DIR)..."
	mkdir -p $(SYSTEMD_DIR)
	cp deploy/*.service $(SYSTEMD_DIR)/
	cp deploy/*.timer $(SYSTEMD_DIR)/
	
	@echo "Reloading systemd daemon..."
	systemctl --user daemon-reload

clean:
	rm -rf bin/*
