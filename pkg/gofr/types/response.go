package types

// Response denotes the response to a incoming Http request
type Response struct {
	// Data holds the data that needs to be served
	Data interface{} `json:"data" xml:"data"`
	// Meta holds the metadata that is requested
	Meta interface{} `json:"meta,omitempty" xml:"meta,omitempty"`
}
