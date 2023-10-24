package types

// RawWithOptions denotes a Raw type but can hold more information about the data
type RawWithOptions struct {
	// Data the raw data that needs to be served
	Data interface{}
	// ContentType denotes the type of data
	ContentType string
	// Headers contains any headers that needs to be passed while serving the data
	Header map[string]string
}
