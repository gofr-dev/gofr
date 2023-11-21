# Swagger

GoFr simplifies API interaction by providing a visual representation of an API's resources, without the need for implementation details. It automatically generates user-friendly documentation from the OpenAPI Specification (formerly known as Swagger), benefiting both developers and end-users. For additional information, visit the [official reference](https://swagger.io/tools/swagger-ui/).

## Configuration

To enable Swagger UI, you need to provide the `openapi.json` file within the `api/` directory located in your project's root directory. This enables two endpoints:

- `.well-known/openapi.json` - provides raw swagger in JSON format
- `.well-known/swagger` - loads the swagger UI

The current implementation of Swagger UI accesses the `.well-known/openapi.json` endpoint for input.
