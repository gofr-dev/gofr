package cache

//func TestRedis(t *testing.T) {
//	_ = config.NewGoDotEnvProvider(log.NewLogger(), "../../../configs")
//	redis, _ := datastore.NewRedisFromEnv(nil)
//	cache := NewRedisCacher(redis)
//	testRedisCacherGet(t, cache)
//	testRedisCacherSet(t, cache)
//	testRedisGetError(t, cache)
//	testRedisDelete(t, cache)
//	_ = cache.redis.Close()
//}
//
//func testRedisCacherGet(t *testing.T, redis RedisCacher) {
//	_ = redis.Set("k1", []byte("123"), time.Second*20)
//	expectedVal := []byte("123")
//
//	resp, _ := redis.Get("k1")
//	if !reflect.DeepEqual(resp, expectedVal) {
//		t.Errorf("[RedisGet]Failed.Got %v\tExpected %v\n", resp, expectedVal)
//	}
//}
//
//func testRedisCacherSet(t *testing.T, redis RedisCacher) {
//	err := redis.Set("k1", []byte("123"), time.Second*20)
//
//	if err != nil {
//		t.Errorf("[RedisSet]Failed. Expected error as nil. Got %v\n", err)
//	}
//}
//
//func testRedisGetError(t *testing.T, redis RedisCacher) {
//	_, err := redis.Get("unknown key")
//
//	if err == nil {
//		t.Errorf("Expected error, but got nil")
//	}
//}
//
//func testRedisDelete(t *testing.T, redis RedisCacher) {
//	_ = redis.Set("key1", []byte("123"), time.Second*20)
//
//	err := redis.Delete("key1")
//	if err != nil {
//		t.Errorf("Expected nil, Got: %v", err)
//	}
//}
