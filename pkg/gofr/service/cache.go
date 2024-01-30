package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type cacheEntry struct {
	resp    *http.Response
	setTime int64
}

type cacheMap struct {
	entry map[string]cacheEntry
	m     sync.Mutex
}
type cache struct {
	cacher *cacheMap
	TTL    time.Duration

	HTTP
}

type CacheConfig struct {
	TTL time.Duration
}

func newCache(config CacheConfig, h HTTP) *cache {
	c := &cache{
		cacher: &cacheMap{entry: make(map[string]cacheEntry)},
		TTL:    config.TTL,
		HTTP:   h,
	}

	go func() {
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()

		for t := range ticker.C {
			c.cacher.m.Lock()
			for k, v := range c.cacher.entry {
				if t.Unix()-v.setTime > int64(c.TTL.Seconds()) {
					delete(c.cacher.entry, k)
				}
			}
			c.cacher.m.Unlock()
		}
	}()

	return c
}

func (c *CacheConfig) apply(h HTTP) HTTP {
	return newCache(*c, h)
}

func (c *cache) get(key string) *http.Response {
	c.cacher.m.Lock()
	v, ok := c.cacher.entry[key]
	c.cacher.m.Unlock()

	if !ok {
		return nil
	}

	return v.resp
}

func (c *cache) set(key string, value *http.Response) {
	c.cacher.m.Lock()
	c.cacher.entry[key] = cacheEntry{
		resp:    value,
		setTime: time.Now().Unix(),
	}
	c.cacher.m.Unlock()
}

func (c *cache) GetWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	headers map[string]string) (*http.Response, error) {
	var (
		resp *http.Response
		err  error
	)

	var keyBuilder strings.Builder

	keyBuilder.WriteString(path)

	for _, param := range queryParams {
		keyBuilder.WriteString(fmt.Sprintf("%v_", param))
	}

	for _, header := range headers {
		keyBuilder.WriteString(fmt.Sprintf("%v_", header))
	}

	key := keyBuilder.String()

	// TODO - make this key fix sized // example - hashing

	// get the response stored in the cacher
	resp = c.get(key)

	if resp == nil {
		resp, err = c.HTTP.GetWithHeaders(ctx, path, queryParams, headers)
	} else {
		return resp, nil
	}

	// checking for any error while calling http service
	if err != nil {
		return nil, err
	}

	c.set(key, resp)

	return resp, nil
}

func (c *cache) Get(ctx context.Context, path string, queryParams map[string]interface{}) (*http.Response, error) {
	return c.GetWithHeaders(ctx, path, queryParams, nil)
}

func (c *cache) Post(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte) (*http.Response, error) {
	return c.PostWithHeaders(ctx, path, queryParams, body, nil)
}

func (c *cache) PostWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte, headers map[string]string) (*http.Response, error) {
	return c.HTTP.PostWithHeaders(ctx, path, queryParams, body, headers)
}

func (c *cache) Patch(ctx context.Context, path string, queryParams map[string]interface{}, body []byte) (*http.Response, error) {
	return c.PatchWithHeaders(ctx, path, queryParams, body, nil)
}

func (c *cache) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte, headers map[string]string) (*http.Response, error) {
	return c.HTTP.PatchWithHeaders(ctx, path, queryParams, body, headers)
}

func (c *cache) Put(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte) (*http.Response, error) {
	return c.PutWithHeaders(ctx, path, queryParams, body, nil)
}

func (c *cache) PutWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte, headers map[string]string) (*http.Response, error) {
	return c.HTTP.PutWithHeaders(ctx, path, queryParams, body, headers)
}

func (c *cache) Delete(ctx context.Context, path string, body []byte) (*http.Response, error) {
	return c.DeleteWithHeaders(ctx, path, body, nil)
}

func (c *cache) DeleteWithHeaders(ctx context.Context, path string, body []byte, headers map[string]string) (*http.Response, error) {
	return c.HTTP.DeleteWithHeaders(ctx, path, body, headers)
}
