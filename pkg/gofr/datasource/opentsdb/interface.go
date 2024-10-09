package opentsdb

import "context"

type OpentsdbProvider interface {
	OpentsDBClient
	Provider
}

// ClientContext implements the Client interface and additionally provides a
// way to return a client that is associated with the given context.
type OpentsdbProviderWithContext interface {
	// WithContext returns a Client that is associated with the given context.
	// Use this to pass a context to underlying transport (e.g. to specify a
	// deadline).
	WithContext(ctx context.Context) OpentsDBClient
	OpentsdbProvider
}

// OpentsDBClient provides methods for GoFr applications to communicate with OpenTSDB
// through its REST APIs. Each method corresponds to an API endpoint as defined in
// the OpenTSDB documentation (http://opentsdb.net/docs/build/html/api_http/index.html#api-endpoints).
type OpentsDBClient interface {

	// HealthCheck checks if the target OpenTSDB server is reachable.
	// It returns an error if the server is unreachable, otherwise returns nil.
	HealthCheck() (any, error)

	// Put handles the 'POST /api/put' endpoint, allowing the storage of data in OpenTSDB.
	//
	// Parameters:
	// - data: A slice of DataPoint objects, which must contain at least one instance.
	// - queryParam: Can be one of the following:
	//   - client.PutRespWithSummary: Requests a summary of the put operation.
	//   - client.PutRespWithDetails: Requests detailed information about the put operation.
	//   - An empty string (""): Indicates no additional response details are required.
	//
	// Return:
	// - On success, it returns a pointer to a PutResponse, along with the HTTP status code and relevant response information.
	// - On failure (due to invalid parameters, response parsing errors, or OpenTSDB connectivity issues), it returns an error.
	//
	// Notes:
	// - Use 'PutRespWithSummary' to receive summarized information about the data that was stored.
	// - Use 'PutRespWithDetails' for a more comprehensive breakdown of the put operation.
	Put(data []DataPoint, queryParam string) (*PutResponse, error)

	// Query implements the 'GET /api/query' endpoint for extracting data
	// in various formats based on the selected serializer.
	//
	// Parameters:
	// - param: An instance of QueryParam containing the current query parameters.
	//
	// Returns:
	// - *QueryResponse on success (status code and response info).
	// - Error on failure (invalid parameters, response parsing failure, or OpenTSDB connection issues).
	Query(param *QueryParam) (*QueryResponse, error)

	// QueryLast is the implementation of 'GET /api/query/last' endpoint.
	// It is introduced firstly in v2.1, and fully supported in v2.2. So it should be aware that this api works
	// well since v2.2 of opentsdb.
	//
	// param is a instance of QueryLastParam holding current query parameters.
	//
	// When query operation is successful, a pointer of QueryLastResponse will be returned with the corresponding
	// status code and response info. Otherwise, an error instance will be returned, when the given parameter
	// is invalid, it failed to parse the response, or OpenTSDB is un-connectable right now.
	QueryLast(param *QueryLastParam) (*QueryLastResponse, error)

	// Aggregators is the implementation of 'GET /api/aggregators' endpoint.
	// It simply lists the names of implemented aggregation functions used in time series queries.
	//
	// When query operation is successful, a pointer of AggregatorsResponse will be returned with the corresponding
	// status code and response info. Otherwise, an error instance will be returned, when it failed to parse the
	// response, or OpenTSDB is un-connectable right now.
	Aggregators() (*AggregatorsResponse, error)

	// Suggest is the implementation of 'GET /api/suggest' endpoint.
	// It provides a means of implementing an "auto-complete" call that can be accessed repeatedly as a user
	// types a request in a GUI. It does not offer full text searching or wildcards, rather it simply matches
	// the entire string passed in the query on the first characters of the stored data.
	// For example, passing a query of type=metrics&q=sys will return the top 25 metrics in the system that start with sys.
	// Matching is case sensitive, so sys will not match System.CPU. Results are sorted alphabetically.
	//
	// sugParm is an instance of SuggestParam storing parameters by invoking /api/suggest.
	//
	// When query operation is successful, a pointer of SuggestResponse will be returned with the corresponding
	// status code and response info. Otherwise, an error instance will be returned, if the given parameter is invalid,
	// or when it failed to parese the response, or OpenTSDB is un-connectable right now.
	Suggest(sugParm *SuggestParam) (*SuggestResponse, error)

	// Dropcaches is the implementation of 'GET /api/dropcaches' endpoint.
	// It purges the in-memory data cached in OpenTSDB. This includes all UID to name
	// and name to UID maps for metrics, tag names and tag values.
	//
	// When query operation is successful, a pointer of DropcachesResponse will be returned with the corresponding
	// status code and response info. Otherwise, an error instance will be returned, when it failed to parese the
	// response, or OpenTSDB is un-connectable right now.
	Dropcaches() (*DropcachesResponse, error)

	// QueryAnnotation is the implementation of 'GET /api/annotation' endpoint.
	// It retrieves a single annotation stored in the OpenTSDB backend.
	//
	// queryAnnoParam is a map storing parameters of a target queried annotation.
	// The key can be such as client.AnQueryStartTime, client.AnQueryTSUid.
	//
	// When query operation is handlering properly by the OpenTSDB backend, a pointer of AnnotationResponse
	// will be returned with the corresponding status code and response info (including the potential error
	// messages replied by OpenTSDB).
	//
	// Otherwise, an error instance will be returned, if the given parameter is invalid,
	// or when it failed to parese the response, or OpenTSDB is un-connectable right now.
	//
	// Note that: the returned non-nil error instance is only responsed by opentsdb-client, not the OpenTSDB backend.
	QueryAnnotation(queryAnnoParam map[string]interface{}) (*AnnotationResponse, error)

	// UpdateAnnotation is the implementation of 'POST /api/annotation' endpoint.
	// It creates or modifies an annotation stored in the OpenTSDB backend.
	//
	// annotation is an annotation to be processed in the OpenTSDB backend.
	//
	// When modification operation is handlering properly by the OpenTSDB backend, a pointer of AnnotationResponse
	// will be returned with the corresponding status code and response info (including the potential error
	// messages replied by OpenTSDB).
	//
	// Otherwise, an error instance will be returned, if the given parameter is invalid,
	// or when it failed to parese the response, or OpenTSDB is un-connectable right now.
	//
	// Note that: the returned non-nil error instance is only responsed by opentsdb-client, not the OpenTSDB backend.
	UpdateAnnotation(annotation *Annotation) (*AnnotationResponse, error)

	// DeleteAnnotation is the implementation of 'DELETE /api/annotation' endpoint.
	// It deletes an annotation stored in the OpenTSDB backend.
	//
	// annotation is an annotation to be deleted in the OpenTSDB backend.
	//
	// When deleting operation is handlering properly by the OpenTSDB backend, a pointer of AnnotationResponse
	// will be returned with the corresponding status code and response info (including the potential error
	// messages replied by OpenTSDB).
	//
	// Otherwise, an error instance will be returned, if the given parameter is invalid,
	// or when it failed to parese the response, or OpenTSDB is un-connectable right now.
	//
	// Note that: the returned non-nil error instance is only responsed by opentsdb-client, not the OpenTSDB backend.
	DeleteAnnotation(annotation *Annotation) (*AnnotationResponse, error)

	// BulkUpdateAnnotations is the implementation of 'POST /api/annotation/bulk' endpoint.
	// It creates or modifies a list of annotation stored in the OpenTSDB backend.
	//
	// annotations is a list of annotations to be processed (to be created or modified) in the OpenTSDB backend.
	//
	// When bulk modification operation is handlering properly by the OpenTSDB backend, a pointer of BulkAnnotatResponse
	// will be returned with the corresponding status code and response info (including the potential error
	// messages replied by OpenTSDB).
	//
	// Otherwise, an error instance will be returned, if the given parameter is invalid,
	// or when it failed to parese the response, or OpenTSDB is un-connectable right now.
	//
	// Note that: the returned non-nil error instance is only responsed by opentsdb-client, not the OpenTSDB backend.
	BulkUpdateAnnotations(annotations []Annotation) (*BulkAnnotatResponse, error)

	// BulkDeleteAnnotations is the implementation of 'DELETE /api/annotation/bulk' endpoint.
	// It deletes a list of annotation stored in the OpenTSDB backend.
	//
	// bulkDelParam contains the bulk deleting info in current invoking 'DELETE /api/annotation/bulk'.
	//
	// When bulk deleting operation is handlering properly by the OpenTSDB backend, a pointer of BulkAnnotatResponse
	// will be returned with the corresponding status code and response info (including the potential error
	// messages replied by OpenTSDB).
	//
	// Otherwise, an error instance will be returned, if the given parameter is invalid,
	// or when it failed to parese the response, or OpenTSDB is un-connectable right now.
	//
	// Note that: the returned non-nil error instance is only responsed by opentsdb-client, not the OpenTSDB backend.
	BulkDeleteAnnotations(bulkDelParam *BulkAnnoDeleteInfo) (*BulkAnnotatResponse, error)

	// QueryUIDMetaData is the implementation of 'GET /api/uid/uidmeta' endpoint.
	// It retrieves a single UIDMetaData stored in the OpenTSDB backend with the given query parameters.
	//
	// metaQueryParam is a map storing parameters of a target queried UIDMetaData.
	// It must contain two key/value pairs with the key "uid" and "type".
	// "type" should be one of client.TypeMetrics ("metric"), client.TypeTagk ("tagk"), and client.TypeTagv ("tagv")
	//
	// When query operation is handlering properly by the OpenTSDB backend, a pointer of UIDMetaDataResponse
	// will be returned with the corresponding status code and response info (including the potential error
	// messages replied by OpenTSDB).
	//
	// Otherwise, an error instance will be returned, if the given parameter is invalid,
	// or when it failed to parese the response, or OpenTSDB is un-connectable right now.
	//
	// Note that: the returned non-nil error instance is only responsed by opentsdb-client, not the OpenTSDB backend.
	QueryUIDMetaData(metaQueryParam map[string]string) (*UIDMetaDataResponse, error)

	// UpdateUIDMetaData is the implementation of 'POST /api/uid/uidmeta' endpoint.
	// It modifies a UIDMetaData.
	//
	// uidMetaData is an instance of UIDMetaData to be modified
	//
	// When update operation is handlering properly by the OpenTSDB backend, a pointer of UIDMetaDataResponse
	// will be returned with the corresponding status code and response info (including the potential error
	// messages replied by OpenTSDB).
	//
	// Otherwise, an error instance will be returned, if the given parameter is invalid,
	// or when it failed to parese the response, or OpenTSDB is un-connectable right now.
	//
	// Note that: the returned non-nil error instance is only responsed by opentsdb-client, not the OpenTSDB backend.
	UpdateUIDMetaData(uidMetaData *UIDMetaData) (*UIDMetaDataResponse, error)

	// DeleteUIDMetaData is the implementation of 'DELETE /api/uid/uidmeta' endpoint.
	// It deletes a target UIDMetaData.
	//
	// uidMetaData is an instance of UIDMetaData whose correspance is to be deleted.
	// The values of uid and type in uidMetaData is required.
	//
	// When delete operation is handlering properly by the OpenTSDB backend, a pointer of UIDMetaDataResponse
	// will be returned with the corresponding status code and response info (including the potential error
	// messages replied by OpenTSDB).
	//
	// Otherwise, an error instance will be returned, if the given parameter is invalid,
	// or when it failed to parese the response, or OpenTSDB is un-connectable right now.
	//
	// Note that: the returned non-nil error instance is only responsed by opentsdb-client, not the OpenTSDB backend.
	DeleteUIDMetaData(uidMetaData *UIDMetaData) (*UIDMetaDataResponse, error)

	// AssignUID is the implementation of 'POST /api/uid/assigin' endpoint.
	// It enables assigning UIDs to new metrics, tag names and tag values. Multiple types and names can be provided
	// in a single call and the API will process each name individually, reporting which names were assigned UIDs
	// successfully, along with the UID assigned, and which failed due to invalid characters or had already been assigned.
	// Assignment can be performed via query string or content data.
	//
	// assignParam is an instance of UIDAssignParam holding the parameters to invoke 'POST /api/uid/assigin'.
	//
	// When assigin operation is handlering properly by the OpenTSDB backend, a pointer of UIDAssignResponse
	// will be returned with the corresponding status code and response info (including the potential error
	// messages replied by OpenTSDB).
	//
	// Otherwise, an error instance will be returned, if the given parameter is invalid,
	// or when it failed to parese the response, or OpenTSDB is un-connectable right now.
	//
	// Note that: the returned non-nil error instance is only responsed by opentsdb-client, not the OpenTSDB backend.
	AssignUID(assignParam *UIDAssignParam) (*UIDAssignResponse, error)

	// QueryTSMetaData is the implementation of 'GET /api/uid/tsmeta' endpoint.
	// It retrieves a single TSMetaData stored in the OpenTSDB backend with the given query parameters.
	//
	// tsuid is a tsuid of a target queried TSMetaData.
	//
	// When query operation is handlering properly by the OpenTSDB backend, a pointer of TSMetaDataResponse
	// will be returned with the corresponding status code and response info (including the potential error
	// messages replied by OpenTSDB).
	//
	// Otherwise, an error instance will be returned, if the given parameter is invalid,
	// or when it failed to parese the response, or OpenTSDB is un-connectable right now.
	//
	// Note that: the returned non-nil error instance is only responsed by opentsdb-client, not the OpenTSDB backend.
	QueryTSMetaData(tsuid string) (*TSMetaDataResponse, error)

	// UpdateTSMetaData is the implementation of 'POST /api/uid/tsmeta' endpoint.
	// It modifies a target TSMetaData with the given fields.
	//
	// tsMetaData is an instance of UIDMetaData whose correspance is to be modified
	//
	// When update operation is handlering properly by the OpenTSDB backend, a pointer of TSMetaDataResponse
	// will be returned with the corresponding status code and response info (including the potential error
	// messages replied by OpenTSDB).
	//
	// Otherwise, an error instance will be returned, when it failed to parese the response,
	// or OpenTSDB is un-connectable right now.
	//
	// Note that: the returned non-nil error instance is only responsed by opentsdb-client, not the OpenTSDB backend.
	UpdateTSMetaData(tsMetaData *TSMetaData) (*TSMetaDataResponse, error)

	// DeleteTSMetaData is the implementation of 'DELETE /api/uid/tsmeta' endpoint.
	// It deletes a target TSMetaData.
	//
	// tsMetaData is an instance of UIDMetaData whose correspance is to be deleted
	//
	// When delete operation is handlering properly by the OpenTSDB backend, a pointer of TSMetaDataResponse
	// will be returned with the corresponding status code and response info (including the potential error
	// messages replied by OpenTSDB).
	//
	// Otherwise, an error instance will be returned, when it failed to parese the response,
	// or OpenTSDB is un-connectable right now.
	//
	// Note that: the returned non-nil error instance is only responsed by opentsdb-client, not the OpenTSDB backend.
	DeleteTSMetaData(tsMetaData *TSMetaData) (*TSMetaDataResponse, error)
}

// Response defines the common behaviours all the specific response for
// different rest-apis shound obey.
// Currently it is an abstraction used in (*clientImpl).sendRequest()
// to stored the different kinds of response contents for all the rest-apis.
type Response interface {

	// SetStatus can be used to set the actual http status code of
	// the related http response for the specific Response instance
	SetStatus(code int)

	// GetCustomParser can be used to retrive a custom-defined parser.
	// Returning nil means current specific Response instance doesn't
	// need a custom-defined parse process, and just uses the default
	// json unmarshal method to parse the contents of the http response.
	GetCustomParser() func(respCnt []byte) error

	// Return the contents of the specific Response instance with
	// the string format
	String() string
}

type Provider interface {
	// UseLogger sets the logger for the OpentsdbClient.
	UseLogger(logger interface{})

	// UseMetrics sets the metrics for the OpentsdbClient.
	UseMetrics(metrics interface{})

	// UseTracer sets the tracer for the OpentsdbClient.
	UseTracer(tracer any)

	// Connect establishes a connection to FileSystem and registers metrics using the provided configuration when the client was Created.
	Connect()
}

// Logger interface is used by opentsdb package to log information about request execution.
type Logger interface {
	Debug(args ...interface{})
	Logf(pattern string, args ...interface{})
	Errorf(pattern string, args ...interface{})
}

type Metrics interface {
	NewHistogram(name, desc string, buckets ...float64)

	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
}
