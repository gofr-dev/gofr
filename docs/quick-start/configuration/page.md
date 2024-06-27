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
HTTP_PORT=9000
```

## Configuring Environments in GoFr
GoFr uses an environment variable, `APP_ENV`, to determine the application's current environment. This variable also guides GoFr to load the corresponding environment file.

### Example:
If `APP_ENV` is set to `dev`, GoFr will attempt to load the `.dev.env` file from the configs directory. If this file is not found, GoFr will default to loading the `.env` file.

In the absence of the `APP_ENV` variable, GoFr will first attempt to load the `.local.env` file. If this file is not found, it will default to loading the `.env` file.

_For example, to run the application in the `dev` environment, use the following command:_

```bash
APP_ENV=dev go run main.go
```


This approach ensures that the correct configurations are used for each environment, providing flexibility and control over the application's behavior in different contexts.
