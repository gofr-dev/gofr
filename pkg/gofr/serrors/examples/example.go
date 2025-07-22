package examples

import (
	"fmt"
	"github.com/pkg/errors"
	"gofr.dev/pkg/gofr/serrors"
)

func Read() {
	err := errors.New("db connection error")
	serror := serrors.New(err, err.Error())
	fmt.Println(serrors.GetInternalError(serror, true))
}
