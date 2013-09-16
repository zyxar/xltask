// +build darwin freebsd netbsd openbsd linux

package main

import (
	"code.google.com/p/go.crypto/ssh/terminal"
	"os"
)

type uterm struct {
	s *terminal.State
	t *terminal.Terminal
}

func newTerm() Term {
	u := new(uterm)
	var err error
	u.s, err = terminal.MakeRaw(0)
	if err != nil {
		panic(err)
	}
	u.t = terminal.NewTerminal(os.Stdin, "lixian >> ")
	return u
}

func (u *uterm) Restore() {
	terminal.Restore(0, u.s)
}

func (u *uterm) ReadLine() (string, error) {
	return u.t.ReadLine()
}
