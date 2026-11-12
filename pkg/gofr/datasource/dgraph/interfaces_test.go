package dgraph

import (
	"context"
	"errors"
	"testing"

	"github.com/dgraph-io/dgo/v210"
	"github.com/dgraph-io/dgo/v210/protos/api"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

var (
	errFakeQuery  = errors.New("fake query failed")
	errFakeAlter  = errors.New("fake alter failed")
	errFakeLogin  = errors.New("fake login failed")
	errFakeCommit = errors.New("fake commit failed")
)

// fakeAPIClient is an in-memory api.DgraphClient that records the requests the dgo
// client sends it and answers with canned responses, so the dgo-backed wrappers can
// be exercised without a Dgraph server.
type fakeAPIClient struct {
	queryResp *api.Response
	queryErr  error
	alterErr  error
	loginResp *api.Response
	loginErr  error
	commitErr error

	lastQuery  *api.Request
	lastAlter  *api.Operation
	lastLogin  *api.LoginRequest
	lastCommit *api.TxnContext
}

func (f *fakeAPIClient) Login(_ context.Context, in *api.LoginRequest, _ ...grpc.CallOption) (*api.Response, error) {
	f.lastLogin = in

	return f.loginResp, f.loginErr
}

func (f *fakeAPIClient) Query(_ context.Context, in *api.Request, _ ...grpc.CallOption) (*api.Response, error) {
	f.lastQuery = in

	return f.queryResp, f.queryErr
}

func (f *fakeAPIClient) Alter(_ context.Context, in *api.Operation, _ ...grpc.CallOption) (*api.Payload, error) {
	f.lastAlter = in

	return &api.Payload{}, f.alterErr
}

func (f *fakeAPIClient) CommitOrAbort(_ context.Context, in *api.TxnContext, _ ...grpc.CallOption) (*api.TxnContext, error) {
	f.lastCommit = in

	return &api.TxnContext{}, f.commitErr
}

func (*fakeAPIClient) CheckVersion(context.Context, *api.Check, ...grpc.CallOption) (*api.Version, error) {
	return &api.Version{}, nil
}

func marshalJwt(t *testing.T, jwt *api.Jwt) []byte {
	t.Helper()

	b, err := jwt.Marshal()
	require.NoError(t, err)

	return b
}

func TestDgraphClientImpl_Txn(t *testing.T) {
	vars := map[string]string{"$name": "gofr"}
	mutation := &api.Mutation{SetJson: []byte(`{"name":"gofr"}`)}
	// dgo attaches the (here empty) gRPC response headers to every response.
	resp := &api.Response{Json: []byte(`{"ok":true}`), Hdrs: map[string]*api.ListOfString{}}

	tests := []struct {
		desc     string
		queryErr error
		call     func(ctx context.Context, c DgraphClient) (*api.Response, error)
		expReq   *api.Request
		expResp  *api.Response
		expErr   error
	}{
		{
			desc:    "query",
			call:    func(ctx context.Context, c DgraphClient) (*api.Response, error) { return c.NewTxn().Query(ctx, "q") },
			expReq:  &api.Request{Query: "q", RespFormat: api.Request_JSON},
			expResp: resp,
		},
		{
			desc:    "query rdf",
			call:    func(ctx context.Context, c DgraphClient) (*api.Response, error) { return c.NewTxn().QueryRDF(ctx, "q") },
			expReq:  &api.Request{Query: "q", RespFormat: api.Request_RDF},
			expResp: resp,
		},
		{
			desc: "query with vars",
			call: func(ctx context.Context, c DgraphClient) (*api.Response, error) {
				return c.NewTxn().QueryWithVars(ctx, "q", vars)
			},
			expReq:  &api.Request{Query: "q", Vars: vars, RespFormat: api.Request_JSON},
			expResp: resp,
		},
		{
			desc: "query rdf with vars",
			call: func(ctx context.Context, c DgraphClient) (*api.Response, error) {
				return c.NewTxn().QueryRDFWithVars(ctx, "q", vars)
			},
			expReq:  &api.Request{Query: "q", Vars: vars, RespFormat: api.Request_RDF},
			expResp: resp,
		},
		{
			desc: "best effort read-only query",
			call: func(ctx context.Context, c DgraphClient) (*api.Response, error) {
				return c.NewReadOnlyTxn().BestEffort().Query(ctx, "q")
			},
			expReq:  &api.Request{Query: "q", ReadOnly: true, BestEffort: true, RespFormat: api.Request_JSON},
			expResp: resp,
		},
		{
			desc: "mutate",
			call: func(ctx context.Context, c DgraphClient) (*api.Response, error) {
				return c.NewTxn().Mutate(ctx, mutation)
			},
			expReq:  &api.Request{Mutations: []*api.Mutation{mutation}},
			expResp: resp,
		},
		{
			desc: "do",
			call: func(ctx context.Context, c DgraphClient) (*api.Response, error) {
				return c.NewTxn().Do(ctx, &api.Request{Query: "raw"})
			},
			expReq:  &api.Request{Query: "raw"},
			expResp: resp,
		},
		{
			desc:     "query error",
			queryErr: errFakeQuery,
			call:     func(ctx context.Context, c DgraphClient) (*api.Response, error) { return c.NewTxn().Query(ctx, "q") },
			expReq:   &api.Request{Query: "q", RespFormat: api.Request_JSON},
			expErr:   errFakeQuery,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			fake := &fakeAPIClient{queryResp: &api.Response{Json: resp.Json}, queryErr: tc.queryErr}
			client := NewDgraphClient(dgo.NewDgraphClient(fake))

			got, err := tc.call(t.Context(), client)

			require.ErrorIs(t, err, tc.expErr)
			require.Equal(t, tc.expResp, got)
			require.Equal(t, tc.expReq, fake.lastQuery)
		})
	}
}

func TestDgraphClientImpl_CommitDiscard(t *testing.T) {
	tests := []struct {
		desc       string
		commitErr  error
		finish     func(ctx context.Context, txn Txn) error
		expAborted bool
		expErr     error
	}{
		{desc: "commit", finish: func(ctx context.Context, txn Txn) error { return txn.Commit(ctx) }},
		{
			desc:      "commit error",
			commitErr: errFakeCommit,
			finish:    func(ctx context.Context, txn Txn) error { return txn.Commit(ctx) },
			expErr:    errFakeCommit,
		},
		{desc: "discard", finish: func(ctx context.Context, txn Txn) error { return txn.Discard(ctx) }, expAborted: true},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			fake := &fakeAPIClient{queryResp: &api.Response{}, commitErr: tc.commitErr}
			txn := NewDgraphClient(dgo.NewDgraphClient(fake)).NewTxn()

			// A mutation marks the transaction dirty so finishing it reaches the server.
			_, err := txn.Mutate(t.Context(), &api.Mutation{SetJson: []byte(`{}`)})
			require.NoError(t, err)

			err = tc.finish(t.Context(), txn)

			require.ErrorIs(t, err, tc.expErr)
			require.NotNil(t, fake.lastCommit)
			require.Equal(t, tc.expAborted, fake.lastCommit.Aborted)
		})
	}
}

func TestDgraphClientImpl_Alter(t *testing.T) {
	op := &api.Operation{Schema: "name: string ."}

	tests := []struct {
		desc     string
		alterErr error
		expErr   error
	}{
		{desc: "success"},
		{desc: "error", alterErr: errFakeAlter, expErr: errFakeAlter},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			fake := &fakeAPIClient{alterErr: tc.alterErr}
			client := NewDgraphClient(dgo.NewDgraphClient(fake))

			err := client.Alter(t.Context(), op)

			require.ErrorIs(t, err, tc.expErr)
			require.Equal(t, op, fake.lastAlter)
		})
	}
}

func TestDgraphClientImpl_Login(t *testing.T) {
	jwt := &api.Jwt{AccessJwt: "access", RefreshJwt: "refresh"}

	tests := []struct {
		desc     string
		loginErr error
		login    func(ctx context.Context, c DgraphClient) error
		expReq   *api.LoginRequest
		expJwt   api.Jwt
		expErr   error
	}{
		{
			desc:   "login",
			login:  func(ctx context.Context, c DgraphClient) error { return c.Login(ctx, "user", "pass") },
			expReq: &api.LoginRequest{Userid: "user", Password: "pass"},
			expJwt: *jwt,
		},
		{
			desc: "login into namespace",
			login: func(ctx context.Context, c DgraphClient) error {
				return c.LoginIntoNamespace(ctx, "user", "pass", 7)
			},
			expReq: &api.LoginRequest{Userid: "user", Password: "pass", Namespace: 7},
			expJwt: *jwt,
		},
		{
			desc: "relogin uses refresh token",
			login: func(ctx context.Context, c DgraphClient) error {
				return errors.Join(c.Login(ctx, "user", "pass"), c.Relogin(ctx))
			},
			expReq: &api.LoginRequest{RefreshToken: "refresh"},
			expJwt: *jwt,
		},
		{
			desc:     "login error",
			loginErr: errFakeLogin,
			login:    func(ctx context.Context, c DgraphClient) error { return c.Login(ctx, "user", "pass") },
			expReq:   &api.LoginRequest{Userid: "user", Password: "pass"},
			expErr:   errFakeLogin,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			fake := &fakeAPIClient{loginResp: &api.Response{Json: marshalJwt(t, jwt)}, loginErr: tc.loginErr}
			client := NewDgraphClient(dgo.NewDgraphClient(fake))

			err := tc.login(t.Context(), client)

			require.ErrorIs(t, err, tc.expErr)
			require.Equal(t, tc.expReq, fake.lastLogin)
			require.Equal(t, tc.expJwt, client.GetJwt())
		})
	}
}
