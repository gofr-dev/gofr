package arangodb

import (
	"context"
	"fmt"
	"time"

	"github.com/arangodb/go-driver/v2/arangodb"
)

func (c *Client) user(ctx context.Context, username string) (arangodb.User, error) {
	return c.client.User(ctx, username)
}

func (c *Client) database(ctx context.Context, name string) (arangodb.Database, error) {
	return c.client.Database(ctx, name)
}

func (c *Client) databases(ctx context.Context) ([]arangodb.Database, error) {
	return c.client.Databases(ctx)
}

func (c *Client) version(ctx context.Context) (arangodb.VersionInfo, error) {
	return c.client.Version(ctx)
}

// createUser creates a new user in ArangoDB.
func (c *Client) createUser(ctx context.Context, username string, options any) error {
	tracerCtx, span := c.addTrace(ctx, "createUser", map[string]string{"user": username})
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Operation: "createUser", ID: username},
		startTime, "createUser", span)

	userOptions, ok := options.(UserOptions)
	if !ok {
		return fmt.Errorf("%w", errInvalidUserOptionsType)
	}

	_, err := c.client.CreateUser(tracerCtx, username, userOptions.toArangoUserOptions())
	if err != nil {
		return err
	}

	return nil
}

// dropUser deletes a user from ArangoDB.
func (c *Client) dropUser(ctx context.Context, username string) error {
	tracerCtx, span := c.addTrace(ctx, "dropUser", map[string]string{"user": username})
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Operation: "dropUser",
		ID: username}, startTime, "dropUser", span)

	err := c.client.RemoveUser(tracerCtx, username)
	if err != nil {
		return err
	}

	return err
}

// grantDB grants permissions for a database to a user.
func (c *Client) grantDB(ctx context.Context, database, username, permission string) error {
	tracerCtx, span := c.addTrace(ctx, "grantDB", map[string]string{"DB": database})
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Operation: "grantDB",
		Database: database, ID: username}, startTime, "grantDB", span)

	user, err := c.client.User(tracerCtx, username)
	if err != nil {
		return err
	}

	err = user.SetDatabaseAccess(tracerCtx, database, arangodb.Grant(permission))

	return err
}

// grantCollection grants permissions for a collection to a user.
func (c *Client) grantCollection(ctx context.Context, database, collection, username, permission string) error {
	tracerCtx, span := c.addTrace(ctx, "GrantCollection", map[string]string{"collection": collection})
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Operation: "GrantCollection",
		Database: database, Collection: collection, ID: username}, startTime,
		"GrantCollection", span)

	user, err := c.client.User(tracerCtx, username)
	if err != nil {
		return err
	}

	err = user.SetCollectionAccess(tracerCtx, database, collection, arangodb.Grant(permission))

	return err
}
