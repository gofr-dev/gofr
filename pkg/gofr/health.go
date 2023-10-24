package gofr

import (
	"gofr.dev/pkg"
	"gofr.dev/pkg/gofr/types"
)

// HeartBeatHandler is a handler for giving a http.StatusOK response
func HeartBeatHandler(*Context) (interface{}, error) {
	return types.Raw{Data: map[string]string{"status": "UP"}}, nil
}

type HealthCheck func() types.Health

// HealthHandler reports the database health
func HealthHandler(c *Context) (interface{}, error) {
	var (
		upCount    int
		downCount  int
		healthData healthResp
	)

	for _, v := range c.ServiceHealth {
		svcHealth := v()

		if svcHealth.Status == pkg.StatusUp {
			upCount++
		} else {
			downCount++
		}

		healthData.Details.Services = append(healthData.Details.Services, svcHealth)
	}

	for _, value := range c.DatabaseHealth {
		dbHealth := value()
		if dbHealth.Status == pkg.StatusUp {
			upCount++
		} else {
			downCount++
		}

		healthData.Details.Databases = append(healthData.Details.Databases, dbHealth)
	}

	// getAppDetails
	healthData.Details.App = getAppDetails(c.Config)

	healthData.Status = finalStatus(upCount, downCount)

	healthResp := map[string]interface{}{"details": healthData.Details, "status": healthData.Status}

	return types.Raw{Data: healthResp}, nil
}

type healthResp struct {
	Details healthDetails
	Status  string `json:"status"`
}

type healthDetails struct {
	Databases []types.Health   `json:"databases,omitempty"`
	App       types.AppDetails `json:"app"`
	Services  []types.Health   `json:"services,omitempty"`
}

func getAppDetails(c Config) types.AppDetails {
	var app types.AppDetails

	app.Name = c.GetOrDefault("APP_NAME", pkg.DefaultAppName)
	app.Version = c.GetOrDefault("APP_VERSION", pkg.DefaultAppVersion)
	app.Framework = pkg.Framework

	return app
}

func finalStatus(upCount, downCount int) string {
	switch {
	case upCount == 0 && downCount > 0:
		return pkg.StatusDown
	case upCount > 0 && downCount == 0:
		return pkg.StatusUp
	case upCount > 0 && downCount > 0:
		return pkg.StatusDegraded
	default:
		return pkg.StatusUp
	}
}
