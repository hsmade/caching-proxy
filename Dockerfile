FROM golang:1.11 AS build
RUN mkdir -p /go/src/github.com/hsmade/cachingProxy
COPY . /go/src/github.com/hsmade/cachingProxy
WORKDIR /go/src/github.com/hsmade/cachingProxy
RUN go get -u github.com/golang/dep/cmd/dep
RUN dep ensure
RUN go test ./...
RUN go build
FROM scratch
COPY --from=build /go/src/github.com/hsmade/cachingProxy/cachingProxy /
CMD ["/cachingProxy"]