package log

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

type levelService struct {
	init         bool
	level        level
	url          string
	app          string
	failureCount int
	rwMutex      sync.RWMutex
	namespace    string
	cluster      string
	userGroup    string
}

const LevelFetchInterval = 10 // In seconds

func newLevelService(l Logger, appName string) *levelService {
	rls := &levelService{}

	if !rls.init {
		lvl := getLevel(os.Getenv("LOG_LEVEL"))
		rls.level = lvl
		rls.url = os.Getenv("LOG_SERVICE_URL")
		rls.app = appName
		rls.namespace = os.Getenv("LOG_SERVICE_NAMESPACE")
		rls.cluster = os.Getenv("LOG_SERVICE_CLUSTER")
		rls.userGroup = os.Getenv("LOG_SERVICE_USER_GROUP")

		if rls.url != "" {
			rls.init = true
			rlsInit = true

			queryParams := url.Values{}
			// Add the parameters to the map
			queryParams.Set("serviceName", rls.app)
			queryParams.Set("namespace", rls.namespace)
			queryParams.Set("userGroup", rls.userGroup)
			queryParams.Set("cluster", rls.cluster)

			req, _ := http.NewRequest(http.MethodGet, rls.url+"/configs?"+queryParams.Encode(), http.NoBody)

			//nolint:gosec // need this to skip TLS verification
			tr := &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
			client := &http.Client{Transport: tr}

			go func(c *http.Client, r *http.Request) {
				for {
					rls.updateRemoteLevel(c, r, l)
					time.Sleep(LevelFetchInterval * time.Second)
				}
			}(client, req)
		}
	}

	return rls
}

func (s *levelService) updateRemoteLevel(client *http.Client, req *http.Request, l Logger) {
	s.rwMutex.Lock()
	defer s.rwMutex.Unlock()
	l.Debugf("Making request to remote logging service %s", s.url)

	resp, err := client.Do(req)
	if err != nil {
		l.Warnf("Could not create log service client. err:%v", err)
		s.failureCount++

		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		l.Warnf("Logging Service returned %d status. Req: %s", resp.StatusCode, req.URL)

		return
	}

	b, _ := io.ReadAll(resp.Body)
	if newLevel := s.getRemoteLevel(b, l); s.level != newLevel {
		l.Debugf("Changing log level from %s to %s because of remote log service", s.level, newLevel)

		s.level = newLevel
	}
}
func (s *levelService) getRemoteLevel(body []byte, l Logger) level {
	type data struct {
		ServiceName string            `json:"serviceName"`
		Config      map[string]string `json:"config"`
		UserGroup   string            `json:"userGroup"`
	}

	level := struct {
		Data []data `json:"data"`
	}{}

	err := json.Unmarshal(body, &level)
	if err != nil {
		l.Warnf("Logging Service returned %v", err)
	}

	if len(level.Data) > 0 {
		logLevel := level.Data[0].Config["LOG_LEVEL"]
		newLevel := getLevel(logLevel)

		return newLevel
	}

	return s.level
}
