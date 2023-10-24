package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"gofr.dev/pkg/middleware"
)

type cachedHTTPService struct {
	*httpService

	cacher       Cacher
	ttl          time.Duration
	keyGenerator KeyGenerator
}

// Get performs HTTP GET requests to an API, optionally caching responses.
// It calculates cache keys, checks for cached data, and if absent, fetches and caches
// the response, enhancing API performance and minimizing redundant requests.
func (c *cachedHTTPService) Get(ctx context.Context, api string, params map[string]interface{}) (*Response, error) {
	return c.GetWithHeaders(ctx, api, params, nil)
}

// GetWithHeaders performs HTTP GET requests to an API, optionally caching responses.
// It calculates cache keys, checks for cached data, and if absent, fetches and caches
// the response, enhancing API performance and minimizing redundant requests.
func (c *cachedHTTPService) GetWithHeaders(ctx context.Context, api string, params map[string]interface{},
	headers map[string]string) (*Response, error) {
	cacheKey := ""
	// if The keyGenerator function is passed by the user
	if c.keyGenerator != nil {
		cacheKey = c.keyGenerator(c.url+"/"+api, params, headers)
	} else {
		headers = c.getHeaders(ctx, headers)
		cacheKey = generateKey(c.url+"/"+api, params, headers)
	}

	cacheKeyStatus := cacheKey + "_status"

	body, _ := c.cacher.Get(cacheKey)
	code, _ := c.cacher.Get(cacheKeyStatus)

	statusCode, err := strconv.Atoi(string(code))

	if body != nil && err == nil {
		c.logger.Debug("getting cached response")

		return &Response{
			Body:       body,
			StatusCode: statusCode,
		}, nil
	}

	resp, err := c.httpService.call(ctx, "GET", api, params, nil, headers)
	if err != nil {
		return nil, err
	}

	if c.ttl == 0 {
		c.ttl = time.Minute * RetryFrequency
	}

	err = c.cacher.Set(cacheKey, resp.Body, c.ttl)
	if err != nil {
		c.logger.Errorf("unable to cache, err:%v", err)
	}

	err = c.cacher.Set(cacheKeyStatus, []byte(strconv.Itoa(resp.StatusCode)), c.ttl)
	if err != nil {
		c.logger.Errorf("unable to cache status code, err:%v", err)
	}

	return resp, nil
}

// generateKey generates a key based on api and params
func generateKey(api string, params map[string]interface{}, headers map[string]string) string {
	if len(params) == 0 && len(headers) == 0 {
		return api
	}
	// sort the param based on keys to ensure that key generated does not change based on order of params in the request
	keys := make([]string, 0)
	for k := range params {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	sortedParams := make(map[string]interface{}, len(params))

	for _, k := range keys {
		sortedParams[k] = params[k]
	}

	keys = make([]string, 0)

	for k := range headers {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	sortedHeaders := make(map[string]interface{}, len(keys))
	for _, k := range keys {
		sortedHeaders[k] = headers[k]
	}

	// convert the sorted map into a string to append it with the API for the key
	b, _ := json.Marshal(sortedParams)
	key := api + ":" + string(b)

	// hash the headers and store it as key
	b, _ = json.Marshal(sortedHeaders)
	h := sha256.New()
	_, _ = h.Write(b)

	key += ":" + fmt.Sprintf("%x", h.Sum(nil))

	return key
}

// getHeaders adds the mandatory headers to the headers passed in the call. We need not add service level headers since
// they do not change for each request.
func (c cachedHTTPService) getHeaders(ctx context.Context, headers map[string]string) map[string]string {
	// add all the mandatory headers to the request
	if headers == nil {
		headers = make(map[string]string)
	}

	if val := ctx.Value(middleware.B3TraceIDKey); val != nil {
		b3TraceID, _ := val.(string)
		headers["X-B3-TraceID"] = b3TraceID
	}

	if val := ctx.Value(middleware.CorrelationIDKey); val != nil {
		correlationID, _ := val.(string)
		headers["X-Correlation-ID"] = correlationID
	}

	if val := ctx.Value(middleware.ClientIPKey); val != nil {
		clientIP, _ := val.(string)
		headers["True-Client-IP"] = clientIP
	}

	if val := ctx.Value(middleware.ZopsmartChannelKey); val != nil {
		zopsmartChannel, _ := val.(string)
		headers["X-Zopsmart-Channel"] = zopsmartChannel
	}

	if val := ctx.Value(middleware.AuthenticatedUserIDKey); val != nil {
		authUserID, _ := val.(string)
		headers["X-Authenticated-UserId"] = authUserID
	}

	if val := ctx.Value(middleware.ZopsmartTenantKey); val != nil {
		zopsmartTenant, _ := val.(string)
		headers["X-Zopsmart-Tenant"] = zopsmartTenant
	}

	if c.auth != "" {
		headers["Authorization"] = c.auth
	}

	return headers
}
