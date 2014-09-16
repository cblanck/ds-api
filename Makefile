VERSION := $(shell git rev-parse HEAD)
VERSION_DIRTY := $(shell if [ ! -z "`git status --porcelain`" ]; then echo '-dirty'; fi)

all:
	go build -ldflags "-X main.build_version ${VERSION}${VERSION_DIRTY}"
