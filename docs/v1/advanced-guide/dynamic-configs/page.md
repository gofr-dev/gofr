# Dynamic Configuration

GoFr offers a dynamic log level solution that allows you to change it without the need for a server restart. This feature offers runtime overrides, centralized management, and real-time updates. With runtime overrides, you can modify settings on the fly, centralize configuration data for easy management, and see real-time updates applied within seconds, providing flexibility, efficient management, and seamless adaptation.

## Usage

You need to have remote config server running with the following endpoint and response format.

**Endpoint** : /configs

**Sample Response**

```json
{
    "data": [
        {
            "id": "00b720c9-2c5d-11ee-b867-3ad926eecd16",
            "serviceName": "sample-service",
            "config": {
                "LOG_LEVEL": "WARN"
            },
            "userGroup": "gofr-example",
            "namespace": "gofr",
            "cluster": "gofr-cluster"
        }
}
```

You need to set the following configs in your `.env` file to enable remote configs.

```bash
# mandatory field
REMOTE_CONFIG_URL=
APP_NAME=
# optional fields
# these are required when multiple apps with same name are registered
REMOTE_NAMESPACE=
REMOTE_CLUSTER=
REMOTE_USER_GROUP=
```

The `REMOTE_CONFIG_URL` is essential for managing remote configurations. It allows the application to fetch configuration values from a remote source, potentially overwriting local settings. Here's the sequence for the Runtime Override Service (ROS) during app startup:

1. **Read Local Configs:** The app reads local configurations.
2. **Fetch Remote Configs:** If `REMOTE_CONFIG_URL` is provided, it retrieves configurations from the remote source.
3. **Automatic Refresh:** Remote configs are refreshed every 30 seconds.

Remote Configs are prioritized over values from the `.env` file.
