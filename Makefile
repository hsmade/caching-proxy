VERSION := $(shell git rev-parse HEAD)

all: build

build:
	docker build -t hsmade/caching-proxy:$(VERSION) .

upload: build
	@echo "Pushing to hsmade/caching-proxy:$(VERSION)"
	docker push hsmade/caching-proxy:$(VERSION)
	docker tag hsmade/caching-proxy:$(VERSION) hsmade/caching-proxy:latest
	@echo "Pushing to hsmade/caching-proxy:latest"
	docker push hsmade/caching-proxy:latest
