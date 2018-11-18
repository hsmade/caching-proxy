package main

import (
	"bytes"
	"flag"
	"github.com/elazarl/goproxy"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
)

func copyBody(source io.ReadCloser, ctx *goproxy.ProxyCtx) (dest1 io.ReadCloser, dest2 io.ReadCloser) {
	buf, err := ioutil.ReadAll(source)
	if err != nil {
		ctx.Warnf("Failed to read body: %v", err)
	}
	dest1 = ioutil.NopCloser(bytes.NewBuffer(buf))
	dest2 = ioutil.NopCloser(bytes.NewBuffer(buf))
	return
}

var (
	port    string
	verbose bool
)

func main() {
	flag.StringVar(&port, "port", "3128", "Port to listen on")
	flag.BoolVar(&verbose, "verbose", false, "Verbose logging on")
	flag.Parse()

	cache := make(map[string]*http.Response, 1000)
	cacheWriteLock := sync.Mutex{}
	proxy := goproxy.NewProxyHttpServer()
	proxy.OnRequest().HandleConnect(goproxy.AlwaysMitm)

	proxy.OnRequest().DoFunc(
		func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			ctx.Logf("Handling request for %v", r.URL.String())
			resp, ok := cache[r.URL.String()]
			if ok {
				ctx.Warnf("Returning cached data[%v] for %v", len(cache), r.URL.String())
				responseCopy := *resp
				cacheWriteLock.Lock()
				responseCopy.Body, resp.Body = copyBody(resp.Body, ctx)
				cacheWriteLock.Unlock()
				return r, &responseCopy
			}
			return r, nil
		})

	proxy.OnResponse().DoFunc(func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		if ctx.Error != nil {
			ctx.Warnf("RETRYING because of %v", ctx.Error)
			resp, _ = http.DefaultClient.Do(ctx.Req) // retry
		} else if resp.StatusCode >= 500 {
			ctx.Warnf("RETRYING because of status code: %v", resp.StatusCode)
			resp, _ = http.DefaultClient.Do(ctx.Req) // retry
		}

		if resp.StatusCode >= 500 || ctx.Error != nil {
			ctx.Warnf("Returning errored response after retry")
			return resp // Don't cache this
		}
		_, ok := cache[ctx.Req.URL.String()]
		if !ok {
			ctx.Warnf("Storing response in cache[%v] for %v", len(cache), ctx.Req.URL.String())
			responseCopy := *resp
			cacheWriteLock.Lock()
			responseCopy.Body, resp.Body = copyBody(resp.Body, ctx)
			cache[ctx.Req.URL.String()] = &responseCopy
			cacheWriteLock.Unlock()
		}
		return resp
	})

	proxy.Verbose = verbose
	log.Printf("Starting proxy on :%v", port)
	log.Fatalln(http.ListenAndServe(":"+port, proxy))
}
