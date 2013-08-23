// +build windows

package xl

import (
	"os"
	"path"
)

func initHome() {
	XLTASK_HOME = path.Dir(os.Args[0])
}
