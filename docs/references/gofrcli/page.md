# **GoFR Command Line Interface**

Managing repetitive tasks, ensuring consistency across projects, and maintaining efficiency in a development workflow can be challenging, especially when working on large-scale applications. Tasks like initializing projects, handling database migrations, or generating boilerplate code can become tedious and repetitive

GoFr CLI addresses these challenges by providing a streamlined, all-in-one command-line tool specifically designed for GoFr applications. It simplifies tasks such as creating and managing database migrations, generating gRPC wrappers, and initializing new projects with standard GoFr conventions.

For example, instead of manually setting up a new project structure, GoFr CLI can instantly scaffold a project with best practices in place. Similarly, managing database schema changes becomes effortless with its migration subcommands, ensuring that your database remains synchronized across environments.

---

# **Prerequisite**
- Go 1.21 or above. To check Go version use the following command
```bash
  go version.
````

# **Installation**
To get started with GoFr CLI, use the below commands

```bash
  go install gofr.dev/cli/gofr@latest
```

To check the installation:
```bash
  gofr version
```
---

## Usage

The CLI can be run directly from the terminal after installation. Here’s the general syntax:

```go
gofr <subcommand> [flags]=[arguments]
```
---

# **Commands**

1. **`init`** 

   The init command initializes a new GoFr project. The init command sets up the foundational structure for a new GoFr project.
   It generates a simple "Hello World!" program as a starting point, allowing developers to quickly dive into building their application.

### Command Usage 
```go
gofr init
```
---

2. **`migrate create`**

   The migrate create command generates a new migration file and organizes your database schema changes in a migrations directory.
   It ensures consistent structure for migrations, simplifying schema updates across environments.

### Command Usage
```go
gofr migrate create -name=<migration-name>
```
### Example
```go
gofr migrate create -name=add_user_table
```
This will create a migration file named add_users_table in the migrations directory.
For detailed instructions on handling database migrations, see the [handling-data-migrations documentation](../../advanced-guide/handling-data-migrations/page.md)

---

3. ***`wrap grpc`***

   The gofr wrap grpc command streamlines gRPC integration in a GoFr project by generating context-aware structures.
   It simplifies accessing datasources, adding tracing, and setting up gRPC handlers with minimal configuration, allowing you to focus on implementing application logic.
   For detailed instructions on using grpc with GoFr see the [gRPC documentation](../../advanced-guide/grpc/page.md)

### Command Usage
```go
gofr wrap grpc -proto=<path_to_the_proto_file>
```
### Generated Files
- ```{serviceName}_gofr.go (auto-generated; do not modify)```
- ```{serviceName}_server.go (example structure below)```

### Benefits
- **Streamlined gRPC Integration**: Automatically generates necessary files to quickly set up gRPC services in your GoFr project.
- **Context-Aware Handling**: The generated server structure includes the necessary hooks to handle requests and responses seamlessly.
- **Minimal Configuration**: Simplifies gRPC handler setup, focusing on business logic rather than the infrastructure.

---

4. **`version`**

   The version command allows you to check the current version of the GoFr CLI tool installed on your system.
   Running this command ensures that you are using the latest version of the tool, helping you stay up to date with any new features, improvements, or bug fixes.
   It's a quick way to verify which version of the GoFr CLI you have.

### Command Usage 
```go
gofr version
```

