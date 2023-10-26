package eventbridge

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/awstesting/mock"
	pkgEventbridge "github.com/aws/aws-sdk-go/service/eventbridge"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg"
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/datastore/pubsub"
	"gofr.dev/pkg/gofr/types"
)

func Test_PubSub_New(t *testing.T) {
	cfg := &Config{
		Region:      "us-east-1",
		EventBus:    "Gofr",
		EventSource: "application",
	}

	_, err := New(cfg)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func Test_PubSub_EventBridge_PublishEvent(t *testing.T) {
	ch := make(chan int)
	tcs := []struct {
		region string
		detail interface{}
		err    error
	}{
		{"", "sample payload", awserr.New("MissingRegion", "could not find region configuration", nil)},
		{"us-east-1", ch, &json.UnsupportedTypeError{Type: reflect.TypeOf(ch)}},
		{"us-east-1", "sample payload", nil},
	}

	for i, tc := range tcs {
		var eBridge Client

		awscfg := aws.NewConfig().WithRegion(tc.region)
		awscfg.Credentials = credentials.NewStaticCredentials("AKID", "SECRET_KEY", "TOKEN")

		eBridge.client = pkgEventbridge.New(mock.Session, awscfg)
		eBridge.cfg = &Config{EventBus: "gofr", EventSource: "application"}
		eb := &eBridge
		err := eb.PublishEvent("myDetailType", tc.detail, map[string]string{})
		assert.Equal(t, tc.err, err, "Test case failed [%v]\n Expected: %v, got: %v", i, tc.err, err)
	}
}

func Test_PubSub_EventBridge_HealthCheck(t *testing.T) {
	var eBridge Client

	awscfg := aws.NewConfig().WithRegion("us-west-2")
	awscfg.Credentials = credentials.NewStaticCredentials("AKID", "SECRET_KEY", "TOKEN")

	eBridge.client = pkgEventbridge.New(mock.Session, awscfg)
	eBridge.cfg = &Config{EventBus: "gofr", EventSource: "application"}
	eb := &eBridge

	testcases := []struct {
		client  *Client
		expResp types.Health
	}{
		{client: nil, expResp: types.Health{Name: datastore.EventBridge, Status: pkg.StatusDown}},
		{client: &Client{client: nil, cfg: &Config{EventBus: "gofr", Region: "us-west-2"}},
			expResp: types.Health{Name: datastore.EventBridge, Status: pkg.StatusDown, Host: "us-west-2", Database: "gofr"}},
		{client: eb, expResp: types.Health{Name: datastore.EventBridge, Status: pkg.StatusUp, Host: "", Database: "gofr"}},
	}
	for i, tc := range testcases {
		resp := tc.client.HealthCheck()
		assert.Equalf(t, tc.expResp, resp, "Test case failed [%v]. Expected: %v, got: %v", i, tc.expResp, resp)
	}
}

func Test_PubSub_EventBridge_PublishEventWithOptions(t *testing.T) {
	c := Client{}

	err := c.PublishEventWithOptions("", "", map[string]string{}, &pubsub.PublishOptions{})
	if err != nil {
		t.Error("Test case failed.")
	}
}

func Test_PubSub_EventBridge_Subscribe(t *testing.T) {
	c := Client{}

	_, err := c.Subscribe()
	if err != nil {
		t.Error("Test case failed")
	}
}

func Test_PubSub_EventBridge_SubscribeWithCommit(t *testing.T) {
	c := Client{}
	f := func(message *pubsub.Message) (bool, bool) { return false, false }

	_, err := c.SubscribeWithCommit(f)
	if err != nil {
		t.Error("Test case failed")
	}
}

func Test_PubSub_EventBridge_Bind(t *testing.T) {
	var k string

	c := Client{}

	err := c.Bind([]byte(`{"test":"test"}`), k)
	if err != nil {
		t.Error("Test case failed.")
	}
}

func Test_PubSub_EventBridge_Ping(t *testing.T) {
	c := Client{}

	err := c.Ping()
	if err != nil {
		t.Error("Test case failed")
	}
}

func Test_PubSub_EventBridge_IsSet(t *testing.T) {
	var eBridge Client

	awscfg := aws.NewConfig().WithRegion("us-west-2")
	awscfg.Credentials = credentials.NewStaticCredentials("AKID", "SECRET_KEY", "TOKEN")

	eBridge.client = pkgEventbridge.New(mock.Session, awscfg)
	eBridge.cfg = &Config{EventBus: "gofr", EventSource: "application"}
	eb := &eBridge

	testcases := []struct {
		client  *Client
		expResp bool
	}{
		{client: nil, expResp: false},
		{client: &Client{client: nil, cfg: &Config{}}, expResp: false},
		{client: eb, expResp: true},
	}
	for i, tc := range testcases {
		resp := tc.client.IsSet()
		assert.Equalf(t, tc.expResp, resp, "Test case failed [%v]. \n Expected: %v, got %v", i, tc.expResp, resp)
	}
}

func Test_PubSub_EventBridge_Retrieve(t *testing.T) {
	tests := []struct {
		desc        string
		credentials customProvider
		expResp     credentials.Value
	}{
		{"empty credentials", customProvider{}, credentials.Value{}},
		{"valid credentials", customProvider{keyID: "testID", secretKey: "testKey"},
			credentials.Value{AccessKeyID: "testID", SecretAccessKey: "testKey"}},
	}

	for i, tc := range tests {
		resp, err := tc.credentials.Retrieve()

		assert.Equal(t, tc.expResp, resp, "Test case [%d] failed.", i)

		assert.Nil(t, err, "Test case [%d] failed.", i)
	}
}

func Test_PubSub_EventBridge_IsExpired(t *testing.T) {
	tests := []struct {
		desc        string
		credentials customProvider
	}{
		{"empty credentials", customProvider{}},
		{"valid credentials", customProvider{keyID: "testID", secretKey: "testKey"}},
	}

	for i, tc := range tests {
		isExpired := tc.credentials.IsExpired()

		assert.Equal(t, false, isExpired, "Test case [%d] failed.", i)
	}
}
