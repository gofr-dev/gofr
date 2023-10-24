package service

import (
	"context"
	"encoding/base64"
	"net/http"
	"time"

	"gofr.dev/pkg/log"

	"go.opencensus.io/plugin/ochttp"
)

type soapService struct {
	httpService
}

// NewSOAPClient sets up an HTTP client with specific transport and timeout settings, configures surge protection,
// and prepares the SOAP service for use.
//
//nolint:revive // this type cannot be exported since we don't want the user to have access to the members
func NewSOAPClient(resourceURL string, logger log.Logger, user, pass string) *soapService {
	auth := ""
	if user != "" {
		auth = "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
	}

	transport := &ochttp.Transport{}

	return &soapService{
		httpService{
			url:    resourceURL,
			logger: logger,
			Client: &http.Client{Transport: transport, Timeout: RetryFrequency * time.Second}, // default timeout is 5 seconds
			sp: surgeProtector{
				isEnabled:             false,
				customHeartbeatURL:    "/.well-known/heartbeat",
				retryFrequencySeconds: RetryFrequency,
				logger:                logger,
			},
			auth:      auth,
			isHealthy: true,
			healthCh:  make(chan bool),
		}}
}

// Call is a soap call for the given SOAP Action and body. The only allowed method in SOAP is POST
func (s *soapService) Call(ctx context.Context, action string, body []byte) (*Response, error) {
	return s.call(ctx, http.MethodPost, "", nil, body, map[string]string{"SOAPAction": action, "Content-Type": "text/xml"})
}

// CallWithHeaders is a soap call for the given SOAP Action and body. The only allowed method in SOAP is POST
func (s *soapService) CallWithHeaders(ctx context.Context, action string, body []byte, headers map[string]string) (*Response, error) {
	if headers == nil {
		headers = make(map[string]string)
	}

	headers["SOAPAction"] = action
	headers["Content-Type"] = "text/xml"

	return s.call(ctx, http.MethodPost, "", nil, body, headers)
}

// Bind takes Response and binds it to i based on content-type.
func (s *soapService) Bind(resp []byte, i interface{}) error {
	s.httpService.contentType = XML

	return s.httpService.Bind(resp, i)
}

// BindStrict takes Response and binds it to i based on content-type.
func (s *soapService) BindStrict(resp []byte, i interface{}) error {
	s.httpService.contentType = XML

	return s.httpService.Bind(resp, i)
}
