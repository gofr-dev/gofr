package config

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	"gofr.dev/pkg/errors"
)

type config interface {
	Get(string) string
	GetOrDefault(string, string) string
}

const refreshFrequency = 30

// RemoteConfig holds the information about the remote config server to maintain and manage the configs for the
// application using a Remote Override Service
type RemoteConfig struct {
	// contains unexported fields
	remoteConfig map[string]string
	localConfig  config
	logger       logger
	appName      string
	url          string
	namespace    string
	cluster      string
	userGroup    string
	frequency    int
}

// NewRemoteConfigProvider creates a new instance of RemoteConfig for fetching remote configurations.
func NewRemoteConfigProvider(localConfig config, remoteConfigURL, appName string, logger logger) *RemoteConfig {
	remoteConfig := make(map[string]string)
	r := &RemoteConfig{
		remoteConfig: remoteConfig,
		localConfig:  localConfig,
		logger:       logger,
		appName:      appName,
		url:          remoteConfigURL,
		frequency:    refreshFrequency,
	}

	r.namespace = localConfig.Get("REMOTE_NAMESPACE")
	checkConfig(r.namespace, "REMOTE_NAMESPACE", logger)

	r.cluster = localConfig.Get("REMOTE_CLUSTER")
	checkConfig(r.cluster, "REMOTE_CLUSTER", logger)

	r.userGroup = localConfig.Get("REMOTE_USER_GROUP")
	checkConfig(r.userGroup, "REMOTE_USER_GROUP", logger)

	go r.refreshConfigs()

	return r
}

// Get retrieves a configuration value by its key. It first checks the remote configurations,
// and if not found, falls back to the local configurations.
func (r *RemoteConfig) Get(key string) string {
	var value string

	if r.remoteConfig != nil {
		value = r.remoteConfig[key]
	}

	if r.localConfig != nil && value == "" {
		value = r.localConfig.Get(key)
	}

	return value
}

// GetOrDefault retrieves a configuration value by its key and returns a default value if not found.
func (r *RemoteConfig) GetOrDefault(key, defaultValue string) string {
	value := r.Get(key)
	if value == "" {
		return defaultValue
	}

	return value
}

func (r *RemoteConfig) refreshConfigs() {
	//nolint:gosec // need this to skip TLS verification
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	queryParams := url.Values{}
	// Add the parameters to the map
	queryParams.Set("serviceName", r.appName)
	queryParams.Set("namespace", r.namespace)
	queryParams.Set("userGroup", r.userGroup)
	queryParams.Set("cluster", r.cluster)

	req, err := http.NewRequest(http.MethodGet, r.url+"/configs?"+queryParams.Encode(), http.NoBody)
	if err != nil {
		r.logger.Infof("Skipping refresh configs due to the following error:", err)
		return
	}

	for {
		resp, err := client.Do(req)
		if err != nil {
			r.logger.Error(err)
			time.Sleep(time.Duration(r.frequency) * time.Second)

			continue
		}

		data, _ := io.ReadAll(resp.Body)

		var refreshedConfigs map[string]string

		refreshedConfigs, err = r.getRemoteConfigs(data)
		if err != nil {
			time.Sleep(time.Duration(r.frequency) * time.Second)
			continue
		}

		r.remoteConfig = refreshedConfigs

		resp.Body.Close()
		time.Sleep(time.Duration(r.frequency) * time.Second)
	}
}

func (r *RemoteConfig) getRemoteConfigs(body []byte) (map[string]string, error) {
	type data struct {
		ServiceName string            `json:"serviceName"`
		Config      map[string]string `json:"config"`
		UserGroup   string            `json:"userGroup"`
	}

	cfg := struct {
		Data []data `json:"data"`
	}{}

	err := json.Unmarshal(body, &cfg)
	if err != nil {
		r.logger.Infof("Unable to unmarshal %v", err)
		return r.remoteConfig, err
	}

	if len(cfg.Data) == 0 {
		r.logger.Infof("Unable to find config for %s", r.appName)

		return r.remoteConfig, errors.EntityNotFound{Entity: "Remote Config", ID: r.appName}
	}

	res := cfg.Data[0]

	return res.Config, nil
}

func checkConfig(configValue, configKey string, logger logger) {
	if configValue == "" {
		logger.Warnf("%s is not set.", configKey)
	}
}
