package cloudsql

// Developer guide: adding support for another cloud provider's managed SQL.
//
// This package is the reference implementation of a GoFr "managed SQL" provider.
// AWS RDS/Aurora and Azure Database can be added later WITHOUT any change to GoFr
// core, by following the same contract this module follows. Each provider lives in
// its OWN published leaf module (gofr.dev/pkg/gofr/datasource/<provider>) so that
// provider's cloud SDK is only pulled into applications that use it — the GoFr
// core never depends on any cloud SDK.
//
// The contract is three steps:
//
//  1. Define a datasource type that embeds *gofrSQL.DB and implements UseLogger,
//     UseMetrics, Connect and Close — the hooks App.AddSQLDB calls. Embedding
//     *gofrSQL.DB promotes the whole container.DB interface, so it is used like any
//     other GoFr SQL connection. Keep the type unexported and return container.DB
//     from the constructor (see New), so the module's public surface stays minimal.
//
//  2. In Connect, when cloud auth is NOT requested, delegate to gofrSQL.NewSQL —
//     the standard username/password datasource. This is what lets the SAME
//     AddSQLDB call work locally and in the cloud with only configuration changing;
//     the developer never branches.
//
//  3. When cloud auth IS requested, obtain an authenticated *database/sql.DB and
//     wrap it with gofrSQL.NewSQLFromDB. That single seam is provider-agnostic, so
//     all SQL behavior (logging, metrics, health, transactions, retry) is reused
//     and never duplicated.
//
// The only provider-specific part is step 3 — "produce an authenticated *sql.DB":
//
//   - GCP Cloud SQL (this module): the Cloud SQL Go Connector registers a
//     driver/dialer that opens a secure tunnel and mints IAM tokens transparently,
//     refreshing them in the background. A static driver registration plus sql.Open
//     is sufficient.
//
//   - AWS RDS / Aurora IAM auth (future module, e.g. .../rdsiam): generate a
//     short-lived (~15 min) token with
//     github.com/aws/aws-sdk-go-v2/feature/rds/auth.BuildAuthToken(ctx, endpoint,
//     region, dbUser, creds) and use it as the connection PASSWORD over TLS (the
//     RDS CA bundle). Because the token EXPIRES, do not bake it into a static DSN —
//     implement a driver.Connector whose Connect mints a fresh token per physical
//     connection, then db := sql.OpenDB(connector) and wrap with NewSQLFromDB.
//
//   - Azure Database for PostgreSQL/MySQL with Microsoft Entra ID (future module,
//     e.g. .../azuread): acquire an access token with
//     github.com/Azure/azure-sdk-for-go/sdk/azidentity (token request scope
//     "https://ossrdbms-aad.database.windows.net/.default") and use it as the
//     PASSWORD over TLS. The token lasts ~1 h; use the same refreshing
//     driver.Connector approach as AWS so new and reconnected pool connections
//     always receive a valid token.
//
// Key point: gofrSQL.NewSQLFromDB accepts an already-open *sql.DB, so the
// refreshing connector (AWS/Azure) or the connector-backed driver (GCP) lives
// entirely inside the provider module. Adding a provider is a new leaf module
// only — no change to GoFr core and no change to this module.
