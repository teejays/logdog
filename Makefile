.PHONY: run-local build-local build-docker run-docker

NAME=logdog
BINARY=$(NAME).bin
CONFIG_FILE=config.toml

# Local
run-local: build-local
	./$(BINARY) --config-file=$(CONFIG_FILE)

build-local: 
	go build -o $(BINARY)

setup-local:
	go mod download

# Docker
run-docker: build-docker
	docker run $(NAME)

build-docker:
	docker build -t $(NAME) .


