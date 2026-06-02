package arangodb

import (
	"context"
	"fmt"

	"github.com/arangodb/go-driver/v2/arangodb"
)

func (c *Client) user(ctx context.Context, username string) (arangodb.User, error) {
	return c.client.User(ctx, username)
}

func (c *Client) database(ctx context.Context, name string) (arangodb.Database, error) {
	return c.client.GetDatabase(ctx, name, nil)
}

// createUser creates a new user in ArangoDB.
func (c *Client) createUser(ctx context.Context, username string, options any) error {
	ctx, done := c.instrumentOp(ctx, &QueryLog{Operation: "createUser", ID: username})
	defer done()

	userOptions, ok := options.(UserOptions)
	if !ok {
		return fmt.Errorf("%w", errInvalidUserOptionsType)
	}

	_, err := c.client.CreateUser(ctx, username, userOptions.toArangoUserOptions())
	if err != nil {
		return err
	}

	return nil
}

// dropUser deletes a user from ArangoDB.
func (c *Client) dropUser(ctx context.Context, username string) error {
	ctx, done := c.instrumentOp(ctx, &QueryLog{Operation: "dropUser", ID: username})
	defer done()

	err := c.client.RemoveUser(ctx, username)
	if err != nil {
		return err
	}

	return err
}

// grantDB grants permissions for a database to a user.
func (c *Client) grantDB(ctx context.Context, database, username, permission string) error {
	ctx, done := c.instrumentOp(ctx, &QueryLog{Operation: "grantDB", Database: database, ID: username})
	defer done()

	user, err := c.client.User(ctx, username)
	if err != nil {
		return err
	}

	err = user.SetDatabaseAccess(ctx, database, arangodb.Grant(permission))

	return err
}

// grantCollection grants permissions for a collection to a user.
func (c *Client) grantCollection(ctx context.Context, database, collection, username, permission string) error {
	ctx, done := c.instrumentOp(ctx, &QueryLog{Operation: "GrantCollection",
		Database: database, Collection: collection, ID: username})
	defer done()

	user, err := c.client.User(ctx, username)
	if err != nil {
		return err
	}

	err = user.SetCollectionAccess(ctx, database, collection, arangodb.Grant(permission))

	return err
}
