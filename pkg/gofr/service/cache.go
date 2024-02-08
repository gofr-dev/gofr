package service

import (
	"context"
	"encoding/json"
	"fmt"
	cache2 "gofr.dev/pkg/gofr/cache"
	"net/http"
	"strings"
	"time"
)

type cache struct {
	CacheProvider cache2.Provider
	TTL           time.Duration

	HTTP
}

type CacheConfig struct {
	TTL           time.Duration
	CacheProvider cache2.Provider
}

func newCache(config CacheConfig, h HTTP) *cache {
	c := &cache{
		CacheProvider: config.CacheProvider,
		TTL:           config.TTL,
		HTTP:          h,
	}

	return c
}

func (c *CacheConfig) addOption(h HTTP) HTTP {
	return newCache(*c, h)
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

	// get the cacheResponse stored in the cacher
	val, err := c.CacheProvider.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(val), resp)

	if resp == nil {
		resp, err = c.HTTP.GetWithHeaders(ctx, path, queryParams, headers)
	} else {
		return resp, nil
	}

	// checking for any error while calling http service
	if err != nil {
		return nil, err
	}

	jsonResponse, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}

	c.CacheProvider.Set(ctx, key, string(jsonResponse))

	return resp, nil
}

func (c *cache) Get(ctx context.Context, path string, queryParams map[string]interface{}) (*http.Response, error) {
	return c.GetWithHeaders(ctx, path, queryParams, nil)
}
