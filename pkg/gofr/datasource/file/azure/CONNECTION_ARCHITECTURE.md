# Azure File Storage Connection Architecture

## Overview

The Azure File Storage connection architecture follows a **layered design pattern** with three main components working together to provide resilient, observable, and efficient connection management.

## Architecture Layers

```
┌─────────────────────────────────────────────────────────────┐
│                    Application Layer                        │
│              (azure.New() - Factory Function)              │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│              azureFileSystem (Wrapper Layer)                │
│  • Embeds CommonFileSystem                                  │
│  • Manages retry logic                                      │
│  • Provides Connect() method                                │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│           CommonFileSystem (Common Layer)                   │
│  • Connection state management                              │
│  • Metrics & observability                                  │
│  • Logging                                                  │
│  • Fast-path optimization                                   │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│          storageAdapter (Provider Layer)                    │
│  • Azure SDK client creation                                │
│  • Credential management                                    │
│  • Share validation                                         │
└─────────────────────────────────────────────────────────────┘
```

## Connection Flow

### 1. Initialization (`azure.New()`)

```go
azure.New(config, logger, metrics)
```

**Steps:**
1. **Validation**: Validates `config` (AccountName, AccountKey, ShareName)
2. **Adapter Creation**: Creates `storageAdapter` with config
3. **FileSystem Creation**: Creates `azureFileSystem` embedding `CommonFileSystem`
4. **Initial Connection Attempt**: Calls `CommonFileSystem.Connect()` with 10-second timeout

**Two Possible Outcomes:**

#### Success Path:
```
New() → CommonFileSystem.Connect() → storageAdapter.Connect() → Success
                                                                    ↓
                                                          Return fs, nil
```

#### Failure Path (with Retry):
```
New() → CommonFileSystem.Connect() → storageAdapter.Connect() → Error
                                                                    ↓
                                                          Log warning
                                                                    ↓
                                                          Start background goroutine
                                                                    ↓
                                                          Return fs, nil (non-blocking)
```

### 2. Connection Layers

#### Layer 1: `azureFileSystem.Connect()`
- **Purpose**: Public API for manual connection attempts
- **Fast-path**: Checks if already connected via `CommonFileSystem.IsConnected()`
- **Behavior**: 
  - If connected → returns immediately
  - If not connected → delegates to `CommonFileSystem.Connect()`

#### Layer 2: `CommonFileSystem.Connect(ctx)`
- **Purpose**: Common connection logic with observability
- **Features**:
  - **Fast-path**: Returns immediately if `c.connected == true`
  - **Metrics Registration**: One-time histogram registration for file operations
  - **Observability**: Tracks connection duration and status
  - **State Management**: Sets `c.connected = true` on success
  - **Logging**: Logs connection success
- **Delegation**: Calls `c.Provider.Connect(ctx)` (which is `storageAdapter.Connect()`)

#### Layer 3: `storageAdapter.Connect(ctx)`
- **Purpose**: Azure SDK-specific connection logic
- **Fast-path**: Returns immediately if `s.shareClient != nil`
- **Steps**:
  1. **Credential Creation**: Creates Azure shared key credential
  2. **Endpoint Resolution**: 
     - Uses `config.Endpoint` if provided
     - Defaults to `https://{AccountName}.file.core.windows.net`
  3. **Share Name Validation**: Trims and validates share name
  4. **URL Construction**: Builds share URL: `{endpoint}/{shareName}`
  5. **Client Creation**: Creates Azure SDK `share.Client` with credentials
  6. **Share Validation**: Calls `GetProperties()` to verify share access
  7. **State Update**: Stores `shareClient` for future operations

## Retry Mechanism

### Background Retry (`startRetryConnect()`)

**Triggered When:**
- Initial connection in `New()` fails
- Application continues to run (non-blocking)

**Mechanism:**
```go
ticker := time.NewTicker(time.Minute)  // Retry every 1 minute
for range ticker.C {
    // Check exit conditions
    if IsConnected() || IsRetryDisabled() {
        return  // Exit retry loop
    }
    
    // Attempt connection
    err := CommonFileSystem.Connect(ctx)
    
    if err == nil {
        // Success - log and exit
        Logger.Infof("Azure connection restored to share %s", Location)
        return
    }
    
    // Failure - log debug and continue
    Logger.Debugf("Azure retry failed, will try again: %v", err)
}
```

**Exit Conditions:**
1. **Connection Success**: Share becomes accessible
2. **Retry Disabled**: `SetDisableRetry(true)` is called
3. **Already Connected**: Connection established by another goroutine

**Benefits:**
- **Resilience**: Automatically recovers from temporary network issues
- **Non-blocking**: Application starts even if Azure is temporarily unavailable
- **Observable**: Logs retry attempts and success

## Fast-Path Optimizations

### 1. Already Connected Check
```go
// In CommonFileSystem.Connect()
if c.connected {
    return nil  // Skip all connection logic
}
```

### 2. Client Reuse
```go
// In storageAdapter.Connect()
if s.shareClient != nil {
    return nil  // Skip client creation
}
```

### 3. Early Return in Connect()
```go
// In azureFileSystem.Connect()
if f.CommonFileSystem.IsConnected() {
    return  // Skip connection attempt
}
```

## Connection State Management

### State Variables

1. **`CommonFileSystem.connected`** (bool)
   - Tracks if connection is established
   - Set to `true` only after successful `Provider.Connect()`
   - Used for fast-path checks

2. **`CommonFileSystem.disableRetry`** (bool)
   - Controls background retry goroutine
   - Set via `SetDisableRetry(true)`
   - Used to stop retry loops in tests

3. **`storageAdapter.shareClient`** (*share.Client)
   - Azure SDK client instance
   - Created once and reused
   - Nil until first successful connection

## Error Handling

### Connection Errors

1. **Validation Errors** (in `New()`):
   - `errInvalidConfig`: Config is nil or ShareName is empty
   - `errAccountNameRequired`: AccountName is empty
   - `errAccountKeyRequired`: AccountKey is empty

2. **Connection Errors** (in `storageAdapter.Connect()`):
   - `errAzureConfigNil`: Config is nil
   - `errShareNameEmpty`: Share name is empty after trimming
   - Credential creation failures
   - Client creation failures
   - Share validation failures (network, permissions, etc.)

### Error Propagation

```
storageAdapter.Connect() error
    ↓
CommonFileSystem.Connect() error
    ↓
azureFileSystem (handles error, starts retry)
    ↓
Application (continues running, retry in background)
```

## Observability

### Metrics
- **Histogram**: `AppFileStats` - tracks connection duration
- **Registered Once**: Using `sync.Once` to avoid duplicate registration

### Logging
- **Info**: Connection success (`"connected to {shareName}"`)
- **Warning**: Initial connection failure (`"Azure File Share {shareName} not available, starting background retry"`)
- **Info**: Retry success (`"Azure connection restored to share {shareName}"`)
- **Debug**: Retry failures (`"Azure retry failed, will try again: {error}"`)

### Status Tracking
- **StatusSuccess**: Connection successful
- **StatusError**: Connection failed
- Tracked via `CommonFileSystem.Observe()` for metrics

## Timeout Management

- **Default Timeout**: `10 seconds` (`defaultTimeout`)
- **Applied At**: 
  - Initial connection in `New()`
  - Manual `Connect()` calls
  - Retry attempts in `startRetryConnect()`
- **Context Cancellation**: Proper cleanup with `defer cancel()`

## Thread Safety

- **Connection State**: Protected by connection checks (fast-path)
- **Client Creation**: Single creation, then reuse (no concurrent creation)
- **Retry Goroutine**: Runs independently, checks state before each attempt
- **Metrics Registration**: Uses `sync.Once` for thread-safe one-time registration

## Usage Patterns

### Pattern 1: Immediate Connection (Success)
```go
fs, err := azure.New(config, logger, metrics)
// fs is ready to use immediately
```

### Pattern 2: Delayed Connection (Failure with Retry)
```go
fs, err := azure.New(config, logger, metrics)
// fs returned, but connection will be established in background
// Operations will fail until connection succeeds
```

### Pattern 3: Manual Reconnection
```go
fs.(*azureFileSystem).Connect()  // Attempt immediate connection
```

## Key Design Decisions

1. **Non-blocking Initialization**: Application starts even if Azure is unavailable
2. **Layered Architecture**: Separation of concerns (wrapper → common → provider)
3. **Fast-path Optimizations**: Multiple levels of early returns
4. **Background Retry**: Automatic recovery without blocking application
5. **Observable**: Built-in metrics and logging at every layer
6. **State Management**: Clear connection state tracking
7. **Timeout Protection**: All connection attempts have timeouts

## Comparison with Other Providers

The Azure implementation follows the same pattern as GCS and S3:
- All use `CommonFileSystem` for common logic
- All implement `StorageProvider` interface
- All have retry mechanisms
- All provide observability

**Azure-specific differences:**
- Uses Azure SDK `share.Client` instead of cloud-specific clients
- Validates share access via `GetProperties()`
- Default endpoint pattern: `{AccountName}.file.core.windows.net`
- Retry interval: 1 minute (vs 30 seconds for GCS)

