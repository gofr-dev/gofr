---
description: "Use the GoFr CLI to scaffold projects, generate boilerplate, and run framework-aware tasks. Speeds up repetitive work and keeps services consistent."
nextjs:
  metadata:
    title: "GoFr CLI Reference — Project Scaffolding and Tooling"
    description: "Use the GoFr CLI to scaffold projects, generate boilerplate, and run framework-aware tasks. Speeds up repetitive work and keeps services consistent."
---

# GoFr Command Line Interface

Managing repetitive tasks and maintaining consistency across large-scale applications is challenging!

**GoFr CLI provides the following:**

* All-in-one command-line tool designed specifically for GoFr applications
* Simplifies **database migrations** management
* **Store Layer Generator** for type-safe data access code from YAML configurations
* Abstracts **tracing**, **metrics** and structured **logging** for GoFr's gRPC server/client
* Enforces standard **GoFr conventions** in new projects

## Prerequisites

- Go 1.25 or above. To check Go version use the following command:
```bash
  go version
```

## **Installation**
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

```bash
  gofr <subcommand> [flags]=[arguments]
```
---

## Commands

The CLI groups its functionality into four commands. See each subpage for full reference:

- [`gofr init`](/docs/references/gofrcli/init) — initialize a new GoFr project with the standard layout.
- [`gofr migrate`](/docs/references/gofrcli/migrate) — create database migration templates with timestamped filenames and an auto-generated registry.
- [`gofr wrap grpc`](/docs/references/gofrcli/wrap-grpc) — generate gRPC server/client wrappers with built-in tracing, metrics, and logging.
- [`gofr store`](/docs/references/gofrcli/store) — generate a type-safe data-access layer from YAML schema definitions.
