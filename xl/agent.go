package xl

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/zyxar/xltask/bt"
	"github.com/zyxar/xltask/cookiejar"
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
	conf     Config
	conn     *http.Client
	id       string
	gdriveid string
	vm       []map[string]*_task
	passrd   PassReader
	sync.Mutex
}

type PassReader interface {
	ReadPassword() (string, error)
}

type Config interface {
	GetId() string
	GetPass() string
	SetPass(string) error
	Boost() bool
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
var ReuseSession error

func init() {
	noSuchTaskErr = errors.New("No such TaskId in list.")
	invalidResponseErr = errors.New("Invalid response.")
	unexpectedErr = errors.New("Unexpected error.")
	taskNotCompletedErr = errors.New("Task not completed.")
	invalidLoginErr = errors.New("Invalid login account.")
	loginFailedErr = errors.New("Login failed.")
	renameTaskErr = errors.New("Rename task ends with error.")
	ReuseSession = errors.New("Reuse previous session in success.")
	initHome()
	err := mkConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	cookieFile = path.Join(XLTASK_HOME, "cookie.json")
}

func (this *Agent) SetPass() {
	if this.passrd == nil {
		this.passrd = DefaultPassReader
	}
	v, err := this.passrd.ReadPassword()
	if err != nil {
		log.Fatalf("Set password failed: %s\n", err)
	}
	fmt.Println()
	this.conf.SetPass(v)
}

func (this *Agent) UsePassReader(p PassReader) {
	if p != nil {
		this.passrd = p
	}
}

func NewAgent(c Config) *Agent {
	cookie, _ := cookiejar.New(nil)
	this := new(Agent)
	this.conf = c
	this.conn = &http.Client{nil, nil, cookie}
	this.vm = make([]map[string]*_task, t_total)
	this.vm[t_normal] = make(map[string]*_task)  // normal
	this.vm[t_expired] = make(map[string]*_task) // expired
	this.vm[t_deleted] = make(map[string]*_task) // deleted
	if c.Boost() {
		this.init()
	}
	return this
}

func (this *Agent) init() {
	if this.Login() != nil {
		return
	}
	this.tasklist_nofresh(4, 1, false)
}

func (this *Agent) Login() error {
	var vcode string
	this.conn.Jar.(*cookiejar.Jar).Load(cookieFile)
	if !this.IsOn() {
		var id string
		var err error
		if id = this.conf.GetId(); id == "" {
			return invalidLoginErr
		}
		loginUrl := fmt.Sprintf("http://login.xunlei.com/check?u=%s&cachetime=%d", id, current_timestamp())
		u, _ := url.Parse("http://xunlei.com/")
	loop:
		if _, err = this.get(loginUrl); err != nil {
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
		if this.conf.GetPass() == "" {
			this.SetPass()
		}
		v := url.Values{}
		v.Set("u", id)
		v.Set("p", hashPass(this.conf.GetPass(), vcode))
		v.Set("verifycode", vcode)
		if _, err = this.post("http://login.xunlei.com/sec2login/", v.Encode()); err != nil {
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
		if r, err := this.get(fmt.Sprintf("%slogin?cachetime=%d&from=0", DOMAIN_LIXIAN, current_timestamp())); err != nil || len(r) < 512 {
			return unexpectedErr
		}
		this.conn.Jar.(*cookiejar.Jar).Save(cookieFile)
	} else {
		return ReuseSession
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
		return false
	}
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

func (this *Agent) Download(taskid string, filter string, fc Fetcher, echo bool) error {
	if fc == nil {
		fc = DefaultFetcher
	}
	task := this.vm[t_normal][taskid]
	if !AssertTaskId(taskid) || task == nil {
		return noSuchTaskErr
	}
	var err error
	switch task.TaskType {
	case _Task_BT:
		err = this.download_bt(task, filter, fc, echo)
	case _Task_NONBT:
		fallthrough
	default:
		if task.DownloadStatus != "2" {
			return taskNotCompletedErr
		} else if task.expired() {
			return errors.New("Task expired.")
		}
		log.Println("Downloading", task.TaskName, "...")
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

func (this *Agent) download_bt(task *_task, filter string, fc Fetcher, echo bool) error {
	btlist, err := this.FillBtList(task.Id)
	if err != nil {
		return err
	}
	rlist := btlist.Record
	for i, _ := range rlist {
		// TODO: make filename safe
		if ok, _ := regexp.MatchString(`(?i)`+filter, rlist[i].FileName); ok && rlist[i].Status == "2" {
			log.Println("Downloading", rlist[i].FileName, "...")
			err = this.download_(rlist[i].DownURL, path.Join(task.TaskName, rlist[i].FileName), fc, echo)
			if err != nil {
				return err
			}
		} else if ok {
			log.Printf("%sSkip incompleted task %s.%s", color_front_cyan, rlist[i].FileName, color_reset)
		} else {
			log.Printf("%sSkip unselected task %s.%s", color_front_yellow, rlist[i].FileName, color_reset)
		}
	}
	return nil
}

func (this *Agent) Dispatch(pattern string, flag int) ([]string, error) {
	if ok, _ := regexp.MatchString(`^\d{7,}$`, pattern); ok {
		return []string{pattern}, nil
	}
	var ids []string
	if tasks, err := this.dispatch(`(?i)`+pattern, flag); err != nil {
		return nil, err
	} else {
		ids = make([]string, 0, len(tasks))
		for i, _ := range tasks {
			ids = append(ids, tasks[i].Id)
			fmt.Printf("#%d %s\n", i+1, tasks[i])
		}
	}
	return ids, nil
}

func (this *Agent) dispatch(pattern string, flag int) ([]*_task, error) {
	/*
	   flag:
	   	0, t_normal,
	   	1, t_deleted
	*/
	if flag < 0 || flag >= t_total {
		return nil, errors.New("Invalid task flag.")
	}
	expr, err := regexp.Compile(pattern)
	if err != nil {
		return nil, errors.New("Pattern unrecognised.")
	}
	var ret []*_task
	switch flag {
	case t_normal:
		fallthrough
	case t_deleted:
		m := this.vm[flag]
		ret = make([]*_task, 0, 32)
		for i, _ := range m {
			if expr.MatchString(m[i].TaskName) {
				ret = append(ret, m[i])
			}
		}
	default:
		ret = make([]*_task, 0, 32)
		for i, _ := range this.vm[t_normal] {
			if expr.MatchString(this.vm[t_normal][i].TaskName) {
				ret = append(ret, this.vm[t_normal][i])
			}
		}
		for i, _ := range this.vm[t_deleted] {
			if expr.MatchString(this.vm[t_deleted][i].TaskName) {
				ret = append(ret, this.vm[t_deleted][i])
			}
		}
	}
	return ret, nil
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
			if ts[i].expired() {
				this.vm[t_expired][ts[i].Id] = &ts[i]
			}
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
	this.Lock()
	resp, err := this.conn.Do(req)
	this.Unlock()
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
	this.Lock()
	resp, err := this.conn.Do(req)
	this.Unlock()
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

func (this *Agent) InfoTasks(id interface{}) {
	switch ids := id.(type) {
	case []string:
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
			fmt.Printf("%s\n", task.Repr())
			if task.TaskType == _Task_BT {
				_, err := this.FillBtList(task.Id)
				if err != nil {
					fmt.Println(err)
				}
			}
		}
	case map[string]string:
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
			fmt.Printf("%s\n", task.Repr())
			if task.TaskType == _Task_BT {
				_, err := this.FillBtList(task.Id)
				if err != nil {
					fmt.Println(err)
				}
			}
		}
	default:
	}
}

func (this *Agent) FillBtList(taskid string) (*_bt_list, error) {
	task := this.getTaskById(taskid)
	if task == nil {
		return nil, noSuchTaskErr
	}
	if task.expired() {
		return nil, errors.New("Task expired.")
	}
	if task.TaskType != _Task_BT {
		return nil, errors.New("Not bt task.")
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
	this.Lock()
	resp, err := this.conn.Do(req)
	this.Unlock()
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
		exp = regexp.MustCompile(`alert\('(.*)'\);.*`)
		s = exp.FindSubmatch(r)
		if s != nil {
			return nil, errors.New(string(s[1]))
		}
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
	this.Lock()
	resp, err := this.conn.Do(req)
	this.Unlock()
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

func (this *Agent) ProcessTask() error {
	uri := fmt.Sprintf(TASKPROCESS_URL, current_timestamp(), current_timestamp())
	v := url.Values{}
	l := len(this.vm[t_normal])
	list := make([]string, 0, l)
	nm_list := make([]string, 0, l)
	bt_list := make([]string, 0, l)
	for i, _ := range this.vm[t_normal] {
		if this.vm[t_normal][i].DownloadStatus == "1" {
			list = append(list, i)
			if this.vm[t_normal][i].TaskType == _Task_BT {
				bt_list = append(bt_list, i)
			} else {
				nm_list = append(nm_list, i)
			}
		}
	}
	if len(list) == 0 {
		return errors.New("Local task list empty.")
	}
	v.Add("list", strings.Join(list, ","))
	v.Add("nm_list", strings.Join(nm_list, ","))
	v.Add("bt_list", strings.Join(bt_list, ","))
	v.Add("uid", this.id)
	v.Add("interfrom", "task")
	var r []byte
	var err error
	if r, err = this.post(uri, v.Encode()); err != nil {
		return err
	}
	exp := regexp.MustCompile(`jsonp\d+\(\{"Process":(.*)\}\)`)
	s := exp.FindSubmatch(r)
	if s == nil {
		return invalidResponseErr
	}
	var res _ptask_resp
	json.Unmarshal(s[1], &res)
	for i, _ := range res.List {
		task := this.getTaskById(res.List[i].Id)
		task.update(&res.List[i])
		fmt.Printf("#%d %s\n", i, task)
	}
	return nil
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
	task := this.vm[t_normal][taskid]
	if task == nil {
		task = this.vm[t_expired][taskid]
	}
	if task == nil {
		task = this.vm[t_deleted][taskid]
	}
	return task
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
	this.Lock()
	resp, err := this.conn.Do(req)
	this.Unlock()
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
	this.Lock()
	resp, err := this.conn.Do(req)
	this.Unlock()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return readBody(resp)
}
