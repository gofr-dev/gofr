package gcs

//
// func TestStorageAdapter_Connect(t *testing.T) {
// 	adapter := &storageAdapter{}
//
// 	err := adapter.Connect(context.Background())
//
// 	require.NoError(t, err)
// }
//
// func TestStorageAdapter_Health_NilClient(t *testing.T) {
// 	adapter := &storageAdapter{
// 		client: nil,
// 		bucket: nil,
// 	}
//
// 	err := adapter.Health(context.Background())
//
// 	require.Error(t, err)
// 	require.ErrorIs(t, err, errGCSClientNotInitialized)
// }
//
// func TestStorageAdapter_Health_NilBucket(t *testing.T) {
// 	adapter := &storageAdapter{
// 		client: &storage.Client{},
// 		bucket: nil,
// 	}
//
// 	err := adapter.Health(context.Background())
//
// 	require.Error(t, err)
// 	require.ErrorIs(t, err, errGCSClientNotInitialized)
// }
//
// func TestStorageAdapter_Close_NilClient(t *testing.T) {
// 	adapter := &storageAdapter{
// 		client: nil,
// 	}
//
// 	err := adapter.Close()
//
// 	require.NoError(t, err)
// }
//
// func TestStorageAdapter_NewReader_EmptyObjectName(t *testing.T) {
// 	adapter := &storageAdapter{
// 		client: &storage.Client{},
// 		bucket: &storage.BucketHandle{},
// 	}
//
// 	reader, err := adapter.NewReader(context.Background(), "")
//
// 	require.Error(t, err)
// 	require.Nil(t, reader)
// 	require.ErrorIs(t, err, errEmptyObjectName)
// }
//
// func TestStorageAdapter_NewRangeReader_EmptyObjectName(t *testing.T) {
// 	adapter := &storageAdapter{
// 		client: &storage.Client{},
// 		bucket: &storage.BucketHandle{},
// 	}
//
// 	reader, err := adapter.NewRangeReader(context.Background(), "", 0, 10)
//
// 	require.Error(t, err)
// 	require.Nil(t, reader)
// 	require.ErrorIs(t, err, errEmptyObjectName)
// }
//
// func TestStorageAdapter_NewRangeReader_InvalidOffset(t *testing.T) {
// 	adapter := &storageAdapter{
// 		client: &storage.Client{},
// 		bucket: &storage.BucketHandle{},
// 	}
//
// 	reader, err := adapter.NewRangeReader(context.Background(), "test.txt", -1, 10)
//
// 	require.Error(t, err)
// 	require.Nil(t, reader)
// 	require.ErrorIs(t, err, errInvalidOffset)
// }
//
// func TestStorageAdapter_NewWriter_EmptyObjectName(t *testing.T) {
// 	adapter := &storageAdapter{
// 		client: &storage.Client{},
// 		bucket: &storage.BucketHandle{},
// 	}
//
// 	writer := adapter.NewWriter(context.Background(), "")
//
// 	require.NotNil(t, writer)
//
// 	n, err := writer.Write([]byte("test"))
//
// 	require.Error(t, err)
// 	require.Equal(t, 0, n)
// 	require.ErrorIs(t, err, errEmptyObjectName)
// }
//
// func TestStorageAdapter_StatObject_EmptyObjectName(t *testing.T) {
// 	adapter := &storageAdapter{
// 		client: &storage.Client{},
// 		bucket: &storage.BucketHandle{},
// 	}
//
// 	info, err := adapter.StatObject(context.Background(), "")
//
// 	require.Error(t, err)
// 	require.Nil(t, info)
// 	require.ErrorIs(t, err, errEmptyObjectName)
// }
//
// func TestStorageAdapter_DeleteObject_EmptyObjectName(t *testing.T) {
// 	adapter := &storageAdapter{
// 		client: &storage.Client{},
// 		bucket: &storage.BucketHandle{},
// 	}
//
// 	err := adapter.DeleteObject(context.Background(), "")
//
// 	require.Error(t, err)
// 	require.ErrorIs(t, err, errEmptyObjectName)
// }
//
// func TestStorageAdapter_CopyObject_EmptySource(t *testing.T) {
// 	adapter := &storageAdapter{
// 		client: &storage.Client{},
// 		bucket: &storage.BucketHandle{},
// 	}
//
// 	err := adapter.CopyObject(context.Background(), "", "dest.txt")
//
// 	require.Error(t, err)
// 	require.ErrorIs(t, err, errEmptySourceOrDest)
// }
//
// func TestStorageAdapter_CopyObject_EmptyDestination(t *testing.T) {
// 	adapter := &storageAdapter{
// 		client: &storage.Client{},
// 		bucket: &storage.BucketHandle{},
// 	}
//
// 	err := adapter.CopyObject(context.Background(), "source.txt", "")
//
// 	require.Error(t, err)
// 	require.ErrorIs(t, err, errEmptySourceOrDest)
// }
//
// func TestStorageAdapter_CopyObject_BothEmpty(t *testing.T) {
// 	adapter := &storageAdapter{
// 		client: &storage.Client{},
// 		bucket: &storage.BucketHandle{},
// 	}
//
// 	err := adapter.CopyObject(context.Background(), "", "")
//
// 	require.Error(t, err)
// 	require.ErrorIs(t, err, errEmptySourceOrDest)
// }
//
// func TestStorageAdapter_CopyObject_SameSourceAndDest(t *testing.T) {
// 	adapter := &storageAdapter{
// 		client: &storage.Client{},
// 		bucket: &storage.BucketHandle{},
// 	}
//
// 	err := adapter.CopyObject(context.Background(), "file.txt", "file.txt")
//
// 	require.Error(t, err)
// 	require.ErrorIs(t, err, errSameSourceAndDest)
// }
//
// func TestFailWriter_Write(t *testing.T) {
// 	expectedErr := errRead
// 	fw := &failWriter{err: expectedErr}
//
// 	n, err := fw.Write([]byte("test data"))
//
// 	require.Error(t, err)
// 	require.Equal(t, 0, n)
// 	require.Equal(t, expectedErr, err)
// }
//
// func TestFailWriter_Close(t *testing.T) {
// 	expectedErr := errRead
// 	fw := &failWriter{err: expectedErr}
//
// 	err := fw.Close()
//
// 	require.Error(t, err)
// 	require.Equal(t, expectedErr, err)
// }
//
// func TestFailWriter_WriteEmptyObjectNameError(t *testing.T) {
// 	fw := &failWriter{err: errEmptyObjectName}
//
// 	n, err := fw.Write([]byte("test"))
//
// 	require.Error(t, err)
// 	require.Equal(t, 0, n)
// 	require.ErrorIs(t, err, errEmptyObjectName)
// }
//
// func TestFailWriter_CloseEmptyObjectNameError(t *testing.T) {
// 	fw := &failWriter{err: errEmptyObjectName}
//
// 	err := fw.Close()
//
// 	require.Error(t, err)
// 	require.ErrorIs(t, err, errEmptyObjectName)
// }
//
// // Tests using MockStorageProvider interface
//
// func TestMockStorageProvider_NewReader_Success(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()
//
// 	mockProvider := NewMockStorageProvider(ctrl)
// 	expectedReader := &fakeStorageReader{Reader: strings.NewReader("test content")}
//
// 	mockProvider.EXPECT().
// 		NewReader(gomock.Any(), "test.txt").
// 		Return(expectedReader, nil)
//
// 	reader, err := mockProvider.NewReader(context.Background(), "test.txt")
//
// 	require.NoError(t, err)
// 	require.NotNil(t, reader)
// }
//
// func TestMockStorageProvider_NewRangeReader_Success(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()
//
// 	mockProvider := NewMockStorageProvider(ctrl)
// 	expectedReader := &fakeStorageReader{Reader: strings.NewReader("partial content")}
//
// 	mockProvider.EXPECT().
// 		NewRangeReader(gomock.Any(), "test.txt", int64(0), int64(10)).
// 		Return(expectedReader, nil)
//
// 	reader, err := mockProvider.NewRangeReader(context.Background(), "test.txt", 0, 10)
//
// 	require.NoError(t, err)
// 	require.NotNil(t, reader)
// }
//
// func TestMockStorageProvider_NewWriter_Success(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()
//
// 	mockProvider := NewMockStorageProvider(ctrl)
// 	expectedWriter := &fakeStorageWriter{}
//
// 	mockProvider.EXPECT().
// 		NewWriter(gomock.Any(), "test.txt").
// 		Return(expectedWriter)
//
// 	writer := mockProvider.NewWriter(context.Background(), "test.txt")
//
// 	require.NotNil(t, writer)
// }
//
// func TestMockStorageProvider_StatObject_Success(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()
//
// 	mockProvider := NewMockStorageProvider(ctrl)
// 	expectedInfo := &file.ObjectInfo{
// 		Name:        "test.txt",
// 		Size:        1024,
// 		ContentType: "text/plain",
// 	}
//
// 	mockProvider.EXPECT().
// 		StatObject(gomock.Any(), "test.txt").
// 		Return(expectedInfo, nil)
//
// 	info, err := mockProvider.StatObject(context.Background(), "test.txt")
//
// 	require.NoError(t, err)
// 	require.NotNil(t, info)
// 	require.Equal(t, expectedInfo.Name, info.Name)
// 	require.Equal(t, expectedInfo.Size, info.Size)
// }
//
// func TestMockStorageProvider_DeleteObject_Success(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()
//
// 	mockProvider := NewMockStorageProvider(ctrl)
//
// 	mockProvider.EXPECT().
// 		DeleteObject(gomock.Any(), "test.txt").
// 		Return(nil)
//
// 	err := mockProvider.DeleteObject(context.Background(), "test.txt")
//
// 	require.NoError(t, err)
// }
//
// func TestMockStorageProvider_CopyObject_Success(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()
//
// 	mockProvider := NewMockStorageProvider(ctrl)
//
// 	mockProvider.EXPECT().
// 		CopyObject(gomock.Any(), "source.txt", "dest.txt").
// 		Return(nil)
//
// 	err := mockProvider.CopyObject(context.Background(), "source.txt", "dest.txt")
//
// 	require.NoError(t, err)
// }
//
// func TestMockStorageProvider_ListObjects_Success(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()
//
// 	mockProvider := NewMockStorageProvider(ctrl)
// 	expectedObjects := []string{"file1.txt", "file2.txt", "file3.txt"}
//
// 	mockProvider.EXPECT().
// 		ListObjects(gomock.Any(), "").
// 		Return(expectedObjects, nil)
//
// 	objects, err := mockProvider.ListObjects(context.Background(), "")
//
// 	require.NoError(t, err)
// 	require.Len(t, objects, 3)
// 	require.Equal(t, expectedObjects, objects)
// }
//
// func TestMockStorageProvider_ListDir_Success(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()
//
// 	mockProvider := NewMockStorageProvider(ctrl)
// 	expectedFiles := []file.ObjectInfo{
// 		{Name: "file1.txt", Size: 100},
// 		{Name: "file2.txt", Size: 200},
// 	}
// 	expectedDirs := []string{"subdir1/", "subdir2/"}
//
// 	mockProvider.EXPECT().
// 		ListDir(gomock.Any(), "").
// 		Return(expectedFiles, expectedDirs, nil)
//
// 	files, dirs, err := mockProvider.ListDir(context.Background(), "")
//
// 	require.NoError(t, err)
// 	require.Len(t, files, 2)
// 	require.Len(t, dirs, 2)
// 	require.Equal(t, expectedFiles, files)
// 	require.Equal(t, expectedDirs, dirs)
// }
//
// func TestMockStorageProvider_Health_Success(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()
//
// 	mockProvider := NewMockStorageProvider(ctrl)
//
// 	mockProvider.EXPECT().
// 		Health(gomock.Any()).
// 		Return(nil)
//
// 	err := mockProvider.Health(context.Background())
//
// 	require.NoError(t, err)
// }
//
// func TestMockStorageProvider_Connect_Success(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()
//
// 	mockProvider := NewMockStorageProvider(ctrl)
//
// 	mockProvider.EXPECT().
// 		Connect(gomock.Any()).
// 		Return(nil)
//
// 	err := mockProvider.Connect(context.Background())
//
// 	require.NoError(t, err)
// }
//
// func TestMockStorageProvider_Close_Success(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()
//
// 	mockProvider := NewMockStorageProvider(ctrl)
//
// 	mockProvider.EXPECT().
// 		Close().
// 		Return(nil)
//
// 	err := mockProvider.Close()
//
// 	require.NoError(t, err)
// }
//
// func TestMockStorageProvider_ListDir_EmptyPrefix(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()
//
// 	mockProvider := NewMockStorageProvider(ctrl)
// 	expectedFiles := []file.ObjectInfo{
// 		{Name: "file1.txt", Size: 100, ContentType: "text/plain", IsDir: false},
// 		{Name: "file2.txt", Size: 200, ContentType: "text/plain", IsDir: false},
// 	}
// 	expectedDirs := []string{"dir1/", "dir2/"}
//
// 	mockProvider.EXPECT().
// 		ListDir(gomock.Any(), "").
// 		Return(expectedFiles, expectedDirs, nil)
//
// 	files, dirs, err := mockProvider.ListDir(context.Background(), "")
//
// 	require.NoError(t, err)
// 	require.Len(t, files, 2)
// 	require.Len(t, dirs, 2)
// 	require.Equal(t, expectedFiles, files)
// 	require.Equal(t, expectedDirs, dirs)
// }
//
// func TestMockStorageProvider_ListDir_WithPrefix(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()
//
// 	mockProvider := NewMockStorageProvider(ctrl)
// 	expectedFiles := []file.ObjectInfo{
// 		{Name: "docs/readme.txt", Size: 500, ContentType: "text/plain", IsDir: false},
// 	}
// 	expectedDirs := []string{"docs/images/"}
//
// 	mockProvider.EXPECT().
// 		ListDir(gomock.Any(), "docs/").
// 		Return(expectedFiles, expectedDirs, nil)
//
// 	files, dirs, err := mockProvider.ListDir(context.Background(), "docs/")
//
// 	require.NoError(t, err)
// 	require.Len(t, files, 1)
// 	require.Len(t, dirs, 1)
// 	require.Equal(t, "docs/readme.txt", files[0].Name)
// 	require.Equal(t, int64(500), files[0].Size)
// 	require.Equal(t, "docs/images/", dirs[0])
// }
//
// func TestMockStorageProvider_ListDir_OnlyFiles(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()
//
// 	mockProvider := NewMockStorageProvider(ctrl)
// 	expectedFiles := []file.ObjectInfo{
// 		{Name: "a.txt", Size: 10, ContentType: "text/plain", IsDir: false},
// 		{Name: "b.txt", Size: 20, ContentType: "text/plain", IsDir: false},
// 		{Name: "c.txt", Size: 30, ContentType: "text/plain", IsDir: false},
// 	}
//
// 	mockProvider.EXPECT().
// 		ListDir(gomock.Any(), "files/").
// 		Return(expectedFiles, []string{}, nil)
//
// 	files, dirs, err := mockProvider.ListDir(context.Background(), "files/")
//
// 	require.NoError(t, err)
// 	require.Len(t, files, 3)
// 	require.Empty(t, dirs)
// }
//
// func TestMockStorageProvider_ListDir_OnlyDirectories(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()
//
// 	mockProvider := NewMockStorageProvider(ctrl)
// 	expectedDirs := []string{"subdir1/", "subdir2/", "subdir3/"}
//
// 	mockProvider.EXPECT().
// 		ListDir(gomock.Any(), "root/").
// 		Return([]file.ObjectInfo{}, expectedDirs, nil)
//
// 	files, dirs, err := mockProvider.ListDir(context.Background(), "root/")
//
// 	require.NoError(t, err)
// 	require.Empty(t, files)
// 	require.Len(t, dirs, 3)
// 	require.Equal(t, expectedDirs, dirs)
// }
//
// func TestMockStorageProvider_ListDir_Empty(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()
//
// 	mockProvider := NewMockStorageProvider(ctrl)
//
// 	mockProvider.EXPECT().
// 		ListDir(gomock.Any(), "empty/").
// 		Return([]file.ObjectInfo{}, []string{}, nil)
//
// 	files, dirs, err := mockProvider.ListDir(context.Background(), "empty/")
//
// 	require.NoError(t, err)
// 	require.Empty(t, files)
// 	require.Empty(t, dirs)
// }
//
// func TestMockStorageProvider_ListDir_Error(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()
//
// 	mockProvider := NewMockStorageProvider(ctrl)
// 	expectedErr := errObjectNotFound
//
// 	mockProvider.EXPECT().
// 		ListDir(gomock.Any(), "invalid/").
// 		Return(nil, nil, expectedErr)
//
// 	files, dirs, err := mockProvider.ListDir(context.Background(), "invalid/")
//
// 	require.Error(t, err)
// 	require.Nil(t, files)
// 	require.Nil(t, dirs)
// 	require.Equal(t, expectedErr, err)
// }
//
// func TestMockStorageProvider_ListDir_MixedContent(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()
//
// 	mockProvider := NewMockStorageProvider(ctrl)
// 	expectedFiles := []file.ObjectInfo{
// 		{Name: "data/file1.json", Size: 1024, ContentType: "application/json", IsDir: false},
// 		{Name: "data/file2.csv", Size: 2048, ContentType: "text/csv", IsDir: false},
// 	}
// 	expectedDirs := []string{"data/archive/", "data/temp/"}
//
// 	mockProvider.EXPECT().
// 		ListDir(gomock.Any(), "data/").
// 		Return(expectedFiles, expectedDirs, nil)
//
// 	files, dirs, err := mockProvider.ListDir(context.Background(), "data/")
//
// 	require.NoError(t, err)
// 	require.Len(t, files, 2)
// 	require.Len(t, dirs, 2)
//
// 	// Verify file details
// 	require.Equal(t, "data/file1.json", files[0].Name)
// 	require.Equal(t, int64(1024), files[0].Size)
// 	require.Equal(t, "application/json", files[0].ContentType)
// 	require.False(t, files[0].IsDir)
//
// 	// Verify directory names
// 	require.Contains(t, dirs, "data/archive/")
// 	require.Contains(t, dirs, "data/temp/")
// }
//
// func TestMockStorageProvider_ListDir_PermissionError(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()
//
// 	mockProvider := NewMockStorageProvider(ctrl)
// 	expectedErr := fmt.Errorf("%w \"restricted/\": permission denied", errFailedToListDirectory)
//
// 	mockProvider.EXPECT().
// 		ListDir(gomock.Any(), "restricted/").
// 		Return(nil, nil, expectedErr)
//
// 	files, dirs, err := mockProvider.ListDir(context.Background(), "restricted/")
//
// 	require.Error(t, err)
// 	require.Nil(t, files)
// 	require.Nil(t, dirs)
// 	require.ErrorIs(t, err, errFailedToListDirectory)
// }
//
// func TestMockStorageProvider_ListDir_LargeDirectory(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()
//
// 	mockProvider := NewMockStorageProvider(ctrl)
//
// 	// Generate large number of files
// 	var expectedFiles []file.ObjectInfo
// 	for i := 0; i < 100; i++ {
// 		expectedFiles = append(expectedFiles, file.ObjectInfo{
// 			Name:        fmt.Sprintf("large/file%d.txt", i),
// 			Size:        int64(i * 10),
// 			ContentType: "text/plain",
// 			IsDir:       false,
// 		})
// 	}
//
// 	mockProvider.EXPECT().
// 		ListDir(gomock.Any(), "large/").
// 		Return(expectedFiles, []string{}, nil)
//
// 	files, dirs, err := mockProvider.ListDir(context.Background(), "large/")
//
// 	require.NoError(t, err)
// 	require.Len(t, files, 100)
// 	require.Empty(t, dirs)
// 	require.Equal(t, "large/file0.txt", files[0].Name)
// 	require.Equal(t, "large/file99.txt", files[99].Name)
// }
//
// func TestMockStorageProvider_ListDir_SpecialCharacters(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()
//
// 	mockProvider := NewMockStorageProvider(ctrl)
// 	expectedFiles := []file.ObjectInfo{
// 		{Name: "special/file with spaces.txt", Size: 100, ContentType: "text/plain", IsDir: false},
// 		{Name: "special/file-with-dashes.txt", Size: 200, ContentType: "text/plain", IsDir: false},
// 	}
// 	expectedDirs := []string{"special/sub dir/"}
//
// 	mockProvider.EXPECT().
// 		ListDir(gomock.Any(), "special/").
// 		Return(expectedFiles, expectedDirs, nil)
//
// 	files, dirs, err := mockProvider.ListDir(context.Background(), "special/")
//
// 	require.NoError(t, err)
// 	require.Len(t, files, 2)
// 	require.Len(t, dirs, 1)
// 	require.Equal(t, "special/file with spaces.txt", files[0].Name)
// 	require.Equal(t, "special/sub dir/", dirs[0])
// }
