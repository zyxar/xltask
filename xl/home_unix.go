// +build darwin freebsd netbsd openbsd linux

package xl

import (
	"os"
	"path"
)

func initHome() {
	XLTASK_HOME = path.Join(os.Getenv("HOME"), ".xltask")
}
