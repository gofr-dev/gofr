package terminal

import (
	"fmt"
)

func (o *Out) Printf(format string, args ...interface{}) {
	fmt.Fprintf(o.out, format, args...)
}

func (o *Out) Print(messages ...interface{}) {
	fmt.Fprint(o.out, messages...)
}

func (o *Out) Println(messages ...interface{}) {
	fmt.Fprintln(o.out, messages...)
}
