package azure

// This file defines interfaces for Azure SDK clients to enable mocking with gomock.
// However, since Azure SDK clients are concrete types, we use httptest servers for testing
// instead of mocking the clients directly. The interfaces are kept for potential future use.

// Note: Azure SDK clients (share.Client, azfile.Client, directory.Client) are concrete structs,
// not interfaces, so they cannot be directly mocked with gomock without significant refactoring.
// For testing, we use:
// 1. httptest servers to mock HTTP responses (like GCS tests)
// 2. gomock for logger/metrics interfaces (from file package)
// 3. Table-driven tests with assert/require for all test cases

