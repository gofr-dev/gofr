# JWT Authentication Example in GoFr

This example demonstrates how to integrate **JWT-based authentication** into a [GoFr](https://gofr.dev) application.

It provides:

- A `/login` endpoint that issues JWT tokens
- A stubbed middleware (`JWTAuth`) for verifying JWTs (commented out until header access is supported)
- A secure route example `/secure` (currently disabled)

---

## 📁 Project Structure

examples/
└── using-jwt-auth/
    ├── handler/
    │   ├── login.go           # Handler to generate JWT token
    │   └── secure.go          # Handler for secure route (reads JWT claims)
    ├── middleware/
    │   └── jwt.go             # JWT authentication middleware (commented out)
    ├── main.go                # Entry point of the app
    └── README.md              # Setup and usage instructions


---

##  Getting Started

> ⚠️ Requires Go 1.22+ and GoFr CLI installed.

###  Clone and Navigate

```bash
git clone https://github.com/<your-repo>/gofr.git
cd gofr/examples/using-jwt-auth

Set JWT Secret
It’s best to load your JWT secret via an environment variable:

export JWT_SECRET=your-256-bit-secret

In main.go, this secret will fall back to a hardcoded value if the variable is not set — only for local testing.

Run the App-
go run main.go

You should see:
INFO[0000] Starting HTTP server on :8000

Get a JWT Token
You can now hit the login route with a username:
curl http://localhost:8000/login?username=demo

Expected response:
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6..."
}

Secure Route with Middleware ( Disabled)-

We’ve included a JWTAuth middleware in middleware/jwt.go. It is commented out for now because GoFr does not currently support reading request headers within middleware.

Once Header Access is Supported:


In main.go, uncomment:
// "gofr.dev/examples/using-jwt-auth/middleware"
app.GET("/secure", middleware.JWTAuth(secret)(handler.SecureHandler()))



In jwt.go, the middleware will:
/*
1. Read Authorization header:
   "Authorization: Bearer <token>"
2. Parse and verify the JWT token.
3. If valid, attach the claims to the context:
   ctx.Context = context.WithValue(ctx.Context, "user", claims)
*/



Then, secure.go can extract that user info from context:
user := ctx.Context.Value("user")



What This Example Covers
✅ Token generation (/login)

🚧 Secure route structure (/secure) — disabled until middleware is active

🛡️ Fallback handling for missing secrets

🔒 Clean separation of handler and middleware logic




Security Best Practices-
-Never hardcode secrets in real-world apps. Use environment variables or secure vaults.
-Always use HTTPS in production.
-Use short-lived access tokens with optional refresh flows.
-Scope tokens and validate claims.


Replace:
secret := "your-256-bit-secret"
With:
secret := os.Getenv("JWT_SECRET")
And make sure to set the environment variable during deployment.



Current Limitation
GoFr (as of now) does not support reading request headers in middleware.
Once it does, this example can be fully functional by:
Enabling the middleware
Removing comments
Validating tokens on /secure

