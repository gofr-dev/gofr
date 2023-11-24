# Support For Oauth

GoFr has an inbuilt middlware which can be used to integrate [OAuth](https://www.oauth.com), which can be enable by setting the env variable `JWKS_ENDPOINT`.

After adding this environment varible, every endpoint need Bearer token else it GoFr returns with Status Code `401`.

**Following endpoints are exempted :**

- /metrics
- /.well-known/health-check
- /.well-known/heartbeat
- /.well-known/openapi.json
- /swagger
- /.well-known/swagger

## Accessing Claims

To access the claims of JWT token gofr provides methods from context.

```go
func (c *gofr.Context) (interface{}, error) {
    // gets a specific claim
	claimAZP := c.GetClaim("azp")

    // get all the claim in map[string]string
	oAuthClaims := c.GetClaims()
}
```
