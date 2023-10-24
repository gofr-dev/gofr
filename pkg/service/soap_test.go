package service

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gofr.dev/pkg/log"
)

// It is a test server that behaves like SOAP API. Based on the different SOAP actions, it returns the different desired responses.
func testSOAPServer() *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var res string
		switch r.Header.Get("SOAPAction") {
		case "gfr":
			res = `<?xml version="1.0" encoding="utf-8"?>
						<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
							<soap:Body>
        						<m:CompanyNameResponse xmlns:m="http://www.zopsmart.com">
           						 	<m:CompanyNameResult>Gofr</m:CompanyNameResult>
								</m:CompanyNameResponse>
   							 </soap:Body>
						</soap:Envelope>`
		case "zop":
			res = `<?xml version="1.0" encoding="utf-8"?>
						<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
							<soap:Body>
        						<m:CompanyNameResponse xmlns:m="http://www.zopsmart.com">
           						 	<m:CompanyNameResult>ZopSmart</m:CompanyNameResult>
								</m:CompanyNameResponse>
   							 </soap:Body>
						</soap:Envelope>`
		}

		resBytes, _ := xml.Marshal(res)
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write(resBytes)
	}))

	return ts
}

// TestCallWithHeaders_SOAP tests the SOAP client with a test soap server
func TestSOAPServer(t *testing.T) {
	ts := testSOAPServer()
	defer ts.Close()

	tests := []struct {
		name string
		// Input
		action string
		// Output
		out string
	}{
		{"action/gfr", "gfr", "Gofr"},
		{"action/zop", "zop", "ZopSmart"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ps := NewSOAPClient(ts.URL, log.NewMockLogger(io.Discard), "", "")
			res, err := ps.Call(context.Background(), tc.action, nil)

			if !strings.Contains(string(res.Body), tc.out) {
				t.Errorf("Unexpected Response : %v", string(res.Body))
			}

			if err != nil {
				t.Errorf("Test %v:\t  error = %v", tc.name, err)
			}
		})
	}
}

// TestCallWithHeaders_SOAP tests the SOAP client by passing custom headers
func TestCallWithHeaders_SOAP(t *testing.T) {
	ts := testSOAPServer()
	defer ts.Close()

	tests := []struct {
		action      string
		headers     map[string]string
		expectedLog string
	}{
		{"zop", map[string]string{"X-Trace-Id": "a123ru", "X-B-Trace-Id": "198d7sf3d"}, `"X-B-Trace-Id":"198d7sf3d","X-Trace-Id":"a123ru"`},
		{"gfr", map[string]string{"X-Zopsmart-Tenant": "zopsmart"}, `"X-Zopsmart-Tenant":"zopsmart"`},
		{"gfr", nil, ``},
	}

	for i, tc := range tests {
		b := new(bytes.Buffer)

		soapClient := NewSOAPClient(ts.URL, log.NewMockLogger(b), "basic-user", "password")

		_, err := soapClient.CallWithHeaders(context.Background(), tc.action, nil, tc.headers)
		if err != nil {
			t.Errorf("Error: %v", err)
		}

		if !strings.Contains(b.String(), tc.expectedLog) {
			t.Errorf("test id  %d headers is not logged", i+1)
		}
	}
}

func Test_SOAPBind_BindStrict_ERROR(t *testing.T) {
	s := soapService{httpService{}}
	tc := []struct {
		f         func(resp []byte, i interface{}) error
		responses []byte
	}{
		{s.Bind, []byte(`{"name":"jerry"}`)},
		{s.BindStrict, []byte(`{"name":"jerry"}`)},
		{s.Bind, []byte(`<>`)},
		{s.BindStrict, []byte(`<>`)},
		{s.BindStrict, []byte(`<Customer><ID>Jerry</ID></Customer>`)},
	}

	for i := range tc {
		type Customer struct {
			ID int
		}

		var c Customer

		err := tc[i].f(tc[i].responses, &c)
		if err == nil {
			t.Errorf("[TESTCASE%d]Failed.expected error but got nil", i+1)
		}

		if s.httpService.contentType != XML {
			t.Errorf("[TESTCASE%d]Failed.Invalid content type. Expected XML Got %v", i+1, s.httpService.contentType)
		}
	}
}

func Test_SOAPBindStrict_Success(t *testing.T) {
	s := soapService{httpService{}}
	tc := []struct {
		f         func(resp []byte, i interface{}) error
		responses []byte
	}{
		{s.BindStrict, []byte(`<Customer><ID>1</ID></Customer>`)},
		{s.Bind, []byte(`<Customer><ID>1</ID></Customer>`)},
	}

	for i := range tc {
		type Customer struct {
			ID int
		}

		var c Customer

		err := tc[i].f(tc[i].responses, &c)
		if err != nil {
			t.Errorf("[TESTCASE%d]Failed.expected no error but got %v", i+1, err)
		}
	}
}
