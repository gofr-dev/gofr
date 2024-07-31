package cassandra

import (
	"reflect"
	"time"

	"github.com/gocql/gocql"
)

type Batch struct {
	*Client

	batch batch
}

func NewBatch(client *Client, batchType int) (*Batch, error) {
	switch batchType {
	case LoggedBatch, UnloggedBatch, CounterBatch:
		batchClient := &Batch{Client: client}

		batchClient.batch = client.cassandra.session.newBatch(gocql.BatchType(batchType))

		return batchClient, nil
	default:
		return nil, ErrUnsupportedBatchType
	}
}

func (b *Batch) BatchQuery(stmt string, values ...any) {
	b.batch.Query(stmt, values...)
}

func (b *Batch) ExecuteBatch() error {
	defer b.postProcess(&QueryLog{Keyspace: b.config.Keyspace}, time.Now())

	if b.batch == nil {
		return ErrBatchNotInitialised
	}

	return b.cassandra.session.executeBatch(b.batch)
}

//nolint:exhaustive // We just want to take care of slice and struct in this case.
func (b *Batch) ExecuteBatchCAS(dest any) (bool, error) {
	applied, iter, err := b.cassandra.session.executeBatchCAS(b.batch)
	if err != nil {
		return false, err
	}

	rvo := reflect.ValueOf(dest)
	if rvo.Kind() != reflect.Ptr {
		b.logger.Error("we did not get a pointer. data is not settable.")

		return false, ErrDestinationIsNotPointer
	}

	rv := rvo.Elem()

	switch rv.Kind() {
	case reflect.Slice:
		numRows := iter.numRows()

		for numRows > 0 {
			val := reflect.New(rv.Type().Elem())

			if rv.Type().Elem().Kind() == reflect.Struct {
				b.rowsToStruct(iter, val)
			} else {
				_ = iter.scan(val.Interface())
			}

			rv = reflect.Append(rv, val.Elem())

			numRows--
		}

		if rvo.Elem().CanSet() {
			rvo.Elem().Set(rv)
		}

	case reflect.Struct:
		b.rowsToStruct(iter, rv)

	default:
		b.logger.Debugf("a pointer to %v was not expected.", rv.Kind().String())

		return false, UnexpectedPointer{target: rv.Kind().String()}
	}

	return applied, err
}
