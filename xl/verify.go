package xl

import (
	"bufio"
	"fmt"
	"github.com/zyxar/taipei"
	"github.com/zyxar/xltask/ed2k"
	"io/ioutil"
	"os"
	"strings"
)

func (this *Agent) verifyTask(task *_task, filename string) error {
	switch task.URL[:4] {
	case "ed2k":
		fmt.Println("Verifying [ED2K]", filename)
		return verify_ed2k(task, filename)
	case "bt:/":
		fmt.Println("Verifying [BT]", filename)
		return this.verify_bt(task, filename)
	}
	return nil
}

func verify_ed2k(task *_task, filename string) error {
	h, err := getEd2kHash(filename)
	if err != nil {
		return err
	}
	if fmt.Sprintf("%x", h) != getEd2kHashFromURL(task.URL) {
		return fmt.Errorf("ED2k hash checking failed.")
	}
	return nil
}

func (this *Agent) verify_bt(task *_task, filename string) error {
	tmp_torrent, err := ioutil.TempFile("", "xltorrent")
	if err != nil {
		return err
	}
	this.GetTorrentByHash(task.Cid, tmp_torrent.Name())
	m, err := taipei.GetMetaInfo(tmp_torrent.Name())
	if err != nil {
		return err
	}
	g, err := taipei.VerifyContent(m, filename)
	if g == false {
		return err
	}
	return nil
}

func getEd2kHash(filename string) ([]byte, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rd := bufio.NewReader(f)
	eh := ed2k.New()
	_, err = rd.WriteTo(eh)
	return eh.Sum(nil), err
}

func getEd2kHashFromURL(uri string) string {
	h := strings.Split(uri, "|")[4]
	return strings.ToLower(h)
}
