SHELL := /bin/bash
GOPATH := /home/${USER}/git/go
export GOPATH

# The name of the executable (default is current directory name)
TARGET := $(shell echo $${PWD\#\#*/})
.DEFAULT_GOAL: $(TARGET)

# These will be provided to the target
VERSIONNAME := HMS-St-Andrew-(1670)
VERSION := 0.4
BUILD := `git rev-parse HEAD`
BUILDTIME := `date +'%y.%m.%d/%H:%M:%S'`

# Use linker flags to provide version/build settings to the target
LDFLAGS=-ldflags "-X=main.Version=$(VERSION) -X=main.Versionname=$(VERSIONNAME) -X=main.Build=$(BUILD) -X=main.Buildtime=$(BUILDTIME)"

# go source files, ignore vendor directory
SRC = $(shell find . -type f -name '*.go' -not -path "./vendor/*")

.PHONY: build

all: build

$(TARGET): $(SRC)
	@go build $(LDFLAGS) -o $(TARGET)

build: $(TARGET)
	@true
