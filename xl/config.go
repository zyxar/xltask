package xl

import (
	"errors"
	"os"
)

func mkConfigDir() (err error) {
	if XLTASK_HOME == "" {
		return os.ErrNotExist
	}
	exists, err := isDirExists(XLTASK_HOME)
	if err != nil {
		return
	}
	if exists {
		return
	}
	return os.Mkdir(XLTASK_HOME, 0755)
}

func isDirExists(path string) (bool, error) {
	stat, err := os.Stat(path)
	if err == nil {
		if stat.IsDir() {
			return true, nil
		}
		return false, errors.New(path + " exists but is not a directory")
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
