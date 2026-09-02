package exporters

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"

	"gofr.dev/pkg/gofr/logging"
)

func attrValue(attrs []attribute.KeyValue, key string) (string, bool) {
	for _, kv := range attrs {
		if string(kv.Key) == key {
			return kv.Value.String(), true
		}
	}

	return "", false
}

func Test_parseResourceAttributes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want map[string]string
	}{
		{"empty", "", map[string]string{}},
		{"whitespace only", "   ", map[string]string{}},
		{"single pair", "cloud.region=asia-south1", map[string]string{"cloud.region": "asia-south1"}},
		{
			"multiple pairs with spaces",
			" cloud.region=asia-south1 , faas.instance=abc123 ",
			map[string]string{"cloud.region": "asia-south1", "faas.instance": "abc123"},
		},
		{"pair without separator is skipped", "cloud.region", map[string]string{}},
		{"empty key is skipped", "=asia-south1", map[string]string{}},
		{"empty value is skipped", "cloud.region=", map[string]string{}},
		{"value may contain the separator", "url=http://x/y?a=b", map[string]string{"url": "http://x/y?a=b"}},
		{"percent-encoded value is decoded", "k8s.pod.name=my%20pod", map[string]string{"k8s.pod.name": "my pod"}},
		{"plus is literal, not a space", "tag=a+b", map[string]string{"tag": "a+b"}},
		{
			"malformed pair does not drop the good ones",
			"cloud.region=asia-south1,broken,faas.instance=abc",
			map[string]string{"cloud.region": "asia-south1", "faas.instance": "abc"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseResourceAttributes(tc.in)

			if len(got) != len(tc.want) {
				t.Fatalf("got %d attributes %v, want %d", len(got), got, len(tc.want))
			}

			for k, want := range tc.want {
				v, ok := attrValue(got, k)
				if !ok {
					t.Errorf("missing attribute %q", k)
					continue
				}

				if v != want {
					t.Errorf("attribute %q = %q, want %q", k, v, want)
				}
			}
		})
	}
}

// Config-sourced attributes are the only way a value set anywhere other than the
// process environment reaches the resource: resource.WithFromEnv reads the OS
// environment directly, so it cannot see configs/.env.
func TestBuildResource_configAttributesReachTheResource(t *testing.T) {
	cfg := Config{AppName: "app", ResourceAttributes: "location=asia-south1,service.instance.id=inst-1"}

	res := buildResource(context.Background(), &cfg, logging.NewMockLogger(logging.INFO))
	if res == nil {
		t.Fatal("expected a resource")
	}

	attrs := res.Attributes()

	for key, want := range map[string]string{
		"location":            "asia-south1",
		"service.instance.id": "inst-1",
		"service.name":        "app",
	} {
		got, ok := attrValue(attrs, key)
		if !ok {
			t.Errorf("missing attribute %q in %v", key, attrs)
			continue
		}

		if got != want {
			t.Errorf("attribute %q = %q, want %q", key, got, want)
		}
	}
}

// Both spellings are honored, and the GoFr-native one wins per key. The
// container concatenates them with its own value last; this asserts the ordering
// that makes that work.
func TestBuildResource_configAttributesWinOverEnvironment(t *testing.T) {
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "location=us-central1,cloud.region=us-central1")

	cfg := Config{AppName: "app", ResourceAttributes: "location=asia-south1"}

	res := buildResource(context.Background(), &cfg, logging.NewMockLogger(logging.INFO))
	if res == nil {
		t.Fatal("expected a resource")
	}

	attrs := res.Attributes()

	if got, _ := attrValue(attrs, "location"); got != "asia-south1" {
		t.Errorf("location = %q, want %q (config must win over the environment)", got, "asia-south1")
	}

	// A key the config does not mention must survive rather than be replaced.
	if got, _ := attrValue(attrs, "cloud.region"); got != "us-central1" {
		t.Errorf("cloud.region = %q, want %q (unmentioned env keys must survive)", got, "us-central1")
	}
}
