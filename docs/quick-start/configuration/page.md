# Configurations

GoFr simplifies configuration management by reading configuration via environment variables.
Application code is decoupled from how configuration is managed as per the {%new-tab-link title="12-factor" href="https://12factor.net/config" %}.
Configs in GoFr can be used to initialize datasources, tracing, setting log levels, changing default HTTP or metrics port.
This abstraction provides a user-friendly interface for configuring user's application without modifying the code itself.

To set configs create a `configs` directory in the project's root and add `.env` file.

Follow this directory structure within the GoFr project:
```dotenv
my-gofr-app/
├── configs/
│   ├── .local.env
│   ├── .dev.env
│   ├── .staging.env
│   └── .prod.env
├── main.go
└── ...
```

By default, GoFr starts HTTP server at port 8000, in order to change that we can add the config `HTTP_PORT`
Similarly to Set the app-name user can add `APP_NAME`. For example:

```dotenv
# configs/.env

APP_NAME=test-service
HTTP_PORT=8001
```

## Configuring Environments in GoFr
GoFr uses an environment variable, `APP_ENV`, to determine the application's current environment. This variable also guides GoFr to load the corresponding environment file.

### Example:
GoFr always loads `configs/.env` first (if present) as the base, then overlays `configs/.<APP_ENV>.env` on top. The overlay file's values override matching keys from `.env`; keys not set in the overlay continue to come from `.env`. If `APP_ENV` is unset, GoFr overlays `configs/.local.env` instead. System environment variables take precedence over both files.

For example, with `APP_ENV=dev` GoFr loads `configs/.env` and then overlays `configs/.dev.env`. Both files are loaded if both exist — the overlay does not replace `.env` wholesale.

_For example, to run the application in the `dev` environment, use the following command:_

```bash
APP_ENV=dev go run main.go
```


This approach ensures that the correct configurations are used for each environment, providing flexibility and control over the application's behavior in different contexts.
