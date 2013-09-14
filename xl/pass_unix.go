// +build darwin freebsd netbsd openbsd linux

package xl

import (
	"code.google.com/p/go.crypto/ssh/terminal"
	"fmt"
	"os"
)

type unix_passer struct {
}

func (u *unix_passer) ReadPassword() (string, error) {
	fmt.Print("Password: ")
	b, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	return string(b), err
}

func initDefaultPassReader() {
	DefaultPassReader = &unix_passer{}
}
