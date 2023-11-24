# Configurations

The recommended way of managing configuration is via environment variables. However, there are some cases where the value of a particular variable needs to be changed across multiple stages.

GoFr provides an easy way to manage this. Application code is decoupled from how configuration is managed as per the [12-factor](https://12factor.net/config). The application will work as if all the variables in a `.env` file are environment variables.

## Managing different environments

The configuration can be adjusted by adding different files in the same `configs` folder.
For example, to have different values for test environment, `.test.env` can be created inside the `configs` folder. Please note that a `.env` file will always be read.
Thus, `.env` file can be used to provide default values and `.test.env` can be used to modify specific keys.

`GOFR_ENV` is a system environment variable which needs to be set in order for overriding values. So, in the test environment `GOFR_ENV` needs to be set as 'test' and only then '.test.env' be considered. If `GOFR_ENV` is not set, `env` is assumed to be 'local'. So, developers can create a file '.local.env' in their systems to manage their workstation specific configurations.

The environment variables in the system take the highest priority. Then the environment variables from the local `.env` file is loaded. If the former is not present, the variables from the `.env` file in the `configs` folder are loaded. If both are present, only the local one is read.

**Good To Know**

```bash
Environment names are not enumerations. So, it can be set to any suitable name and any
number of configurations can be added to the application.
```

## List of Supported Configurations

### Application

{% table %}

- Configuration
- Description
- Default Value

---

- **APP_NAME**
- This is the name of the application. This is used in logging, tracing etc. Keeping it unique across multiple projects will help identify the application related logs and traces.
- gofr

---

- **APP_VERSION**
- This is the version of the application and it is used in logging.
- dev

---

- **LOG_LEVEL**
- It will change application's log level
- INFO

---

{% /table %}

### Server

{% table %}

- Configuration
- Description
- Default Value

---

- **HTTP_PORT**
- This is the port on which http server will run.
- 8000

---

- **HTTP_TO_HTTPS**
- If you set it to true, all http requests to the server will be redirected to https.
- false

---

- **HTTPS_PORT**
- This is the port on which the https server will run.
- 443

---

- **KEY_FILE**
- Location of the Private key file for the https server.
- -

---

- **CERTIFICATE_FILE**
- Location of the public certificate file for the https server. The certificate - represents the server’s identity and contains the server’s public key.
- -

---

- **GRPC_PORT**
- The port at which GRPC will run.
- 50051

---

- **METRICS_PORT**
- It creates a metric server by default. It can also be used for profiling.
- 2121

---

{% /table %}

### Middleware

**OAuth**

| Config Name       | Description                                                          |
| ----------------- | -------------------------------------------------------------------- |
| **JWKS_ENDPOINT** | Specifies the JSON Web Key Set (JWKS) endpoint for OAuth middleware. |

**CORS**

| Config Name                      | Description                                  |
| -------------------------------- | -------------------------------------------- |
| **Access-Control-Allow-Headers** | Defines the allowed HTTP headers for CORS.   |
| **Access-Control-Allow-Methods** | Specifies the allowed HTTP methods for CORS. |
| **Access-Control-Allow-Origin**  | Sets the origin that is allowed for CORS.    |
| **Access-Control-Allow-Credentials**  | Sets the allowed credentials for CORS.    |
| **Access-Control-Expose-Headers**  | Sets the exposed headers for CORS.    |
| **Access-Control-Max-Age**  | Sets the max age for CORS.    |


**NewRelic**

| Config Name          | Description                                         |
| -------------------- | --------------------------------------------------- |
| **APP_NAME**         | Sets the name of the application for New Relic.     |
| **NEWRELIC_LICENSE** | Specifies the license key for New Relic monitoring. |

**Tracer**

| Config Name         | Description                                                          |
| ------------------- | -------------------------------------------------------------------- |
| **TRACER_EXPORTER** | Enables and configures the tracer exporter.                          |
| **TRACER_NAME**     | Sets the name of the tracer.                                         |
| **TRACER_PORT**     | Specifies the port for the tracer.                                   |
| **GCP_PROJECT_ID**  | Specifies the Google Cloud Platform (GCP) project ID for the tracer. |

### Datastore

**SQL**

| Config Name                | Description                                                                   |
| -------------------------- | ----------------------------------------------------------------------------- |
| **DB_HOST**                | Host name of the database server.                                             |
| **DB_USER**                | User name for connecting to the database.                                     |
| **DB_PASSWORD**            | Password for the database user.                                               |
| **DB_NAME**                | Name of the database you want to connect to.                                  |
| **DB_PORT**                | Port on which the database server is running.                                 |
| **DB_DIALECT**             | Specifies the particular type of SQL database (e.g., mysql, mssql, postgres). |
| **DB_SSL**                 | Set to true to enable SSL encryption.                                         |
| **DB_KEY_FILE**            | Path to the key file for SSL encryption (if applicable).                      |
| **DB_CERTIFICATE_FILE**    | Path to the certificate file for SSL encryption (if applicable).              |
| **DB_CA_CERTIFICATE_FILE** | Path to the CA certificate file for SSL encryption (if applicable).           |

For CockroachDB, Yugabyte DB_DIALECT will remain as postgres.

**MongoDB**

| Config Name               | Description                                                        |
| ------------------------- | ------------------------------------------------------------------ |
| **MONGO_DB_HOST**         | Hostname or IP address of the MongoDB server.                      |
| **MONGO_DB_PORT**         | Port number where MongoDB is running (default is 27017).           |
| **MONGO_DB_USER**         | Username for authenticating with the MongoDB server.               |
| **MONGO_DB_PASS**         | Password for authenticating with the MongoDB server.               |
| **MONGO_DB_NAME**         | Name of the MongoDB database to connect to.                        |
| **MONGO_DB_ENABLE_SSL**   | "true" to enable SSL/TLS encryption, "false" to disable it.        |
| **MONGO_DB_RETRY_WRITES** | "true" to enable retryable writes, "false" to disable.             |
| **MONGO_CONN_RETRY**      | Number of connection retry attempts in case of a connection error. |

**Cassandra**

| Config Name                       | Description                                                                                       |
| --------------------------------- | ------------------------------------------------------------------------------------------------- |
| **CASS_DB_KEYSPACE**              | Specifies the keyspace (database) name for Cassandra.                                             |
| **CASS_DB_CONSISTENCY**           | Specifies the consistency level for Cassandra queries.                                            |
| **CASS_DB_HOST**                  | Specifies the hostname or IP address of the Cassandra server.                                     |
| **CASS_DB_PORT**                  | Specifies the port number for Cassandra connections.                                              |
| **CASS_DB_USER**                  | Specifies the username for authenticating with Cassandra.                                         |
| **CASS_DB_PASS**                  | Specifies the password for authenticating with Cassandra.                                         |
| **CASS_DB_TIMEOUT**               | Specifies the timeout for Cassandra queries.                                                      |
| **CASS_DB_CONN_TIMEOUT**          | Specifies the connection timeout for Cassandra in milliseconds.                                   |
| **CASS_CONN_RETRY**               | Specifies whether to enable connection retry for Cassandra.                                       |
| **CASS_DB_CERTIFICATE_FILE**      | Path to the client certificate file.                                                              |
| **CASS_DB_KEY_FILE**              | Path to the client private key file.                                                              |
| **CASS_DB_ROOT_CERTIFICATE_FILE** | Path to the root certificate file (trusted CA certificates).                                      |
| **CASS_DB_INSECURE_SKIP_VERIFY**  | Set to "true" to skip verification of server certificates (not recommended for production).       |
| **CASS_DB_HOST_VERIFICATION**     | Set to "skip" to skip hostname verification. Set to "full" to perform full hostname verification. |

**DyanmoDB**

| Config Name                    | Description                                                                          |
| ------------------------------ | ------------------------------------------------------------------------------------ |
| **DYNAMODB_REGION**            | The AWS region where your DynamoDB instance is located.                              |
| **DYNAMODB_ACCESS_KEY_ID**     | Your AWS access key ID for authentication.                                           |
| **DYNAMODB_SECRET_ACCESS_KEY** | Your AWS secret access key for authentication.                                       |
| **DYNAMODB_ENDPOINT_URL**      | Specify the endpoint URL for a local DynamoDB instance, or omit it for AWS DynamoDB. |

**Elastic Search**

| Config Name             | Description                                         |
| ----------------------- | --------------------------------------------------- |
| **ELASTIC_SEARCH_HOST** | Hostname or IP address of your Elasticsearch server |
| **ELASTIC_SEARCH_PORT** | Port number for Elasticsearch (usually 9200).       |
| **ELASTIC_SEARCH_USER** | Username for authentication.                        |
| **ELASTIC_SEARCH_PASS** | Password for authentication.                        |

**Redis**

| Config Name          | Description                                          |
| -------------------- | ---------------------------------------------------- |
| **REDIS_HOST**       | Hostname for the Redis server.                       |
| **REDIS_PASSWORD**   | Password for the Redis server.                       |
| **REDIS_PORT**       | Port on which the server is running (Default: 6379). |
| **REDIS_SSL**        | Enable SSL/TLS Authentication for Redis.             |
| **REDIS_CONN_RETRY** | Connection retry interval for Redis in seconds.      |

**Solr**

| Config Name   | Description                |
| ------------- | -------------------------- |
| **SOLR_HOST** | Host name or IP address    |
| **SOLR_PORT** | Port number for connection |

### File store

#### SFTP

| Configuration     | Description                                                                            |
| ----------------- | -------------------------------------------------------------------------------------- |
| **FILE_STORE**    | SFTP file storage. Utilizes the Secure File Transfer Protocol for remote file storage. |
| **SFTP_HOST**     | SFTP host address. The address of the SFTP server.                                     |
| **SFTP_USER**     | SFTP username. The username for SFTP authentication.                                   |
| **SFTP_PASSWORD** | SFTP password. The password for SFTP authentication.                                   |
| **SFTP_PORT**     | SFTP port number. The port used for SFTP connections.                                  |

#### FTP

| Configuration    | Description                                                                      |
| ---------------- | -------------------------------------------------------------------------------- |
| **FILE_STORE**   | FTP file storage. Implements the File Transfer Protocol for remote file storage. |
| **FTP_HOST**     | FTP host address. The address of the FTP server.                                 |
| **FTP_USER**     | FTP username. The username for FTP authentication.                               |
| **FTP_PASSWORD** | FTP password. The password for FTP authentication.                               |
| **FTP_PORT**     | FTP port number. The port used for FTP connections.                              |

#### AWS

| Configuration              | Description                                                                            |
| -------------------------- | -------------------------------------------------------------------------------------- |
| **FILE_STORE**             | AWS file storage. Utilizes Amazon Web Services for scalable and reliable file storage. |
| **AWS_STORAGE_ACCESS_KEY** | AWS storage access key. The access key for AWS storage.                                |
| **AWS_STORAGE_SECRET_KEY** | AWS storage secret key. The secret key for AWS storage.                                |
| **AWS_STORAGE_TOKEN**      | AWS storage security token. A security token for AWS storage, if applicable.           |
| **AWS_STORAGE_BUCKET**     | AWS storage bucket. The name of the AWS storage bucket.                                |
| **AWS_STORAGE_REGION**     | AWS storage region. The region for AWS storage, e.g., us-east-1.                       |

#### GCP

| Configuration  | Description                                                                            |
| -------------- | -------------------------------------------------------------------------------------- |
| **FILE_STORE** | GCP file storage. Utilizes Google Cloud Platform for secure and scalable file storage. |

#### AZURE

| Configuration                 | Description                                                                                      |
| ----------------------------- | ------------------------------------------------------------------------------------------------ |
| **FILE_STORE**                | Azure file storage. Leverages Microsoft Azure for cloud-based file storage.                      |
| **AZURE_STORAGE_ACCOUNT**     | Azure storage account. The Azure storage account name.                                           |
| **AZURE_STORAGE_ACCESS_KEY**  | Azure storage access key. The access key for Azure storage.                                      |
| **AZURE_STORAGE_CONTAINER**   | Azure storage container. The name of the Azure storage container.                                |
| **AZURE_STORAGE_BLOCK_SIZE**  | Azure storage block size. The block size for Azure storage, if applicable.                       |
| **AZURE_STORAGE_PARALLELISM** | Azure storage parallelism. The level of parallelism for Azure storage operations, if applicable. |

These updated descriptions remove redundancy and provide clear information about each configuration.

### Pubsub

Certainly, here are the configurations for the PubSub Backend presented in a cleaner format:

#### KAFKA

| Configuration   | Description                     |
| --------------- | ------------------------------- |
| **KAFKA_HOSTS** | The addresses of Kafka brokers. |
| **KAFKA_TOPIC** | The name of the Kafka topic.    |

#### AVRO

| Configuration           | Description                     |
| ----------------------- | ------------------------------- |
| **AVRO_SCHEMA_URL**     | The URL of the Avro schema.     |
| **AVRO_SCHEMA_VERSION** | The version of the Avro schema. |
| **AVRO_SUBJECT**        | The subject of the Avro schema. |

#### EventBridge

| Configuration                     | Description                            |
| --------------------------------- | -------------------------------------- |
| **EVENT_BRIDGE_BUS**              | The name of the EventBridge bus.       |
| **EVENT_BRIDGE_SOURCE**           | The source for EventBridge events.     |
| **EVENT_BRIDGE_REGION**           | The region for EventBridge.            |
| **EVENT_BRIDGE_RETRY_FREQUENCY**  | The frequency of retry attempts.       |
| **EVENTBRIDGE_ACCESS_KEY_ID**     | The access key ID for EventBridge.     |
| **EVENTBRIDGE_SECRET_ACCESS_KEY** | The secret access key for EventBridge. |

#### Google

| Configuration                | Description                                                               |
| ---------------------------- | ------------------------------------------------------------------------- |
| **GOOGLE_TOPIC_NAME**        | The name of the Google Pub/Sub topic.                                     |
| **GOOGLE_PROJECT_ID**        | The ID of the Google Cloud project.                                       |
| **GOOGLE_SUBSCRIPTION_NAME** | The name of the Google Pub/Sub subscription.                              |
| **GOOGLE_TIMEOUT_DURATION**  | The duration for timeouts in Google Pub/Sub.                              |
| **GOOGLE_CONN_RETRY**        | Google Pub/Sub connection retry. Retry settings for Google Pub/Sub.       |
| **PUBSUB_EMULATOR_HOST**     | Google Pub/Sub emulator host. The host for the Pub/Sub emulator, if used. |

#### Eventhub

| Configuration           | Description                                       |
| ----------------------- | ------------------------------------------------- |
| **EVENTHUB_NAMESPACE**  | Event Hub namespace. The namespace for Event Hub. |
| **EVENTHUB_NAME**       | Event Hub name. The name of the Event Hub.        |
| **EVENTHUB_SAS_NAME**   | Event Hub SAS name. The SAS name for Event Hub.   |
| **EVENTHUB_SAS_KEY**    | Event Hub SAS key. The SAS key for Event Hub.     |
| **AZURE_CLIENT_ID**     | Azure client ID. The client ID for Azure.         |
| **AZURE_CLIENT_SECRET** | Azure client secret. The client secret for Azure. |
| **AZURE_TENANT_ID**     | Azure tenant ID. The tenant ID for Azure.         |

### Notifier

| Variable Name             | Description                               |
| ------------------------- | ----------------------------------------- |
| **SNS_ACCESS_KEY**        | Access key for Amazon SNS                 |
| **SNS_SECRET_ACCESS_KEY** | Secret access key for Amazon SNS          |
| **SNS_REGION**            | AWS region where Amazon SNS is configured |
| **SNS_PROTOCOL**          | Protocol for communication                |
| **SNS_ENDPOINT**          | Amazon SNS endpoint URL                   |
| **SNS_TOPIC_ARN**         | ARN of the SNS topic                      |
| **NOTIFIER_BACKEND**      | Choice of notifier backend [SNS]          |
