# Configurations

GoFr simplifies configuration management by reading configuration via environment variables.
Application code is decoupled from how configuration is managed as per the {%new-tab-link title="12-factor" href="https://12factor.net/config" %}.
Configs in GoFr can be used to initialise datasources, tracing , setting log levels, changing default http or metrics port.
This abstraction provides a user-friendly interface for configuring the application without modifying the code itself.

To set configs create a `configs` directory in the project's root and add `.env` file.

Follow this directory structure within the GoFr project:
```dotenv
my-gofr-app/
├── config/
│   ├── dev.env
│   ├── staging.env
│   └── prod.env
├── main.go
└── ...
```

By default, GoFr starts HTTP server at port 8000, in order to change that we can add the config `HTTP_PORT`
Similarly to Set the app-name user can add `APP_NAME`. For example:

```bash
# configs/.env

APP_NAME=test-service
HTTP_PORT=9000
```
