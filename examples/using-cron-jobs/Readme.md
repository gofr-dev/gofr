# Using Cron Jobs in GoFr

This example demonstrates how to schedule and run background jobs using the **GoFr** framework’s built-in Cron Job support.

---

## Overview

In this example, we:

* Schedule a job named `counter` to run **every second**.
* Increment a counter on each execution and log its value.
* Run the application for a limited duration (3 seconds) to demonstrate the cron execution in action.
* Include a unit test to verify that the cron job executes successfully.

---

## How to Run

1. **Clone the repository** and navigate to this example:

   ```bash
   git clone https://github.com/gofr-dev/gofr.git
   cd gofr/examples/using-cron-jobs
   ```

2. **Run the application**:

   ```bash
   go run main.go
   ```

   * The counter will increment every second.
   * Application will stop automatically after 3 seconds.
     *(In a real-world application, you would call `app.Run()` to keep the cron running indefinitely.)*

---

## How It Works

### main.go

```go
app.AddCronJob("* * * * * *", "counter", count)
```

* Schedules the `count` function to run every second.
* Cron syntax here uses six fields (including seconds) — `"* * * * * *"` means “every second.”

```go
time.Sleep(duration * time.Second)
```

* Stops the application after the `duration` (3 seconds) for demonstration purposes.

### count Function

* Acquires a write lock.
* Increments the counter variable `n`.
* Logs the current counter value.

---

## Testing

The test (`main_test.go`):

* Runs the `main()` function.
* Waits slightly longer than 1 second.
* Checks if the counter has incremented exactly once.
* Logs the metrics server host.

Run the test:

```bash
go test -v
```

Expected output:

```
=== RUN   Test_UserPurgeCron
--- PASS: Test_UserPurgeCron (1.10s)
PASS
```

---

## Example Output

When running `main.go`, you should see:

```
INFO    Count: 1
INFO    Count: 2
INFO    Count: 3
```

