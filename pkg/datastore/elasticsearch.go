package datastore

import (
	"encoding/json"
	"fmt"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"

	"gofr.dev/pkg"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/log"
)

// ElasticSearchCfg  stores the configuration parameters required to connect to Elasticsearch.
type ElasticSearchCfg struct {
	Host                    string
	Ports                   []int
	Username                string
	Password                string
	CloudID                 string
	ConnectionRetryDuration int
}

// Elasticsearch stores the elasticSearch client along with logger and configs to connect to ElasticSearch DB.
type Elasticsearch struct {
	*elasticsearch.Client
	config *ElasticSearchCfg
	logger log.Logger
}

// NewElasticsearchClient factory function for Elasticsearch
func NewElasticsearchClient(logger log.Logger, c *ElasticSearchCfg) (Elasticsearch, error) {
	addresses := make([]string, 0)

	for _, port := range c.Ports {
		addresses = append(addresses, fmt.Sprintf("http://%s:%v", c.Host, port))
	}

	cfg := elasticsearch.Config{Addresses: addresses, Username: c.Username, Password: c.Password, CloudID: c.CloudID}

	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return Elasticsearch{config: c, logger: logger}, err
	}

	return Elasticsearch{Client: client, config: c, logger: logger}, nil
}

// Ping makes a call to check connection with elastic search
func (e *Elasticsearch) Ping() (*esapi.Response, error) {
	return e.Client.Info()
}

// HealthCheck return the Health of the elastic search client
func (e *Elasticsearch) HealthCheck() types.Health {
	resp := types.Health{
		Name:   ElasticSearch,
		Status: pkg.StatusDown,
		Host:   e.config.Host,
	}
	// The following check is for the condition when the connection to Elasticsearch has not been made during initialization
	if e.Client == nil {
		e.logger.Errorf("%v", errors.HealthCheckFailed{Dependency: ElasticSearch, Reason: "Elastic search not initialized."})
		return resp
	}

	_, err := e.Ping()
	if err != nil {
		return resp // error getting response
	}

	resp.Status = pkg.StatusUp

	return resp
}

// Hits retrieves the data from the response which returned by elasticsearch client
func (e *Elasticsearch) Hits(res *esapi.Response) ([]interface{}, error) {
	r, err := e.Body(res)
	if err != nil {
		return nil, err
	}

	// to unmarshal the data retrieves form the hits
	hits := struct {
		Hits struct {
			Hits []interface{} `json:"hits"`
		} `json:"hits"`
	}{}

	err = bind(r, &hits)
	if err != nil {
		return nil, err
	}

	return hits.Hits.Hits, nil
}

// Body retrieves body from the response which returned by elasticsearch client
func (e *Elasticsearch) Body(res *esapi.Response) (map[string]interface{}, error) {
	var r map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return nil, err
	}

	defer res.Body.Close()

	return r, nil
}

// Bind binds the response returned by the elasticsearch client to target(should not be array)
func (e *Elasticsearch) Bind(res *esapi.Response, target interface{}) error {
	data, err := e.getData(res)
	if err != nil || len(data) == 0 {
		return err
	}

	err = bind(data[0], target)
	if err != nil {
		return err
	}

	return nil
}

// BindArray binds the response returned by the elasticsearch client to target(should be array)
func (e *Elasticsearch) BindArray(res *esapi.Response, target interface{}) error {
	data, err := e.getData(res)
	if err != nil {
		return err
	}

	err = bind(data, target)
	if err != nil {
		return err
	}

	return nil
}

func (e *Elasticsearch) getData(res *esapi.Response) ([]interface{}, error) {
	hits, err := e.Hits(res)
	if err != nil {
		return nil, err
	}

	if len(hits) == 0 {
		return nil, nil
	}

	var tempData []struct {
		Source interface{} `json:"_source"`
	}

	err = bind(hits, &tempData)
	if err != nil {
		e.logger.Errorf("error in binding: %v", err)
		return nil, err
	}

	data := make([]interface{}, len(tempData))

	for i := range tempData {
		data[i] = tempData[i].Source
	}

	return data, nil
}

func bind(data, resp interface{}) error {
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}

	err = json.Unmarshal(body, resp)
	if err != nil {
		return err
	}

	return nil
}
