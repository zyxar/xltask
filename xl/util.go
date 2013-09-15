package xl

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"crypto/md5"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

var stats []string
var coloring []string

func init() {
	stats = make([]string, 16)
	stats[0] = "waiting"
	stats[1] = "downloading"
	stats[2] = "completed"
	stats[3] = "failed"
	stats[5] = "pending"
	stats[6] = "expired" // 2 + 4
	coloring = make([]string, 16)
	coloring[0] = color_front_yellow
	coloring[1] = color_front_magenta
	coloring[2] = color_front_green
	coloring[3] = color_front_red
	coloring[6] = color_front_cyan
	coloring[5] = color_front_blue
	coloring[4] = color_reset
}

func readBody(resp *http.Response) ([]byte, error) {
	buffer := bytes.NewBuffer(make([]byte, 0, 1024))
	var rd io.ReadCloser
	var err error
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		rd, _ = gzip.NewReader(resp.Body)
		defer rd.Close()
	case "deflate":
		rd = flate.NewReader(resp.Body)
		defer rd.Close()
	default:
		rd = resp.Body
	}
	defer func() {
		e := recover()
		if e == nil {
			return
		}
		if panicErr, ok := e.(error); ok && panicErr == bytes.ErrTooLarge {
			err = panicErr
		} else {
			panic(e)
		}
	}()
	_, err = buffer.ReadFrom(rd)
	return buffer.Bytes(), err
}

func current_timestamp() int {
	return int(time.Now().UnixNano() / 1000000)
}

func current_random() string {
	return fmt.Sprintf("%d%v", current_timestamp(), rand.Float64()*1000000)
}

func hashPass(pass, vcode string) string {
	h := md5.New()
	v := EncryptPass(pass)
	io.WriteString(h, v)
	io.WriteString(h, vcode)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func EncryptPass(pass string) string {
	if len(pass) == 32 {
		if ok, _ := regexp.MatchString(`[a-f0-9]{32,32}`, pass); ok {
			return pass
		}
	}
	h := md5.New()
	v := pass
	io.WriteString(h, v)
	v = fmt.Sprintf("%x", h.Sum(nil))
	h.Reset()
	io.WriteString(h, v)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func md5sum(raw interface{}) []byte {
	h := md5.New()
	switch raw.(type) {
	case []byte:
		h.Write(raw.([]byte))
	case string:
		io.WriteString(h, raw.(string))
	default:
		io.WriteString(h, fmt.Sprintf("%s", raw))
	}
	return h.Sum(nil)
}

func (this _task) String() string {
	j, _ := strconv.Atoi(this.DownloadStatus)
	k, _ := strconv.Atoi(this.Flag)
	j += k
	status := stats[j]
	return fmt.Sprintf("%s%s %s %s %s %.1f%% %s%s", coloring[j], this.Id, this.TaskName, status, this.FileSize, this.Progress, trim(this.LeftLiveTime), color_reset)
}

func (this _task) expired() bool {
	return this.Flag == "4"
}

func (this _task) Repr() string {
	j, _ := strconv.Atoi(this.DownloadStatus)
	k, _ := strconv.Atoi(this.Flag)
	j += k
	status := stats[j]
	ret := coloring[j] + this.Id + " " + this.TaskName + " " + status + " " + this.FileSize + " " + trim(this.LeftLiveTime) + "\n"
	if this.Cid != "" {
		ret += this.Cid + " "
	}
	if this.GCid != "" {
		ret += this.GCid + "\n"
	}
	ret += this.URL
	if this.LixianURL != "" {
		ret += "\n" + this.LixianURL
	}
	return ret + color_reset
}

func trim(raw string) string {
	exp := regexp.MustCompile(`<font color='([a-z]*)'>(.*)</font>`)
	s := exp.FindStringSubmatch(raw)
	if s == nil {
		return raw
	}
	return s[2]
}

func printTaskList(ts map[string]*_task) {
	j := 0
	for i, _ := range ts {
		j++
		fmt.Printf("#%d %v\n", j, ts[i])
	}
}

func AssertTaskId(taskid string) bool {
	r, _ := regexp.MatchString(`^\d+$`, taskid)
	return r
}

func (this _bt_list) String() string {
	r := fmt.Sprintf("%s%s %s/%d%s\n", color_front_blue, this.InfoId, this.BtNum, this.BtPerNum, color_reset)
	for i, _ := range this.Record {
		j, _ := strconv.Atoi(this.Record[i].Status)
		r += fmt.Sprintf("%s#%d %s %s %s%s\n", coloring[j], this.Record[i].Id, this.Record[i].FileName, this.Record[i].SizeReadable, stats[j], color_reset)
	}
	return r
}

func getTaskPre(resp []byte) (*_task_pre, error) {
	exp := regexp.MustCompile(`queryCid\((.*)\)`)
	s := exp.FindSubmatch(resp)
	if s == nil {
		return nil, invalidResponseErr
	}
	ss := bytes.Split(s[1], []byte(","))
	j := 0
	if len(ss) >= 10 {
		j = 1
	}
	ret := _task_pre{}
	ret.Cid = string(bytes.Trim(ss[0], "' "))
	ret.GCid = string(bytes.Trim(ss[1], "' "))
	ret.SizeCost = string(bytes.Trim(ss[2], "' "))
	ret.FileName = string(bytes.Trim(ss[j+3], "' "))
	ret.Goldbean = string(bytes.Trim(ss[j+4], "' "))
	ret.Silverbean = string(bytes.Trim(ss[j+5], "' "))
	var err error
	if ret.Goldbean != "0" || ret.Silverbean != "0" {
		err = fmt.Errorf("Task need bean: %s:%s", ret.Goldbean, ret.Silverbean)
	}
	return &ret, err
}

func evalParse(queryUrl []byte) *_bt_qtask {
	exp := regexp.MustCompile(`'([0-9A-Za-z]{40,40})','(\d+)','(.*)','(\d)',new Array\((.*)\),new Array\((.*)\),new Array\((.*)\),new Array\((.*)\),new Array\((.*)\),new Array\((.*)\),'([\d\.]+)','(\d)'`)
	s := exp.FindSubmatch(queryUrl)
	if s == nil {
		return nil
	}
	var task _bt_qtask
	task.InfoId = string(s[1])
	task.Size = string(s[2])
	task.Name = string(s[3])
	task.IsFull = string(s[4])
	a := bytes.Split(s[5], []byte(","))
	task.Files = make([]string, len(a))
	for i := 0; i < len(a); i++ {
		task.Files[i] = string(bytes.Trim(a[i], "' "))
	}
	a = bytes.Split(s[6], []byte(","))
	task.Sizesf = make([]string, len(a))
	for i := 0; i < len(a); i++ {
		task.Sizesf[i] = string(bytes.Trim(a[i], "' "))
	}
	a = bytes.Split(s[7], []byte(","))
	task.Sizes = make([]string, len(a))
	for i := 0; i < len(a); i++ {
		task.Sizes[i] = string(bytes.Trim(a[i], "' "))
	}
	a = bytes.Split(s[8], []byte(","))
	task.Picked = make([]string, len(a))
	for i := 0; i < len(a); i++ {
		task.Picked[i] = string(bytes.Trim(a[i], "' "))
	}
	a = bytes.Split(s[9], []byte(","))
	task.Ext = make([]string, len(a))
	for i := 0; i < len(a); i++ {
		task.Ext[i] = string(bytes.Trim(a[i], "' "))
	}
	a = bytes.Split(s[10], []byte(","))
	task.Index = make([]string, len(a))
	for i := 0; i < len(a); i++ {
		task.Index[i] = string(bytes.Trim(a[i], "' "))
	}
	task.Random = string(s[11])
	task.Ret = string(s[12])
	return &task
}
