package xl

import (
	"encoding/json"
	"io/ioutil"
	"path"
)

type file_passer struct {
	filename string
}

func (f *file_passer) ReadPassword() (string, error) {
	cont, err := ioutil.ReadFile(f.filename)
	if err != nil {
		return "", err
	}
	var Conf struct {
		Password string `json:"password"`
	}
	json.Unmarshal(cont, &Conf)
	return Conf.Password, nil
}

var DefaultPassReader PassReader
var FilePassReader PassReader

func init() {
	FilePassReader = &file_passer{path.Join(XLTASK_HOME, "config.json")}
	initDefaultPassReader()
}
