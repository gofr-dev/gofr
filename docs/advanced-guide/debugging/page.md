# Using `pprof` in GoFr Applications

In GoFr applications, `pprof` profiling is automatically enabled when the environment variable `APP_ENV` is set to `DEBUG`. The profiling endpoints are served on the internal `METRICS_PORT`, which defaults to `2121` if not specified.

This guide explains how to enable and use `pprof` in GoFr applications.

---

## Enabling `pprof` in GoFr

### Prerequisites
1. Set the environment variable `APP_ENV` to `DEBUG` in your `/configs/*.env` file:
   ```bash
   APP_ENV=DEBUG
   ```
2. Ensure the `METRICS_PORT` is set (default is `2121`):
   ```bash
   METRICS_PORT=2121
   ```

When `APP_ENV=DEBUG`, GoFr automatically registers the following `pprof` routes:
- `/debug/pprof/cmdline`
- `/debug/pprof/profile`
- `/debug/pprof/symbol`
- `/debug/pprof/trace`
- `/debug/pprof/` (index)

---

## Accessing `pprof` Endpoints

Once `pprof` is enabled, you can access the profiling endpoints at `http://localhost:<METRICS_PORT>/debug/pprof/`. For example, if `METRICS_PORT` is `2121`, the endpoints will be available at:
- `http://localhost:2121/debug/pprof/`

### Available Endpoints
1. **`/debug/pprof/cmdline`**:
    - Returns the command-line arguments of the running application.

2. **`/debug/pprof/profile`**:
    - Generates a CPU profile for the application.

3. **`/debug/pprof/symbol`**:
    - Resolves program counters into function names.

4. **`/debug/pprof/trace`**:
    - Captures an execution trace of the application.

5. **`/debug/pprof/` (index)**:
    - Provides an index page with links to all available profiling endpoints, including memory, goroutine, and blocking profiles.

---

## Collecting Profiling Data

### 1. **CPU Profiling**
To collect a CPU profile:
```bash
curl -o cpu.pprof http://localhost:2121/debug/pprof/profile
```

### 2. **Memory Profiling**
To collect a memory profile:
```bash
curl -o mem.pprof http://localhost:2121/debug/pprof/heap
```

### 3. **Goroutine Profiling**
To collect information about running goroutines:
```bash
curl -o goroutine.pprof http://localhost:2121/debug/pprof/goroutine
```

### 4. **Execution Trace**
To collect an execution trace:
```bash
curl -o trace.out http://localhost:2121/debug/pprof/trace
```

---

## Analyzing Profiling Data

### Using `go tool pprof`
To analyze CPU, memory, or goroutine profiles:
```bash
go tool pprof <profile_file>
```

#### **`top`**
Shows the functions consuming the most resources (e.g., CPU or memory).
```bash
go tool pprof cpu.pprof
(pprof) top
```

#### **`list`**
Displays the source code of a specific function, along with resource usage.
```bash
(pprof) list <function_name>
```
Example:
```bash
(pprof) list main.myFunction
```

#### **`web`**
Generates a visual representation of the profile in your browser. This requires Graphviz to be installed.
```bash
(pprof) web
```


### Using `go tool trace`
To analyze execution traces:
```bash
go tool trace trace.out
```

---

## Example Workflow

1. **Set Environment Variables**:
   ```bash
   APP_ENV=DEBUG
   METRICS_PORT=2121
   ```

2. **Run Your GoFr Application**:
   ```bash
   go run main.go
   ```

3. **Collect Profiling Data**:
    - Collect a CPU profile:
      ```bash
      curl -o cpu.pprof http://localhost:2121/debug/pprof/profile
      ```
    - Collect a memory profile:
      ```bash
      curl -o mem.pprof http://localhost:2121/debug/pprof/heap
      ```


4. **Analyze the Data**:
    - Analyze the CPU profile:
      ```bash
      go tool pprof cpu.pprof
      (pprof) top
      (pprof) list main.myFunction
      (pprof) web
      ```
    - Analyze the memory profile:
      ```bash
      go tool pprof mem.pprof
      (pprof) top
      (pprof) list main.myFunction
      (pprof) web
      ```

---

## References
- [Go `pprof` Documentation](https://pkg.go.dev/net/http/pprof)
- [Profiling Go Programs](https://blog.golang.org/profiling-go-programs)
- [Go Execution Tracer](https://golang.org/doc/diagnostics.html#tracing)
```