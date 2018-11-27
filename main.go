package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"flag"
	"github.com/elazarl/goproxy"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"
)

var (
	port        string
	verbose     bool
	maxRretries int
	retrySleep  time.Duration
)

type cacheItem struct {
	response *http.Response
	body     *[]byte
}

func extractBody(source *http.Response, ctx *goproxy.ProxyCtx) (data *[]byte) {
	buf, err := ioutil.ReadAll(source.Body)
	if err != nil {
		ctx.Warnf("Failed to read body: %v", err)
	}
	err = source.Body.Close()
	if err != nil {
		ctx.Warnf("Failed to close body: %v", err)
	}
	data = &buf
	source.Body = ioutil.NopCloser(bytes.NewReader(buf))
	return
}

func copyResponse(r *http.Response, b *[]byte) *http.Response {
	c := *r
	c.Body = ioutil.NopCloser(bytes.NewReader(*b))
	return &c
}

func generateHash(r *http.Request) string {
	hash := md5.New()
	return hex.EncodeToString(hash.Sum([]byte(r.URL.Host + r.URL.Path + r.Method + r.Header.Get("X-Auth-Token"))))
}

func main() {
	flag.StringVar(&port, "port", "3128", "Port to listen on")
	flag.BoolVar(&verbose, "verbose", false, "Verbose logging on")
	flag.IntVar(&maxRretries, "max-retries", 30, "Max retries per call when a 500 is received")
	flag.DurationVar(&retrySleep, "retry-sleep", 1*time.Second, "Time to sleep in between retries")
	flag.Parse()

	cache := make(map[string]cacheItem, 1000)

	cacheWriteLock := sync.Mutex{}
	proxy := goproxy.NewProxyHttpServer()
	proxy.OnRequest().HandleConnect(goproxy.AlwaysMitm)

	proxy.OnRequest().DoFunc(
		func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			hash := generateHash(r)
			cacheWriteLock.Lock()
			defer cacheWriteLock.Unlock()
			item, ok := cache[hash]
			if ok {
				ctx.Warnf("Returning cached data[%v] for %v", len(cache), r.URL.String())
				response := copyResponse(item.response, item.body)
				return r, response
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
				retries++
			}
		}

		if resp.StatusCode >= 500 || ctx.Error != nil {
			ctx.Warnf("Returning errored response after retry")
			return resp // Don't cache this
		}

		cacheWriteLock.Lock()
		defer cacheWriteLock.Unlock()

		_, ok := cache[hash]
		if !ok {
			responseCopy := *resp
			responseBody := extractBody(resp, ctx)
			cache[hash] = cacheItem{&responseCopy, responseBody}
		}
		return resp
	})

	proxy.Verbose = verbose
	log.Printf("Starting proxy on :%v", port)
	log.Fatalln(http.ListenAndServe(":"+port, proxy))
}
