package terminal

import (
	"fmt"
)

func (o *output) Printf(format string, args ...interface{}) {
	fmt.Fprintf(o.out, format, args...)
}

func (o *output) Print(messages ...interface{}) {
	fmt.Fprint(o.out, messages...)
}

func (o *output) Println(messages ...interface{}) {
	fmt.Fprintln(o.out, messages...)
}
