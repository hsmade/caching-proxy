VERSION := $(shell git rev-parse HEAD)

all: build upload

build:
	docker build -t hsmade/caching-proxy:$(VERSION) .

upload: build
	docker push hsmade/caching-proxy:$(VERSION)
