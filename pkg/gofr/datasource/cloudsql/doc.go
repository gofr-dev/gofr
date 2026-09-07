package cloudsql

// Developer guide: adding support for another cloud provider's managed SQL.
//
// This package is the reference implementation of a GoFr "managed SQL" provider.
// AWS RDS/Aurora and Azure Database can be added later WITHOUT any change to GoFr
// core, by following the same contract this module follows. Each provider lives in
// its OWN published leaf module (gofr.dev/pkg/gofr/datasource/<provider>) so that
// provider's cloud SDK is only pulled into applications that use it — GoFr core
// never depends on any cloud SDK, and crucially neither does the provider module:
// it imports only database/sql/driver and its own cloud SDK, never gofr.dev.
//
// The contract is small. The provider exposes a type with a single method:
//
//	Connect() (driver.Connector, func() error, error)
//
// returning (1) a database/sql driver.Connector that authenticates to the managed
// database, (2) a cleanup run on Close to tear down anything database/sql does not
// own (e.g. a background token/credential refresher), and (3) an error. Returning a
// nil connector with a nil error means "no managed auth requested" — App.AddSQLDB
// then keeps GoFr's standard env-configured username/password connection. That is
// what lets the SAME app.AddSQLDB(provider.New(app.Config)) call work locally and in
// the cloud with only configuration changing; the developer never branches.
//
// GoFr core does the rest: App.AddSQLDB hands the connector to
// gofrSQL.NewSQLFromConnector, which opens it (with tracing) and wraps it in GoFr's
// standard SQL datasource — so logging, metrics, health checks, transactions and the
// background retry/metrics goroutines are reused and never duplicated, and the GCP
// (or AWS/Azure) SDK stays entirely inside the provider module.
//
// The only provider-specific part is building the driver.Connector:
//
//   - GCP Cloud SQL (this module): cloudsqlconn.NewDialer opens a secure tunnel and
//     mints IAM tokens transparently, refreshing them in the background. The
//     connector routes the driver's dial through that dialer — pgx via
//     stdlib.GetConnector with a custom DialFunc; MySQL via mysql.NewConnector with
//     a registered dial context. cleanup is the dialer's Close.
//
//   - AWS RDS / Aurora IAM auth (future module, e.g. .../rdsiam): generate a
//     short-lived (~15 min) token with
//     github.com/aws/aws-sdk-go-v2/feature/rds/auth.BuildAuthToken(ctx, endpoint,
//     region, dbUser, creds) and use it as the connection PASSWORD over TLS (the RDS
//     CA bundle). Because the token EXPIRES, implement a driver.Connector whose
//     Connect mints a fresh token per physical connection.
//
//   - Azure Database for PostgreSQL/MySQL with Microsoft Entra ID (future module,
//     e.g. .../azuread): acquire an access token with
//     github.com/Azure/azure-sdk-for-go/sdk/azidentity (token request scope
//     "https://ossrdbms-aad.database.windows.net/.default") and use it as the
//     PASSWORD over TLS. The token lasts ~1 h; use the same refreshing
//     driver.Connector approach as AWS so new and reconnected pool connections
//     always receive a valid token.
//
// Key point: gofrSQL.NewSQLFromConnector accepts any driver.Connector, so the
// authenticating/refreshing connector lives entirely inside the provider module.
// Adding a provider is a new leaf module only — no change to GoFr core and no change
// to this module.
