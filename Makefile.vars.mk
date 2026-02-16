## These are some common variables for Make

PROJECT_ROOT_DIR = .
PROJECT_NAME ?= gandalf
PROJECT_OWNER ?= appuio

## BUILD:go
BIN_FILENAME ?= $(PROJECT_NAME)

## BUILD:docker
DOCKER_CMD ?= docker

IMG_TAG ?= latest
# Image URL to use all building/pushing image targets
CONTAINER_IMG ?= local.dev/$(PROJECT_OWNER)/$(PROJECT_NAME):$(IMG_TAG)
