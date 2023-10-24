package store

import "testing"

func TestFilter_GenSolrQuery(t *testing.T) {
	testcases := []struct {
		input  Filter
		output string
	}{
		{Filter{}, ""},
		{Filter{"1234", ""}, "id:1234 "},
		{Filter{"", "Henry"}, "name:Henry "},
		{Filter{"123", "Henry"}, "id:123 AND name:Henry "},
	}

	for i, tc := range testcases {
		resp := tc.input.GenSolrQuery()
		if resp != tc.output {
			t.Errorf("[TEST CASE %d]Expected %v\tGot %v\n", i+1, tc.output, resp)
		}
	}
}
