# Configurations

GoFr reads configuration via environment variables. It provides an easy way to manage this. Application code is decoupled from how configuration is managed as per the {% new-tab-link title="12-factor" href="https://12factor.net/config" /%}.
Configs in GoFr can be used to initialise datasources, tracing. In doing so it abstract the logic and gives an easy interface to setup different things.

To set configs create a `configs` directory in the project's root and add `.env` file.

By default, GoFr starts HTTP server at port 8000, in order to change that we can add the config `HTTP_PORT`
Similarly to Set the app-name you can add `APP_NAME`. For example:

```dotenv
# configs/.env

APP_NAME=test-service
HTTP_PORT=9000
```

