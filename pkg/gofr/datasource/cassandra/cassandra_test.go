package cassandra

//func Test_Connect(t *testing.T) {
//	ctrl := gomock.NewController(t)
//	defer ctrl.Finish()
//
//	mockLogger := NewMockLogger(INFO)
//	mockMetrics := NewMockMetrics(ctrl)
//
//	testCases := []struct {
//		desc       string
//		config     *gocql.ClusterConfig
//		expected   string
//		shouldFail bool
//	}{
//		{"successful connection", &gocql.ClusterConfig{},
//			"connected to 'test_keyspace' keyspace at host 'host1, host2' and port '9042'", false},
//		{"connection failure", &gocql.ClusterConfig{}, "error connecting to cassandra: some error", true},
//	}
//
//	for i, tc := range testCases {
//		t.Run(tc.desc, func(t *testing.T) {
//			client := &Client{
//				clusterConfig: tc.config,
//				logger:        mockLogger,
//				metrics:       mockMetrics,
//			}
//
//			if tc.shouldFail {
//				mockLogger.EXPECT().Error(gomock.Any()).Times(1)
//			} else {
//				mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(2)
//			}
//
//			client.Connect()
//
//			if tc.shouldFail {
//				assert.Nil(t, client.session, "TEST[%d], Failed.\n%s", i, tc.desc)
//			} else {
//				assert.NotNil(t, client.session, "TEST[%d], Failed.\n%s", i, tc.desc)
//			}
//		})
//	}
//}
