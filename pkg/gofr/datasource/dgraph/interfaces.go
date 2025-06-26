package dgraph

import (
	"context"

	"github.com/dgraph-io/dgo/v210"
	"github.com/dgraph-io/dgo/v210/protos/api"
)

// Txn is an interface for Dgraph transactions.
type Txn interface {
	// BestEffort sets the transaction to best-effort mode.
	BestEffort() Txn

	// Query executes a query against the transaction.
	Query(ctx context.Context, q string) (*api.Response, error)

	// QueryRDF executes an RDF query against the transaction.
	QueryRDF(ctx context.Context, q string) (*api.Response, error)

	// QueryWithVars executes a query with variables against the transaction.
	QueryWithVars(ctx context.Context, q string, vars map[string]string) (*api.Response, error)

	// QueryRDFWithVars executes an RDF query with variables against the transaction.
	QueryRDFWithVars(ctx context.Context, q string, vars map[string]string) (*api.Response, error)

	// Mutate applies a mutation to the transaction.
	Mutate(ctx context.Context, mu *api.Mutation) (*api.Response, error)

	// Do performs a raw request against the transaction.
	Do(ctx context.Context, req *api.Request) (*api.Response, error)

	// Commit commits the transaction.
	Commit(ctx context.Context) error

	// Discard discards the transaction.
	Discard(ctx context.Context) error
}

// DgraphClient is an interface that defines the methods for interacting with Dgraph.
//
//nolint:revive // dgraph.DgraphClient is repetitive. A better name could have been chosen, but it's too late as it's already exported.
type DgraphClient interface {
	// NewTxn creates a new transaction (read-write) for interacting with the Dgraph database.
	NewTxn() Txn

	// NewReadOnlyTxn creates a new read-only transaction for querying the Dgraph database.
	NewReadOnlyTxn() Txn

	// Alter applies schema or other changes to the Dgraph database.
	Alter(ctx context.Context, op *api.Operation) error

	// Login logs in to the Dgraph database.
	Login(ctx context.Context, userid string, password string) error

	// LoginIntoNamespace logs in to the Dgraph database with a specific namespace.
	LoginIntoNamespace(ctx context.Context, userid string, password string, namespace uint64) error

	// GetJwt returns the JWT token for the Dgraph client.
	GetJwt() api.Jwt

	// Relogin relogs in to the Dgraph database.
	Relogin(ctx context.Context) error
}

// dgraphClientImpl is a struct that implements the DgraphClient interface.
type dgraphClientImpl struct {
	client *dgo.Dgraph
}

// NewDgraphClient returns a new Dgraph client.
func NewDgraphClient(client *dgo.Dgraph) DgraphClient {
	return &dgraphClientImpl{client: client}
}

// NewTxn creates a new transaction (read-write) for interacting with the Dgraph database.
func (d *dgraphClientImpl) NewTxn() Txn {
	return &txnImpl{d.client.NewTxn()}
}

// NewReadOnlyTxn creates a new read-only transaction for querying the Dgraph database.
func (d *dgraphClientImpl) NewReadOnlyTxn() Txn {
	return &txnImpl{d.client.NewReadOnlyTxn()}
}

// Alter applies schema or other changes to the Dgraph database.
func (d *dgraphClientImpl) Alter(ctx context.Context, op *api.Operation) error {
	return d.client.Alter(ctx, op)
}

// Login logs in to the Dgraph database.
func (d *dgraphClientImpl) Login(ctx context.Context, userid, password string) error {
	return d.client.Login(ctx, userid, password)
}

// LoginIntoNamespace logs in to the Dgraph database with a specific namespace.
func (d *dgraphClientImpl) LoginIntoNamespace(ctx context.Context, userid, password string, namespace uint64) error {
	return d.client.LoginIntoNamespace(ctx, userid, password, namespace)
}

// GetJwt returns the JWT token for the Dgraph client.
func (d *dgraphClientImpl) GetJwt() api.Jwt {
	return d.client.GetJwt()
}

// Relogin relogs in to the Dgraph database.
func (d *dgraphClientImpl) Relogin(ctx context.Context) error {
	return d.client.Relogin(ctx)
}

// txnImpl is the struct that implements the Txn interface by wrapping *dgo.Txn.
type txnImpl struct {
	txn *dgo.Txn
}

// BestEffort sets the transaction to best-effort mode.
func (t *txnImpl) BestEffort() Txn {
	t.txn.BestEffort()
	return t
}

// Query executes a query against the transaction.
func (t *txnImpl) Query(ctx context.Context, q string) (*api.Response, error) {
	return t.txn.Query(ctx, q)
}

// QueryRDF executes an RDF query against the transaction.
func (t *txnImpl) QueryRDF(ctx context.Context, q string) (*api.Response, error) {
	return t.txn.QueryRDF(ctx, q)
}

// QueryWithVars executes a query with variables against the transaction.
func (t *txnImpl) QueryWithVars(ctx context.Context, q string, vars map[string]string) (*api.Response, error) {
	return t.txn.QueryWithVars(ctx, q, vars)
}

// QueryRDFWithVars executes an RDF query with variables against the transaction.
func (t *txnImpl) QueryRDFWithVars(ctx context.Context, q string, vars map[string]string) (*api.Response, error) {
	return t.txn.QueryRDFWithVars(ctx, q, vars)
}

// Mutate applies a mutation to the transaction.
func (t *txnImpl) Mutate(ctx context.Context, mu *api.Mutation) (*api.Response, error) {
	return t.txn.Mutate(ctx, mu)
}

// Do performs a raw request against the transaction.
func (t *txnImpl) Do(ctx context.Context, req *api.Request) (*api.Response, error) {
	return t.txn.Do(ctx, req)
}

// Commit commits the transaction.
func (t *txnImpl) Commit(ctx context.Context) error {
	return t.txn.Commit(ctx)
}

// Discard discards the transaction.
func (t *txnImpl) Discard(ctx context.Context) error {
	return t.txn.Discard(ctx)
}
