package eventhub

import "github.com/Azure/azure-amqp-common-go/v3/aad"

type AzureAadJWTProvider interface {
	NewJWTProvider(c *Config, opts ...aad.JWTProviderOption) (*aad.TokenProvider, error)
}
