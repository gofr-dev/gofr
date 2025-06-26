package main

// Yet to be implemented
// // CacheTester provides methods to test cache implementations
// type CacheTester struct {
// 	name  string
// 	cache cache.Cache
// }
//
// func (ct *CacheTester) runTests(ctx context.Context) {
// 	fmt.Printf("\n=== Running tests for %s ===\n", ct.name)
//
// 	// Test basic Set and Get
// 	ct.testSetGet(ctx)
//
// 	// Test TTL expiration
// 	ct.testTTL(ctx)
//
// 	// Test Delete
// 	ct.testDelete(ctx)
//
// 	// Test WrapQuery
// 	ct.testWrapQuery(ctx)
//
// 	fmt.Printf("=== Completed tests for %s ===\n", ct.name)
// }
//
// func (ct *CacheTester) testSetGet(ctx context.Context) {
// 	fmt.Println("\nTesting Set/Get operations:")
//
// 	err := ct.cache.Set(ctx, "test-key", "test-value", time.Minute)
// 	if err != nil {
// 		log.Printf("Set failed: %v", err)
// 		return
// 	}
//
// 	value, err := ct.cache.Get(ctx, "test-key")
// 	if err != nil {
// 		log.Printf("Get failed: %v", err)
// 		return
// 	}
//
// 	if value == "test-value" {
// 		fmt.Println("✓ Set/Get test passed")
// 	} else {
// 		fmt.Printf("✗ Set/Get test failed. Expected 'test-value', got '%s'\n", value)
// 	}
// }
//
// func (ct *CacheTester) testTTL(ctx context.Context) {
// 	fmt.Println("\nTesting TTL expiration:")
//
// 	err := ct.cache.Set(ctx, "ttl-key", "ttl-value", time.Second)
// 	if err != nil {
// 		log.Printf("Set failed: %v", err)
// 		return
// 	}
//
// 	// Immediate get should succeed
// 	value, err := ct.cache.Get(ctx, "ttl-key")
// 	if err != nil || value != "ttl-value" {
// 		fmt.Println("✗ TTL test failed - immediate get failed")
// 		return
// 	}
//
// 	// Wait for TTL to expire
// 	time.Sleep(time.Second * 2)
//
// 	value, err = ct.cache.Get(ctx, "ttl-key")
// 	if err != nil || value != "" {
// 		fmt.Println("✗ TTL test failed - value persisted after TTL")
// 		return
// 	}
//
// 	fmt.Println("✓ TTL test passed")
// }
//
// func (ct *CacheTester) testDelete(ctx context.Context) {
// 	fmt.Println("\nTesting Delete operation:")
//
// 	err := ct.cache.Set(ctx, "delete-key", "delete-value", time.Minute)
// 	if err != nil {
// 		log.Printf("Set failed: %v", err)
// 		return
// 	}
//
// 	err = ct.cache.Delete(ctx, "delete-key")
// 	if err != nil {
// 		log.Printf("Delete failed: %v", err)
// 		return
// 	}
//
// 	value, err := ct.cache.Get(ctx, "delete-key")
// 	if err != nil || value != "" {
// 		fmt.Println("✗ Delete test failed - value still exists")
// 		return
// 	}
//
// 	fmt.Println("✓ Delete test passed")
// }
//
// func (ct *CacheTester) testWrapQuery(ctx context.Context) {
// 	fmt.Println("\nTesting WrapQuery functionality:")
//
// 	queryCount := 0
// 	queryFn := func(ctx context.Context) (string, error) {
// 		queryCount++
// 		return fmt.Sprintf("query-result-%d", queryCount), nil
// 	}
//
// 	// The first call should execute the query
// 	result1, err := ct.cache.WrapQuery(ctx, "wrap-key", time.Minute, queryFn)
// 	if err != nil {
// 		log.Printf("WrapQuery failed: %v", err)
// 		return
// 	}
//
// 	// Second call should return a cached result
// 	result2, err := ct.cache.WrapQuery(ctx, "wrap-key", time.Minute, queryFn)
// 	if err != nil {
// 		log.Printf("WrapQuery failed: %v", err)
// 		return
// 	}
//
// 	if result1 == result2 && queryCount == 1 {
// 		fmt.Println("✓ WrapQuery test passed")
// 	} else {
// 		fmt.Printf("✗ WrapQuery test failed. QueryCount: %d, Results: %s, %s\n",
// 			queryCount, result1, result2)
// 	}
// }
//
// func main() {
// 	ctx := context.Background()
//
// 	// Initialize Redis cache
// 	rdbConfig := redisdb.RdbConfig{
// 		Host:     "localhost",
// 		Port:     "6379",
// 		Password: "",
// 	}
//
// 	rdb, err := rdbConfig.RdbConnect(ctx)
// 	if err != nil {
// 		log.Fatalf("Error connecting to Redis: %v", err)
// 	}
//
// 	// Create cache instances
// 	redisImpl := redisCache.NewRedisConfig(rdb)
// 	redisImpl = cache.NewTracer(redisImpl)
// 	redisImpl = cache.WithContextSupport(redisImpl)
// 	redisImpl = cache.WithLogging(redisImpl, logging.NewLogger(1))
// 	memoryImpl := memory.NewInMemoryCache()
//
// 	// Create testers
// 	redisTester := &CacheTester{
// 		name:  "Redis Cache",
// 		cache: redisImpl,
// 	}
// 	memoryTester := &CacheTester{
// 		name:  "In-Memory Cache",
// 		cache: memoryImpl,
// 	}
//
// 	// Run tests
// 	redisTester.runTests(ctx)
// 	memoryTester.runTests(ctx)
//
// 	// Clean up
// 	if err := rdb.Close(); err != nil {
// 		log.Printf("Error closing Redis connection: %v", err)
// 	}
// }
