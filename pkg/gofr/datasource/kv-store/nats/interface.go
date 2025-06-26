package nats

import (
	"time"

	"github.com/nats-io/nats.go"
)

type KeyValue interface {
	Get(key string) (entry nats.KeyValueEntry, err error)
	GetRevision(key string, revision uint64) (entry nats.KeyValueEntry, err error)
	Put(key string, value []byte) (revision uint64, err error)
	PutString(key string, value string) (revision uint64, err error)
	Create(key string, value []byte) (revision uint64, err error)
	Update(key string, value []byte, last uint64) (revision uint64, err error)
	Delete(key string, opts ...nats.DeleteOpt) error
	Purge(key string, opts ...nats.DeleteOpt) error
	Watch(keys string, opts ...nats.WatchOpt) (nats.KeyWatcher, error)
	WatchAll(opts ...nats.WatchOpt) (nats.KeyWatcher, error)
	WatchFiltered(keys []string, opts ...nats.WatchOpt) (nats.KeyWatcher, error)
	Keys(opts ...nats.WatchOpt) ([]string, error)
	ListKeys(opts ...nats.WatchOpt) (nats.KeyLister, error)
	History(key string, opts ...nats.WatchOpt) ([]nats.KeyValueEntry, error)
	Bucket() string
	PurgeDeletes(opts ...nats.PurgeOpt) error
	Status() (nats.KeyValueStatus, error)
}

type JetStream interface {
	AccountInfo() (*nats.AccountInfo, error)
}

// MockKeyValueEntry for testing.
type MockKeyValueEntry struct {
	value []byte
}

func (*MockKeyValueEntry) Bucket() string             { return "" }
func (*MockKeyValueEntry) Key() string                { return "" }
func (m *MockKeyValueEntry) Value() []byte            { return m.value }
func (*MockKeyValueEntry) Revision() uint64           { return 0 }
func (*MockKeyValueEntry) Created() time.Time         { return time.Time{} }
func (*MockKeyValueEntry) Delta() uint64              { return 0 }
func (*MockKeyValueEntry) Operation() nats.KeyValueOp { return 0 }
