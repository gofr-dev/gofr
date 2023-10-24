package types

import (
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"
)

func TestResponse(t *testing.T) {
	check := Response{Data: "testData", Meta: "testData"}
	xmlCheck, _ := xml.Marshal(check)
	jsonCheck, _ := json.Marshal(check)

	if !strings.Contains(string(jsonCheck), "data") || !strings.Contains(string(xmlCheck), "meta") {
		t.Errorf("Error in marshaling")
	}
}
