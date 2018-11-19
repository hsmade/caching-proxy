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
	"crypto/md5"
	"encoding/hex"
	"time"
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
	maxRretries int
)

func generateHash(r *http.Request) string {
	hash := md5.New()
	return hex.EncodeToString(hash.Sum([]byte(r.URL.Host + r.URL.Path + r.Method + r.Header.Get("X-Auth-Token"))))
}

func main() {
	flag.StringVar(&port, "port", "3128", "Port to listen on")
	flag.BoolVar(&verbose, "verbose", false, "Verbose logging on")
	flag.IntVar(&maxRretries, "max-retries", 30, "Max retries per call when a 500 is received")
	flag.Parse()

	cache := make(map[string]*http.Response, 1000)

	cacheWriteLock := sync.Mutex{}
	proxy := goproxy.NewProxyHttpServer()
	proxy.OnRequest().HandleConnect(goproxy.AlwaysMitm)

	proxy.OnRequest().DoFunc(
		func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			hash := generateHash(r)
			resp, ok := cache[hash]
			if ok {
				ctx.Warnf("Returning cached data[%v] for %v", len(cache), r.URL.String())
				cacheWriteLock.Lock()
				responseCopy := *resp
				responseCopy.Body, resp.Body = copyBody(resp.Body, ctx)
				cacheWriteLock.Unlock()
				return r, &responseCopy
			}
			return r, nil
		})

	proxy.OnResponse().DoFunc(func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		hash := generateHash(ctx.Req)
		if resp.StatusCode >= 500 {
			retries := 1
			for {
				ctx.Warnf("%v gave 500, RETRY: %v", ctx.Req.URL.String(), retries)
				time.Sleep(time.Second * 1)
				resp, _ = http.DefaultClient.Do(ctx.Req) // retry
				if resp.StatusCode != 500 {
					break
				}
				retries ++
			}
		}

		if resp.StatusCode >= 500 || ctx.Error != nil {
			ctx.Warnf("Returning errored response after retry")
			return resp // Don't cache this
		}

		cacheWriteLock.Lock()
		_, ok := cache[hash]
		if !ok {
			responseCopy := *resp
			responseCopy.Body, resp.Body = copyBody(resp.Body, ctx)
			cache[hash] = &responseCopy
		}
		cacheWriteLock.Unlock()
		return resp
	})

	proxy.Verbose = verbose
	log.Printf("Starting proxy on :%v", port)
	log.Fatalln(http.ListenAndServe(":"+port, proxy))
}
