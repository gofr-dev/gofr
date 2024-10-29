package terminal

import (
	"fmt"
)

func (o *Output) Printf(format string, args ...interface{}) {
	fmt.Fprintf(o.out, format, args...)
}

func (o *Output) Print(messages ...interface{}) {
	fmt.Fprint(o.out, messages...)
}

func (o *Output) Println(messages ...interface{}) {
	fmt.Fprintln(o.out, messages...)
}
