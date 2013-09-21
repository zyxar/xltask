package xl

type XLTask struct {
	agent    *Agent
	Id       string  // task id
	Name     string  // task name
	Type     string  // task type, bt or nonbt
	Cat      string  // user defined catagory, not used currently
	Flag     string  // normal, expired, or deleted
	Status   string  // downloading, pending, paused, completed, failed
	Progress float32 // downloading progress
	Speed    string
	Life     string
	URL      string // original url
	DownURL  string // downloadable url
	Hash     string // cid, infohash of bt
	Cookie   string
	Size     string
	Length   string
	Uid      int
}

type Task interface {
	Task() *XLTask
	CloneTask() XLTask
}
