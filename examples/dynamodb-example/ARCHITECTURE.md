# DynamoDB Integration Architecture

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        GoFr Application                        │
├─────────────────────────────────────────────────────────────────┤
│  Context (gofr.Context)                                        │
│  ├── Request                                                   │
│  ├── Container                                                │
│  │   ├── Logger                                               │
│  │   ├── Metrics                                              │
│  │   ├── Tracer                                               │
│  │   └── DynamoDB (DynamoDBProvider)                          │
│  └── Responder                                                │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                    DynamoDB Interface                          │
├─────────────────────────────────────────────────────────────────┤
│  interface DynamoDB {                                          │
│    Get(ctx, key) (map[string]any, error)                      │
│    Set(ctx, key, attributes) error                            │
│    Delete(ctx, key) error                                     │
│    HealthCheck(ctx) (any, error)                              │
│  }                                                             │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                DynamoDB Implementation                         │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │   AWS SDK v2    │  │   Logging       │  │   Metrics       │ │
│  │   - PutItem     │  │   - Debug       │  │   - Histograms  │ │
│  │   - GetItem     │  │   - Info        │  │   - Counters    │ │
│  │   - DeleteItem  │  │   - Error       │  │                 │ │
│  │   - DescribeTable│  │                 │  │                 │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │   Tracing       │  │   Health Check  │  │   Configuration │ │
│  │   - Spans       │  │   - Table Status│  │   - Table Name  │ │
│  │   - Attributes  │  │   - Region Info │  │   - Region      │ │
│  │   - Duration    │  │   - Error Info  │  │   - Endpoint    │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                    AWS DynamoDB                                │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │   Production    │  │   Local Dev     │  │   Testing       │ │
│  │   - Real AWS    │  │   - DynamoDB    │  │   - Uber Mocks  │ │
│  │   - IAM Roles   │  │   - Local       │  │   - Unit Tests  │ │
│  │   - Regions     │  │   - Docker      │  │   - Integration │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Testing Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Test Suite                                  │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │   Unit Tests    │  │   Mock Tests    │  │   Integration   │ │
│  │   - Interface   │  │   - Uber Mocks  │  │   - Real DB     │ │
│  │   - Logic       │  │   - Expectations│  │   - Docker      │ │
│  │   - Error Cases │  │   - Error Sims  │  │   - End-to-End  │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                Uber Mocks (go.uber.org/mock)                   │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │   MockDynamoDB  │  │   Expectations  │  │   Verification  │ │
│  │   - Set()       │  │   - Times()     │  │   - Calls()     │ │
│  │   - Get()       │  │   - Return()    │  │   - Order()     │ │
│  │   - Delete()    │  │   - Do()        │  │   - AnyTimes()  │ │
│  │   - HealthCheck()│  │                 │  │                 │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Data Flow

```
1. HTTP Request → GoFr Context
2. Context.DynamoDB.Set() → DynamoDB Interface
3. DynamoDB Implementation → AWS SDK
4. AWS SDK → DynamoDB Table
5. Response ← AWS SDK ← DynamoDB Table
6. Response ← DynamoDB Implementation ← AWS SDK
7. Response ← Context.DynamoDB ← DynamoDB Interface
8. HTTP Response ← GoFr Context
```

## Key Components

### 1. Interface Layer
- **DynamoDB**: Core interface with Get, Set, Delete, HealthCheck
- **DynamoDBProvider**: Extended interface with logging, metrics, tracing

### 2. Implementation Layer
- **AWS SDK v2**: Official AWS DynamoDB client
- **AttributeValue**: Marshaling/unmarshaling for DynamoDB types
- **Configuration**: Table, region, endpoint settings

### 3. Observability Layer
- **Logging**: Debug, info, error logging with structured data
- **Metrics**: Histogram for operation duration, counters for success/failure
- **Tracing**: OpenTelemetry spans with operation details

### 4. Testing Layer
- **Uber Mocks**: Generated mocks for all interfaces
- **Mock Container**: Pre-configured test container with all mocks
- **Test Cases**: Unit tests, integration tests, error scenarios

## Benefits

1. **Separation of Concerns**: Clean interface separation
2. **Testability**: Comprehensive mocking with Uber mocks
3. **Observability**: Full logging, metrics, and tracing support
4. **Flexibility**: Easy to swap implementations
5. **Consistency**: Follows GoFr patterns and conventions
6. **Maintainability**: Well-documented and structured code

