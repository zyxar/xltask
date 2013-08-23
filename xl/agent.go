package xl

import (
	"../bt"
	"../cookiejar"
	"bytes"
	"code.google.com/p/go.crypto/ssh/terminal"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type Agent struct {
	account  *Account
	conn     *http.Client
	On       bool
	id       string
	gdriveid string
	vm       []map[string]*_task
	sync.Mutex
}

type Account struct {
	id string
	pw string
}

var timestamp int
var cookieFile string
var noSuchTaskErr error
var invalidResponseErr error
var unexpectedErr error
var taskNotCompletedErr error
var invalidLoginErr error
var loginFailedErr error
var renameTaskErr error
var XLTASK_HOME string

func init() {
	noSuchTaskErr = errors.New("No such TaskId in list.")
	invalidResponseErr = errors.New("Invalid response.")
	unexpectedErr = errors.New("Unexpected error.")
	taskNotCompletedErr = errors.New("Task not completed.")
	invalidLoginErr = errors.New("Invalid login account.")
	loginFailedErr = errors.New("Login failed.")
	renameTaskErr = errors.New("Rename task ends with error.")
	initHome()
	err := mkConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	cookieFile = path.Join(XLTASK_HOME, "cookie.json")
}

func (this *Agent) SetPass() error {
	fmt.Print("Password: ")
	v, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}
	fmt.Println()
	this.account.pw = string(v)
	return nil
}

func NewAgent() *Agent {
	cookie, _ := cookiejar.New(nil)
	this := new(Agent)
	this.conn = &http.Client{nil, nil, cookie}
	this.account = new(Account)
	this.On = false
	this.vm = make([]map[string]*_task, t_total)
	this.vm[t_normal] = make(map[string]*_task)  // normal
	this.vm[t_expired] = make(map[string]*_task) // expired
	this.vm[t_deleted] = make(map[string]*_task) // deleted
	this.init()
	return this
}

func (this *Agent) init() {
	if this.Login("") != nil {
		return
	}
	this.tasklist_nofresh(4, 1, false)
}

func (this *Agent) Login(id string) error {
	this.Lock()
	defer this.Unlock()
	var vcode string
	this.conn.Jar.(*cookiejar.Jar).Load(cookieFile)
	if !this.IsOn() {
		if id == "" {
			return invalidLoginErr
		}
		this.account.id = id
		loginUrl := fmt.Sprintf("http://login.xunlei.com/check?u=%s&cachetime=%d", this.account.id, current_timestamp())
		u, _ := url.Parse("http://xunlei.com/")
	loop:
		_, err := this.get(loginUrl)
		if err != nil {
			return err
		}
		cks := this.conn.Jar.Cookies(u)
		for i, _ := range cks {
			if cks[i].Name == "check_result" {
				if len(cks[i].Value) < 3 {
					goto loop
				}
				vcode = cks[i].Value[2:]
				vcode = strings.ToUpper(vcode)
				log.Println("verify_code:", vcode)
				break
			}
		}
		this.SetPass()
		v := url.Values{}
		v.Set("u", this.account.id)
		v.Set("p", hashPass(this.account.pw, vcode))
		v.Set("verifycode", vcode)
		_, err = this.post("http://login.xunlei.com/sec2login/", v.Encode())
		if err != nil {
			return err
		}
		cks = this.conn.Jar.Cookies(u)
		for i, _ := range cks {
			if cks[i].Name == "userid" {
				this.id = cks[i].Value
				log.Println("id:", this.id)
				break
			}
		}
		if len(this.id) == 0 {
			return loginFailedErr
		}

		r, err := this.get(fmt.Sprintf("%slogin?cachetime=%d&from=0", DOMAIN_LIXIAN, current_timestamp()))
		if len(r) < 512 {
			return unexpectedErr
		}
		this.conn.Jar.(*cookiejar.Jar).Save(cookieFile)
		this.On = true
	}
	return nil
}

func (this *Agent) IsOn() bool {
	id := this.getCookie("http://xunlei.com", "userid")
	if id == "" {
		return false
	}
	this.id = id
	r, _ := this.get(fmt.Sprintf("%suser_task?userid=%s&st=0", DOMAIN_LIXIAN, this.id))
	if ok, _ := regexp.Match(`top.location='http://cloud.vip.xunlei.com/task.html\?error=`, r); ok {
		log.Println("previous login timeout")
		this.On = false
		return false
	}
	this.On = true
	return true
}

func (this *Agent) getCookie(uri, name string) string {
	u, _ := url.Parse(uri)
	cks := this.conn.Jar.Cookies(u)
	for i, _ := range cks {
		if cks[i].Name == name {
			return cks[i].Value
		}
	}
	return ""
}

func (this *Agent) Download(taskid string, fc Fetcher, echo bool) error {
	if fc == nil {
		fc = Aria2{}
	}
	task := this.vm[t_normal][taskid]
	if !AssertTaskId(taskid) || task == nil {
		return noSuchTaskErr
	}
	var err error
	switch task.TaskType {
	case _Task_BT:
		err = this.download_bt(task, fc, echo)
	case _Task_NONBT:
		fallthrough
	default:
		if task.DownloadStatus != "2" {
			return taskNotCompletedErr
		}
		err = this.download_(task.LixianURL, task.TaskName, fc, echo)
	}
	if err != nil {
		return err
	}
	return verifyTask(task, task.TaskName)
}

func (this *Agent) download_(uri, filename string, fc Fetcher, echo bool) error {
	if uri == "" {
		return unexpectedErr
	}
	return fc.Fetch(uri, this.gdriveid, filename, echo)
}

func (this *Agent) download_bt(task *_task, fc Fetcher, echo bool) error {
	btlist, err := this.FillBtList(task.Id)
	if err != nil {
		return err
	}
	rlist := btlist.Record
	for i, _ := range rlist {
		if rlist[i].Status == "2" {
			err = this.download_(rlist[i].DownURL, path.Join(task.TaskName, rlist[i].FileName), fc, echo)
			if err != nil {
				return err
			}
		} else {
			log.Printf("%sSkip incompleted task %s.%s", color_front_cyan, rlist[i].FileName, color_reset)
		}
	}
	return nil
}

func (this *Agent) ShowTasks() error {
	return this.tasklist_nofresh(4, 1, true)
}

func (this *Agent) tasklist_nofresh(tid, page int, show bool) error {
	if current_timestamp()-timestamp > 5000 {
		/*
			tid:
			1 downloading
			2 completed
			4 downloading|completed|expired
			11 deleted - not used now?
			13 expired - not used now?
		*/
		if tid != 4 || tid != 1 || tid != 2 {
			tid = 4
		}
		uri := fmt.Sprintf(SHOWTASK_UNFRESH, tid, page, _page_size, page)
		r, err := this.get(uri)
		if err != nil {
			return err
		}
		var resp _task_resp
		exp := regexp.MustCompile(`rebuild\((\{.*\})\)`)
		s := exp.FindSubmatch(r)
		if s == nil {
			return invalidResponseErr
		}
		json.Unmarshal(s[1], &resp)
		if this.gdriveid == "" {
			this.gdriveid = resp.Info.User.Cookie
			log.Println("gdriveid:", this.gdriveid)
		}
		ts := resp.Info.Tasks
		for i, _ := range ts {
			this.vm[t_normal][ts[i].Id] = &ts[i]
		}
	}
	if show {
		printTaskList(this.vm[t_normal])
	}
	timestamp = current_timestamp()
	return nil
}

func (this *Agent) readExpired() ([]byte, error) {
	uri := fmt.Sprintf(EXPIRE_HOME, this.id)
	log.Println("==>", uri)
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("User-Agent", user_agent)
	req.Header.Add("Accept-Encoding", "gzip, deflate")
	req.AddCookie(&http.Cookie{Name: "lx_nf_all", Value: url.QueryEscape(_expired_ck)})
	req.AddCookie(&http.Cookie{Name: "pagenum", Value: _page_size})
	resp, err := this.conn.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return readBody(resp)
}

func (this *Agent) ShowExpiredTasks(show bool) error {
	r, err := this.readExpired()
	ts, _ := parseHistory(r)
	for i, _ := range ts {
		this.vm[t_expired][ts[i].Id] = ts[i]
	}
	if show {
		printTaskList(this.vm[t_expired])
	}
	fmt.Printf("%s\n", ts)
	return err
}

func (this *Agent) readHistory(page int) ([]byte, error) {
	var uri string
	if page > 0 {
		uri = fmt.Sprintf(HISTORY_PAGE, this.id, page)
	} else {
		uri = fmt.Sprintf(HISTORY_HOME, this.id)
	}

	log.Println("==>", uri)
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("User-Agent", user_agent)
	req.Header.Add("Accept-Encoding", "gzip, deflate")
	req.AddCookie(&http.Cookie{Name: "lx_nf_all", Value: url.QueryEscape(_deleted_ck)})
	req.AddCookie(&http.Cookie{Name: "pagenum", Value: _page_size})
	resp, err := this.conn.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return readBody(resp)
}

func (this *Agent) ShowDeletedTasks(show bool) error {
	j := 0
	next := true
	var err error
	var r []byte
	var ts []*_task
	for next {
		j++
		r, err = this.readHistory(j)
		ts, next = parseHistory(r)
		for i, _ := range ts {
			this.vm[t_deleted][ts[i].Id] = ts[i]
		}
	}
	if show {
		printTaskList(this.vm[t_deleted])
	}
	return err
}

func parseHistory(in []byte) ([]*_task, bool) {
	es := `<input id="d_status(\d+)"[^<>]+value="(.*)" />\s+<input id="dflag\d+"[^<>]+value="(.*)" />\s+<input id="dcid\d+"[^<>]+value="(.*)" />\s+<input id="f_url\d+"[^<>]+value="(.*)" />\s+<input id="taskname\d+"[^<>]+value="(.*)" />\s+<input id="d_tasktype\d+"[^<>]+value="(.*)" />`
	exp := regexp.MustCompile(es)
	s := exp.FindAllSubmatch(in, -1)
	ret := make([]*_task, len(s))
	for i, _ := range s {
		b, _ := strconv.Atoi(string(s[i][7]))
		ret[i] = &_task{Id: string(s[i][1]), DownloadStatus: string(s[i][2]), Cid: string(s[i][4]), URL: string(s[i][5]), TaskName: string(s[i][6]), TaskType: byte(b)}
	}
	exp = regexp.MustCompile(`<li class="next"><a href="([^"]+)">[^<>]*</a></li>`)
	return ret, exp.FindSubmatch(in) != nil
}

func (this *Agent) DelayTask(taskid string) error {
	if !AssertTaskId(taskid) {
		return noSuchTaskErr
	}
	uri := fmt.Sprintf(TASKDELAY_URL, taskid+"_1", "task", current_timestamp())
	r, err := this.get(uri)
	if err != nil {
		return err
	}
	exp := regexp.MustCompile(`^task_delay_resp\((.*}),\[.*\]\)`)
	s := exp.FindSubmatch(r)
	if s == nil {
		return invalidResponseErr
	}
	var resp struct {
		K struct {
			Llt string `json:"left_live_time"`
		} `json:"0"`
		Result byte `json:"result"`
	}
	json.Unmarshal(s[1], &resp)
	log.Printf("%s: %s\n", taskid, resp.K.Llt)
	return nil
}

func (this *Agent) InfoTasks(ids []string) {
	for i, _ := range ids {
		if !AssertTaskId(ids[i]) {
			continue
		}
		task := this.vm[t_normal][ids[i]]
		if task == nil {
			task = this.vm[t_expired][ids[i]]
		}
		if task == nil {
			task = this.vm[t_deleted][ids[i]]
		}
		if task == nil {
			continue
		}
		if task.TaskType == _Task_BT {
			_, err := this.FillBtList(task.Id)
			if err != nil {
				fmt.Println(err)
			}
		} else {
			fmt.Printf("%s\n", task.Repr())
		}
	}
}

func (this *Agent) FillBtList(taskid string) (*_bt_list, error) {
	task := this.getTaskById(taskid)
	if !AssertTaskId(taskid) || task == nil {
		return nil, noSuchTaskErr
	}
	if task.TaskType != _Task_BT {
		return nil, fmt.Errorf("Not bt task.")
	}
	uri := fmt.Sprintf(FILLBTLIST_URL, task.Id, task.Cid, this.id, "task", current_timestamp())
	log.Println("==>", uri)
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("User-Agent", user_agent)
	req.Header.Add("Accept-Encoding", "gzip, deflate")
	req.AddCookie(&http.Cookie{Name: "pagenum", Value: _bt_page_size})
	resp, err := this.conn.Do(req)
	if err != nil {
		return nil, err
	}
	r, err := readBody(resp)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	exp := regexp.MustCompile(`fill_bt_list\({"Result":(.*)}\)`)
	s := exp.FindSubmatch(r)
	if s == nil {
		return nil, invalidResponseErr
	}
	var bt_list _bt_list
	json.Unmarshal(s[1], &bt_list)
	fmt.Printf("%v\n", bt_list)
	return &bt_list, nil
}

// supported uri schemes:
// 'ed2k', 'http', 'https', 'ftp', 'bt', 'magnet', 'thunder', 'Flashget', 'qqdl'
func (this *Agent) AddTask(req string) error {
	ttype := _TASK_TYPE
	if strings.HasPrefix(req, "magnet:") || strings.Contains(req, "get_torrent?userid=") {
		ttype = _TASK_TYPE_MAGNET
	} else if strings.HasPrefix(req, "ed2k://") {
		ttype = _TASK_TYPE_ED2K
	} else if strings.HasPrefix(req, "bt://") || strings.HasSuffix(req, ".torrent") {
		ttype = _TASK_TYPE_BT
	} else if ok, _ := regexp.MatchString(`^[a-zA-Z0-9]{40,40}$`, req); ok {
		ttype = _TASK_TYPE_BT
		req = "bt://" + req
	}
	switch ttype {
	case _TASK_TYPE, _TASK_TYPE_ED2K:
		return this.addSimpleTask(req)
	case _TASK_TYPE_BT:
		return this.addBtTask(req)
	case _TASK_TYPE_MAGNET:
		return this.addMagnetTask(req)
	case _TASK_TYPE_INVALID:
		fallthrough
	default:
		return unexpectedErr
	}
	panic(unexpectedErr.Error())
}

func (this *Agent) AddBatchTasks(urls []string) error {
	// TODO: filter urls
	v := url.Values{}
	for i := 0; i < len(urls); i++ {
		j := "[" + strconv.Itoa(i) + "]"
		v.Add("cid"+j, "")
		v.Add("url"+j, url.QueryEscape(urls[i]))
	}
	uri := fmt.Sprintf(BATCHTASKCOMMIT_URL, current_timestamp())
	r, err := this.post(uri, v.Encode())
	fmt.Printf("%s\n", r)
	return err
}

func (this *Agent) addSimpleTask(uri string) error {
	dest := fmt.Sprintf(TASKCHECK_URL, url.QueryEscape(uri), current_random(), current_timestamp())
	r, err := this.get(dest)
	if err == nil {
		task_pre, err := getTaskPre(r)
		if err != nil {
			return err
		}
		var t_type string
		if strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "ftp://") || strings.HasPrefix(uri, "https://") {
			t_type = strconv.Itoa(_TASK_TYPE)
		} else if strings.HasPrefix(uri, "ed2k://") {
			t_type = strconv.Itoa(_TASK_TYPE_ED2K)
		} else {
			return fmt.Errorf("Invalid protocol scheme.")
		}
		v := url.Values{}
		v.Add("callback", "ret_task")
		v.Add("uid", this.id)
		v.Add("cid", task_pre.Cid)
		v.Add("gcid", task_pre.GCid)
		v.Add("size", task_pre.SizeCost)
		v.Add("goldbean", task_pre.Goldbean)
		v.Add("silverbean", task_pre.Silverbean)
		v.Add("t", task_pre.FileName)
		v.Add("url", uri)
		v.Add("type", t_type)
		v.Add("o_page", "task")
		v.Add("o_taskid", "0")
		dest = TASKCOMMIT_URL + v.Encode()
		r, err = this.get(dest)
		if err != nil {
			return err
		}
		if ok, _ := regexp.Match(`ret_task\(.*\)`, r); ok {
			return nil
		} else {
			return invalidResponseErr
		}
	}
	return err
}

func (this *Agent) addBtTask(uri string) error {
	if strings.HasPrefix(uri, "bt://") {
		return this.addMagnetTask(fmt.Sprintf(GETTORRENT_URL, this.id, uri[5:]))
	}
	return this.addTorrentTask(uri)
}

func (this *Agent) addMagnetTask(link string) error {
	uri := fmt.Sprintf(URLQUERY_URL, url.QueryEscape(link), current_random())
	r, err := this.get(uri)
	if err != nil {
		return err
	}
	exp := regexp.MustCompile(`queryUrl\((1,.*)\)`)
	s := exp.FindSubmatch(r)
	if s == nil {
		if ok, _ := regexp.Match(`queryUrl\(-1,'[0-9A-Za-z]{40,40}'.*`, r); ok {
			return fmt.Errorf("Bt task already exists.")
		}
		return invalidResponseErr
	}
	task := evalParse(s[1])
	v := url.Values{}
	v.Add("uid", this.id)
	v.Add("btname", task.Name)
	v.Add("cid", task.InfoId)
	v.Add("tsize", task.Size)
	findex := strings.Join(task.Index, "_")
	size := strings.Join(task.Sizes, "_")
	v.Add("findex", findex)
	v.Add("size", size)
	v.Add("from", "0")
	dest := fmt.Sprintf(BTTASKCOMMIT_URL, current_timestamp())
	r, err = this.post(dest, v.Encode())
	exp = regexp.MustCompile(`jsonp.*\(\{"id":"(\d+)","avail_space":"\d+".*\}\)`)
	s = exp.FindSubmatch(r)
	if s == nil {
		return invalidResponseErr
	}
	this.tasklist_nofresh(4, 1, false)
	this.FillBtList(string(s[1]))
	return nil
}

func (this *Agent) addTorrentTask(filename string) (err error) {
	var file *os.File
	if file, err = os.Open(filename); err != nil {
		return
	}
	defer file.Close()
	if _, err = bt.GetMetaInfo(filename); err != nil {
		return
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	var part io.Writer
	if part, err = writer.CreateFormFile("filepath", filename); err != nil {
		return
	}
	io.Copy(part, file)
	writer.WriteField("random", current_random())
	writer.WriteField("interfrom", "task")

	dest := TORRENTUPLOAD_URL
	log.Println("==>", dest)
	req, err := http.NewRequest("POST", dest, bytes.NewReader(body.Bytes()))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Add("User-Agent", user_agent)
	req.Header.Add("Accept-Encoding", "gzip, deflate")
	resp, err := this.conn.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	r, err := readBody(resp)
	exp := regexp.MustCompile(`<script>document\.domain="xunlei\.com";var btResult =(\{.+\});var btRtcode = 0</script>`)
	s := exp.FindSubmatch(r)
	if s != nil {
		var result _btup_result
		json.Unmarshal(s[1], &result)
		v := url.Values{}
		v.Add("uid", this.id)
		v.Add("btname", result.Name) // TODO: filter illegal char
		v.Add("cid", result.InfoId)
		v.Add("tsize", strconv.Itoa(result.Size))
		findex := make([]string, 0, len(result.List))
		size := make([]string, 0, len(result.List))
		for i := 0; i < len(result.List); i++ {
			findex = append(findex, result.List[i].Id)
			size = append(size, result.List[i].Size)
		}
		v.Add("findex", strings.Join(findex, "_"))
		v.Add("size", strings.Join(size, "_"))
		v.Add("from", "0")
		dest = fmt.Sprintf(BTTASKCOMMIT_URL, current_timestamp())
		r, err = this.post(dest, v.Encode())
		exp = regexp.MustCompile(`jsonp.*\(\{"id":"(\d+)","avail_space":"\d+".*\}\)`)
		s = exp.FindSubmatch(r)
		if s == nil {
			return invalidResponseErr
		}
		this.tasklist_nofresh(4, 1, false)
		this.FillBtList(string(s[1]))
		return nil
	}
	exp = regexp.MustCompile(`parent\.edit_bt_list\((\{.*\}),'`)
	s = exp.FindSubmatch(r)
	if s == nil {
		return fmt.Errorf("Add bt task failed.")
	}
	// var result _btup_result
	// json.Unmarshal(s[1], &result)
	return fmt.Errorf("Bt task already exists.")
}

func (this *Agent) getReferer() string {
	return fmt.Sprintf(TASK_BASE, this.id)
}

func (this *Agent) GetTorrentByHash(hash, file string) {
	uri := fmt.Sprintf(GETTORRENT_URL, this.id, strings.ToUpper(hash))
	r, err := this.get(uri)
	if err != nil {
		return
	}
	exp := regexp.MustCompile(`alert\('(.*)'\)`)
	s := exp.FindSubmatch(r)
	if s != nil {
		log.Printf("%s\n", s[1])
		return
	}
	ioutil.WriteFile(file, r, 0644)
}

func (this *Agent) PauseTasks(ids []string) error {
	tids := strings.Join(ids, ",")
	tids += ","
	uri := fmt.Sprintf(TASKPAUSE_URL, tids, this.id, current_timestamp())
	r, err := this.get(uri)
	if err != nil {
		return err
	}
	if bytes.Compare(r, []byte("pause_task_resp()")) != 0 {
		return invalidResponseErr
	}
	return nil
}

func (this *Agent) getTaskById(taskid string) *_task {
	// TODO: merge all types
	return this.vm[t_normal][taskid]
}

func (this *Agent) RenameTask(taskid, newname string) error {
	if !AssertTaskId(taskid) {
		return noSuchTaskErr
	}
	task := this.getTaskById(taskid)
	if task == nil {
		return noSuchTaskErr
	}
	v := url.Values{}
	v.Add("taskid", taskid)
	if task.TaskType == _Task_BT {
		v.Add("bt", "1")
	} else {
		v.Add("bt", "0")
	}
	v.Add("filename", newname)
	r, err := this.get(RENAME_URL + v.Encode())
	if err != nil {
		return err
	}
	var resp struct {
		Result   int    `json:"result"`
		TaskId   int    `json:"taskid"`
		FileName string `json:"filename"`
	}
	json.Unmarshal(r[1:len(r)-1], &resp)
	if resp.Result != 0 {
		return fmt.Errorf("error in rename task: %d", resp.Result)
	}
	log.Println(resp.TaskId, "=>", resp.FileName)
	return nil
}

func (this *Agent) ReAddAllExpiredTasks() error {
	r, err := this.get(DELAYONCE_URL)
	if err != nil {
		return err
	}
	log.Printf("%s\n", r)
	return nil
}

func (this *Agent) DeleteTasks(ids []string) error {
	/*
	   del_type:
	   0 normal
	   1 deleted
	   3 normal|expired
	   4 all expired
	*/
	var del_type byte = 0
	var normal, deleted, expired bool
	vids := make([]string, 0, len(ids))
	j := 0
	for i, _ := range ids {
		// aggressively delete cache
		if this.vm[t_deleted][ids[i]] != nil {
			deleted = true
			delete(this.vm[t_deleted], ids[i])
		} else if this.vm[t_expired][ids[i]] != nil {
			expired = true
			delete(this.vm[t_expired], ids[i])
		} else if this.vm[t_normal][ids[i]] != nil {
			normal = true
			delete(this.vm[t_normal], ids[i])
		} else {
			continue
		}
		vids = append(vids, ids[i])
		j++
	}
	vids = vids[:j]
	tids := strings.Join(vids, ",")
	tids += ","
	if deleted && (normal || expired) {
		return fmt.Errorf("Can delete all mixed catagory of tasks")
	} else if deleted {
		del_type = t_deleted
	} else if expired && !normal {
		del_type = t_expired
	} else if expired && normal {
		del_type = t_mixed
	} else {
		del_type = t_normal
	}
	uri := fmt.Sprintf(TASKDELETE_URL, current_timestamp(), del_type, current_timestamp())
	data := url.Values{}
	data.Add("taskids", tids)
	data.Add("databases", "0,")
	data.Add("interfrom", "task")
	r, err := this.post(uri, data.Encode())
	if err != nil {
		return err
	}
	if ok, _ := regexp.Match(`\{"result":1,"type":`, r); ok {
		log.Printf("%s\n", r)
		return nil
	}
	return unexpectedErr
}

func (this *Agent) DeleteTask(taskid string) error {
	if !AssertTaskId(taskid) {
		return noSuchTaskErr
	}
	tids := taskid + ","
	/*
	   del_type:
	   0 normal
	   1 deleted
	   3 normal|expired
	   4 all expired
	*/
	var del_type byte
	if this.vm[t_normal][taskid] != nil {
		del_type = t_normal
	} else if this.vm[t_deleted][taskid] != nil {
		del_type = t_deleted
	} else if this.vm[t_expired][taskid] != nil {
		del_type = t_expired
	} else {
		return noSuchTaskErr
	}
	uri := fmt.Sprintf(TASKDELETE_URL, current_timestamp(), del_type, current_timestamp())
	data := url.Values{}
	data.Add("taskids", tids)
	data.Add("databases", "0,")
	data.Add("interfrom", "task")
	r, err := this.post(uri, data.Encode())
	if err != nil {
		return err
	}
	if ok, _ := regexp.Match(`\{"result":1,"type":`, r); ok {
		log.Printf("%s\n", r)
		delete(this.vm[del_type], taskid)
		return nil
	}
	return unexpectedErr
}

func (this *Agent) PurgeTask(taskid string) error {
	if !AssertTaskId(taskid) {
		return noSuchTaskErr
	}
	tids := taskid + ","
	var del_type byte
	if this.vm[t_expired][taskid] != nil {
		del_type = t_expired
	} else if this.vm[t_deleted][taskid] != nil {
		del_type = t_deleted
	} else {
		del_type = t_normal
	}
	uri := fmt.Sprintf(TASKDELETE_URL, current_timestamp(), del_type, current_timestamp())
	data := url.Values{}
	data.Add("taskids", tids)
	data.Add("databases", "0,")
	data.Add("interfrom", "task")
	r, err := this.post(uri, data.Encode())
	if err != nil {
		return err
	}
	if ok, _ := regexp.Match(`\{"result":1,"type":`, r); ok {
		log.Printf("%s\n", r)
		delete(this.vm[del_type], taskid)
		if del_type != t_deleted {
			uri = fmt.Sprintf(TASKDELETE_URL, current_timestamp(), t_deleted, current_timestamp())
			data = url.Values{}
			data.Add("taskids", tids)
			data.Add("databases", "0,")
			data.Add("interfrom", "task")
			r, err = this.post(uri, data.Encode())
			if err != nil {
				return err
			}
			if ok, _ := regexp.Match(`\{"result":1,"type":`, r); ok {
				log.Printf("%s\n", r)
				return nil
			} else {
				return unexpectedErr
			}
		}
		return nil
	}
	return unexpectedErr
}

func (this *Agent) get(dest string) ([]byte, error) {
	log.Println("==>", dest)
	req, err := http.NewRequest("GET", dest, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("User-Agent", user_agent)
	req.Header.Add("Accept-Encoding", "gzip, deflate")
	resp, err := this.conn.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return readBody(resp)
}

func (this *Agent) post(dest string, data string) ([]byte, error) {
	log.Println("==>", dest)
	req, err := http.NewRequest("POST", dest, strings.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("User-Agent", user_agent)
	req.Header.Add("Accept-Encoding", "gzip, deflate")
	resp, err := this.conn.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return readBody(resp)
}