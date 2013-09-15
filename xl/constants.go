package xl

const (
	user_agent    = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_7_5) AppleWebKit/537.13 (KHTML, like Gecko) Chrome/24.0.1290.1 Safari/537.13"
	_page_size    = "100"
	_bt_page_size = "999"
)

const (
	t_normal = iota
	t_deleted
	_
	t_mixed
	t_expired
	t_total
)

const (
	_Task_BT = iota
	_Task_NONBT
)

const (
	_TASK_TYPE = iota
	_TASK_TYPE_BT
	_TASK_TYPE_ED2K
	_TASK_TYPE_INVALID
	_TASK_TYPE_MAGNET
)

const (
	color_front_red     = "\x1b[31m"
	color_front_green   = "\x1b[32m"
	color_front_yellow  = "\x1b[33m"
	color_front_blue    = "\x1b[34m"
	color_front_magenta = "\x1b[35m"
	color_front_cyan    = "\x1b[36m"
	color_reset         = "\x1b[0m"
)

const (
	_deleted_ck = `page_check_all=history&fltask_all_guoqi=1&class_check=0&page_check=task&fl_page_id=0&class_check_new=0&set_tab_status=11`
	_expired_ck = `page_check_all=history&class_check=0&page_check=task&fl_page_id=0&class_check_new=0&set_tab_status=13`
)

const (
	DOMAIN_LIXIAN       = "http://dynamic.cloud.vip.xunlei.com/"
	TASK_BASE           = DOMAIN_LIXIAN + "user_task?userid=%s" //G_USERID
	TASK_HOME           = DOMAIN_LIXIAN + "user_task?userid=%s&st=4"
	TASK_PAGE           = DOMAIN_LIXIAN + "user_task?userid=%s&st=%s&p=%s" //"G_USERID,G_STATUS,G_PAGENUM
	HISTORY_HOME        = DOMAIN_LIXIAN + "user_history?userid=%s"
	EXPIRE_HOME         = DOMAIN_LIXIAN + "user_history?type=1&userid=%s"
	HISTORY_PAGE        = DOMAIN_LIXIAN + "user_history?userid=%s&p=%d"
	APPLY_HOME          = DOMAIN_LIXIAN + "user_apply?userid=%s"
	APPLY_PAGE          = DOMAIN_LIXIAN + "user_apply?userid=%s&p=%s"
	LOGIN_URL           = DOMAIN_LIXIAN + "login"
	INTERFACE_URL       = DOMAIN_LIXIAN + "interface"
	TASKDELAY_URL       = INTERFACE_URL + "/task_delay?taskids=%s&interfrom=%s&noCacheIE=%d"
	GETTORRENT_URL      = INTERFACE_URL + "/get_torrent?userid=%s&infoid=%s"
	TASKPAUSE_URL       = INTERFACE_URL + "/task_pause?tid=%s&uid=%s&noCacheIE=%d"
	REDOWNLOAD_URL      = INTERFACE_URL + "/redownload?callback=jsonp%d"
	SHOWCLASS_URL       = INTERFACE_URL + "/show_class?callback=jsonp%d&type_id=%d"
	MENUGET_URL         = INTERFACE_URL + "/menu_get"
	FILLBTLIST_URL      = INTERFACE_URL + "/fill_bt_list?callback=fill_bt_list&tid=%s&infoid=%s&g_net=1&p=1&uid=%s&interfrom=%s&noCacheIE=%d"
	TASKCHECK_URL       = INTERFACE_URL + "/task_check?callback=queryCid&url=%s&interfrom=task&random=%s&tcache=%d"
	TASKCOMMIT_URL      = INTERFACE_URL + "/task_commit?"
	BATCHTASKCOMMIT_URL = INTERFACE_URL + "/batch_task_commit?callback=jsonp%d"
	TORRENTUPLOAD_URL   = INTERFACE_URL + "/torrent_upload"
	BTTASKCOMMIT_URL    = INTERFACE_URL + "/bt_task_commit?callback=jsonp%d"
	URLQUERY_URL        = INTERFACE_URL + "/url_query?callback=queryUrl&u=%s&random=%s"
	DELAYONCE_URL       = INTERFACE_URL + "/delay_once?callback=anything"
	RENAME_URL          = INTERFACE_URL + "/rename?"
	TASKPROCESS_URL     = INTERFACE_URL + "/task_process?callback=jsonp%d&t=%d"
	TASKDELETE_URL      = INTERFACE_URL + "/task_delete?callback=jsonp%d&type=%d&noCacheIE=%d"
	SHOWTASK_UNFRESH    = INTERFACE_URL + "/showtask_unfresh?type_id=%d&page=%d&tasknum=%s&p=%d&interfrom=task"
)
