// +build windows

package xl

import (
	"code.google.com/p/go.crypto/ssh/terminal"
	"os"
)

type win_passer struct {
}

func (*win_passer) Read(p []byte) (int, error) {
	return os.Stdin.Read(p)
}

func (*win_passer) Write(p []byte) (int, error) {
	return os.Stdout.Write(p)
}

func (this *win_passer) ReadPassword() (string, error) {
	term := terminal.NewTerminal(this, "")
	return term.ReadPassword("Password: ")
}

func initDefaultPassReader() {
	DefaultPassReader = &win_passer{}
}
