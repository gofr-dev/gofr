# 🔐 JWT Authentication Example with RS256 in GoFr

This example demonstrates how to implement **JWT authentication using RSA (RS256)** in a [GoFr](https://gofr.dev) application, including secure routes and JWKS public key exposure.

---

## ✅ Features

- `POST /login` – issues RS256-signed JWT tokens after username/password authentication.
- `GET /secure` – protected route, only accessible with a valid token.
- `GET /oauth2/jwks` – JWKS-compliant endpoint for public key discovery.
- Uses GoFr’s native `app.EnableOAuth()` for token verification via JWKS.
- Environment-based configuration for RSA key paths.

---

## 📁 Project Structure

examples/
└── using-jwt-auth/
├── handler/
│ ├── login.go # Login logic: authenticates user, signs JWT
│ └── secure.go # Protected route that reads JWT claims
├── middleware/
│ └── jwt.go # (Optional) Role-based guard middleware
├── keys/
│ ├── private.pem # RSA private key (🔐 should NOT be committed)
│ └── public.pem # RSA public key (used for JWKS)
├── main.go # Application entry point
├── .env # RSA key paths
├── .gitignore # Hides secrets and keys
└── README.md 


---

## 🚀 Getting Started

> ✅ Requires **Go 1.22+** and **GoFr CLI** installed.

### 1️⃣ Clone and Navigate

```bash
git clone https://github.com/<your-username>/gofr.git
cd gofr/examples/using-jwt-auth

2️⃣ Add RSA Keys
Create your RSA key pair (or use the included ones for demo only):

# Generate private key
openssl genrsa -out keys/private.pem 2048

# Extract public key
openssl rsa -in keys/private.pem -pubout -out keys/public.pem

Then add the paths to your .env:

PRIVATE_KEY_PATH=keys/private.pem
PUBLIC_KEY_PATH=keys/public.pem

3️⃣ Run the App

go run main.go
You should see:

INFO[0000] Starting HTTP server on :8000
🪪 Issue a JWT Token

curl -X POST http://localhost:8000/login \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"password123"}'

Response:
{
  "data": {
    "token": "<JWT_TOKEN>",
    "expires_at": 1723456821,
    "token_type": "Bearer"
  }
}

🔐 Call Secure Route
Use the token from above:

curl http://localhost:8000/secure \
  -H "Authorization: Bearer <JWT_TOKEN>"

Expected response:

{
  "message": "Access granted to protected resource",
  "username": "testuser",
  "access": "This is a protected endpoint. Authenticated as: testuser"
}
🛰️ JWKS Endpoint
Your public key is exposed at:

GET http://localhost:8000/oauth2/jwks

Used by GoFr's app.EnableOAuth() for token verification.

🔐 Security Practices
✅ Use env vars to load secrets
✅ Use RS256 instead of HS256
✅ Use short-lived tokens with claim validation
✅ Enable role-based access using middleware.RequireRole