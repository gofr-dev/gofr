package errors

import "testing"

func Test_DataStoreNotInitialized_Error(t *testing.T) {
	err := DataStoreNotInitialized{DBName: "SQL", Reason: "Empty host"}
	expErr := "couldn't initialize SQL, Empty host"

	if err.Error() != expErr {
		t.Errorf("FAILED, Expected: %v, Got: %v", expErr, err)
	}
}
