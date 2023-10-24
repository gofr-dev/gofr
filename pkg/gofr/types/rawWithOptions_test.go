package types

import (
	"encoding/json"
	"encoding/xml"
	"fmt"

	"testing"
)

type RCT RawWithOptions

type xmlMapEntry struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

func (r RCT) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if len(r.Header) == 0 {
		return nil
	}

	err := e.EncodeToken(start)
	if err != nil {
		return err
	}

	for k, v := range r.Header {
		err := e.Encode(xmlMapEntry{XMLName: xml.Name{Local: k}, Value: v})
		if err != nil {
			return err
		}
	}

	return e.EncodeToken(start.End())
}

func TestRawWithContentType(t *testing.T) {
	check := RawWithOptions{Data: "testData", ContentType: "test", Header: map[string]string{"Test": "Pass"}}
	xmlCheck, _ := xml.MarshalIndent(RCT(check), "", "")

	jsonCheck, _ := json.Marshal(check)
	textCheck := fmt.Sprint(check)

	expectedJSON := `{"Data":"testData","ContentType":"test","Header":{"Test":"Pass"}}`
	expectedXML := `<RCT><Test>Pass</Test></RCT>`
	expectedTEXT := `{testData test map[Test:Pass]}`

	if string(jsonCheck) != expectedJSON || string(xmlCheck) != expectedXML || textCheck != expectedTEXT {
		t.Errorf("Error in marshaling")
	}
}
