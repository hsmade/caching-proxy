all: build upload

build:
	VERSION=$(rev-parse HEAD)
	docker build -t hsmade/cachingProxy:${VERSION} .

upload: build
	docker push hsmade/cachingProxy:${VERSION}
