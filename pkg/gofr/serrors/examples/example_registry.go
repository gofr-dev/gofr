package examples

import (
	"fmt"
	"gofr.dev/pkg/gofr/serrors"
	"net/http"
)

const EntityNotFound = "EntityNotFound"

var registry = map[string]serrors.Registry{
	EntityNotFound: {
		InternalMessage: "No entity found",
		ExternalStatus:  http.StatusNotFound,
		ExternalMessage: "Requested resource was not found",
		Level:           serrors.INFO,
		SubStatusCode:   "RESOURCE_MISSING",
	},
	"db_conn_failed": {
		SubStatusCode:   "5001",
		InternalMessage: "Database connection failed",
		Retryable:       true,
	},
	"invalid_token": {
		SubStatusCode:   "4011",
		InternalMessage: "Authentication token invalid",
		Retryable:       false,
	},
}

func ReadRegistry() {
	for name, _ := range registry {
		err := serrors.NewFromRegistry(nil, name, registry)
		fmt.Println(serrors.GetInternalError(err, true))
		fmt.Println(serrors.GetExternalError(err))
	}
}
