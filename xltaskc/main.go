package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/zyxar/xltask/xl"
	"regexp"
	"strings"
)

type Term interface {
	ReadLine() (string, error)
	Restore()
}

func main() {
	initConf()
	if printVer {
		printVersion()
		return
	}
	term := newTerm()
	defer term.Restore()
	agent := xl.NewAgent(conf)
	var err error
	if err = agent.Login(); err == nil {
		conf.Password = xl.EncryptPass(conf.Password)
		conf.save()
	} else if err == xl.ReuseSession {
		fmt.Println(err)
	} else {
		if conf.Account == "" {
			err = conf.load()
			if err != nil || conf.Account == "" {
				flag.Usage()
				return
			}
		}
		if err = agent.Login(); err != nil {
			fmt.Println(err)
			return
		}
		conf.Password = xl.EncryptPass(conf.Password)
		conf.save()
	}
	{
		insufficientArgErr := errors.New("Insufficient arguments.")
		clearscr()
		var err error
		var line string
		var cmds []string
		exp := regexp.MustCompile(`\s+`)
	LOOP:
		for {
			line, err = term.ReadLine()
			if err != nil {
				break
			}
			line = strings.TrimSpace(line)
			line = exp.ReplaceAllString(line, " ")
			if line == "" {
				continue
			}
			cmds = strings.Split(line, " ")
			switch cmds[0] {
			case "cls":
				fallthrough
			case "clear":
				clearscr()
			case "show":
				fallthrough
			case "ls":
				agent.ShowTasks()
			case "ld":
				agent.ShowDeletedTasks(true)
			case "le":
				agent.ShowExpiredTasks(true)
			case "ll":
				fallthrough
			case "info":
				if len(cmds) >= 2 {
					agent.InfoTasks(cmds[1:])
				} else {
					err = insufficientArgErr
				}
			case "dl":
				fallthrough
			case "download":
				if len(cmds) < 2 {
					err = insufficientArgErr
				} else {
					tasks := make(map[string]string)
					for i, _ := range cmds[1:] {
						p := strings.Split(cmds[1:][i], "/")
						var filter string
						if len(p) == 1 {
							filter = `.*`
						} else {
							filter = p[1]
						}
						ts, _ := agent.Dispatch(p[0], 0)
						for j, _ := range ts {
							tasks[ts[j]] = filter
						}
					}
					if len(tasks) == 0 {
						err = errors.New("No task matches.")
					} else {
						for i, _ := range tasks {
							if err = agent.Download(i, tasks[i], nil, true); err != nil {
								fmt.Println(err)
							}
						}
						err = nil
					}
				}
			case "add":
				if len(cmds) >= 2 {
					j := 1
					for j < len(cmds) {
						if err = agent.AddTask(cmds[j]); err != nil {
							fmt.Println(err)
						}
						j++
					}
					err = nil
				} else {
					err = insufficientArgErr
				}
			case "rm":
				fallthrough
			case "delete":
				if len(cmds) == 2 {
					if tasks, err := agent.Dispatch(cmds[1], 2); err == nil {
						switch len(tasks) {
						case 0:
							err = errors.New("No task matches.")
						case 1:
							err = agent.DeleteTask(tasks[0])
						default:
							err = agent.DeleteTasks(tasks)
						}
					}
				} else if len(cmds) > 2 {
					err = agent.DeleteTasks(cmds[1:])
				} else {
					err = insufficientArgErr
				}
			case "purge":
				if len(cmds) < 2 {
					err = insufficientArgErr
				} else {
					var tasks []string
					if len(cmds) == 2 {
						tasks, err = agent.Dispatch(cmds[1], 2)
					} else {
						tasks = cmds[1:]
					}
					if len(tasks) == 0 {
						err = errors.New("No task matches.")
					} else {
						for j, _ := range tasks {
							if err = agent.PurgeTask(tasks[j]); err != nil {
								fmt.Println(err)
							}
						}
						err = nil
					}
				}
			case "readd":
				// re-add tasks from deleted or expired
			case "pause":
				if len(cmds) > 1 {
					err = agent.PauseTasks(cmds[1:])
				} else {
					err = insufficientArgErr
				}
			case "restart":
				// restart paused tasks
			case "rename":
				fallthrough
			case "mv":
				if len(cmds) == 3 {
					err = agent.RenameTask(cmds[1], cmds[2])
				} else {
					err = insufficientArgErr
				}
			case "delay":
				if len(cmds) == 2 {
					err = agent.DelayTask(cmds[1])
				} else {
					err = insufficientArgErr
				}
			case "link":
				// get lixian_URL of a task
			case "dispatch":
				if len(cmds) == 2 {
					_, err = agent.Dispatch(cmds[1], 2)
				} else {
					err = insufficientArgErr
				}
			case "version":
				printVersion()
			case "update":
				err = agent.ProcessTask()
			case "quit":
				fallthrough
			case "exit":
				break LOOP
			default:
				err = fmt.Errorf("Unrecognised command: %s", cmds[0])
			}
			if err != nil {
				fmt.Println(err)
			}
		}
	}
}
