package main

import (
	"github.com/elazarl/goproxy"
	"log"
	"net/http"
	"io/ioutil"
	"bytes"
	"io"
	"sync"
	"flag"
)

func copyBody(source io.ReadCloser) (dest1 io.ReadCloser, dest2 io.ReadCloser) {
	buf, _ := ioutil.ReadAll(source)
	dest1 = ioutil.NopCloser(bytes.NewBuffer(buf))
	dest2 = ioutil.NopCloser(bytes.NewBuffer(buf))
	return
}

var port string


func main() {
	flag.StringVar(&port, "port", "3128", "Port to listen on")
	flag.Parse()

	cache := make(map[string]*http.Response, 1000)
	cacheWriteLock := sync.Mutex{}
	proxy := goproxy.NewProxyHttpServer()
	proxy.OnRequest().HandleConnect(goproxy.AlwaysMitm)

	proxy.OnRequest().DoFunc(
		func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			resp, ok := cache[r.URL.String()]; if ok {
				ctx.Warnf("Returning cached data[%v] for %v", len(cache), r.URL.String())
				responseCopy := *resp
				responseCopy.Body, resp.Body = copyBody(resp.Body)
				return r, &responseCopy
			}
			return r, nil
		})

	proxy.OnResponse().DoFunc(func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		if resp.StatusCode >= 500 {
			resp, _ = http.DefaultClient.Do(ctx.Req)  // retry
		}
		if resp.StatusCode >= 500 {
			return resp  // Don't cache this
		}
		_, ok := cache[ctx.Req.URL.String()]; if !ok {
			ctx.Warnf("Storing response in cache[%v] for %v", len(cache), ctx.Req.URL.String())
			responseCopy := *resp
			responseCopy.Body, resp.Body = copyBody(resp.Body)
			cacheWriteLock.Lock()
			cache[ctx.Req.URL.String()] = &responseCopy
			cacheWriteLock.Unlock()
		}
		return resp
	})

	//proxy.Verbose = true
	log.Fatalln(http.ListenAndServe(":" + port, proxy))
}
