package terminal

import (
	"fmt"
)

func (o *Out) Printf(format string, args ...any) {
	fmt.Fprintf(o.out, format, args...)
}

func (o *Out) Print(messages ...any) {
	fmt.Fprint(o.out, messages...)
}

func (o *Out) Println(messages ...any) {
	fmt.Fprintln(o.out, messages...)
}
