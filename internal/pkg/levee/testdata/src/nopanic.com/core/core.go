
package core

import (
)

type Source struct {
	Data string
	ID   int
}

func Calls() {
	s := Source{Data: "password", ID: 1337}
	panic(s)
}