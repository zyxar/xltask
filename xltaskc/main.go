package main

import (
	"../xl"
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
)

var id string

type config struct {
	Account string `json:"account"`
}

func init() {
	flag.StringVar(&id, "login", "", "login account")
	flag.StringVar(&id, "l", "", "login account")
}

func clearscr() {
	fmt.Printf("%c[2J%c[0;0H", 27, 27)
}

func prompt() {
	fmt.Print("lixian >> ")
}

func main() {
	flag.Parse()
	agent := xl.NewAgent()
	if !agent.On {
		if id == "" {
			co, err := ioutil.ReadFile("config.json")
			if err != nil {
				flag.Usage()
				return
			}
			var v config
			json.Unmarshal(co, &v)
			id = v.Account
		}
		agent.Login(id)
	}
	{
		insufficientArgErr := errors.New("Insufficient arguments.")
		clearscr()
		rd := bufio.NewReader(os.Stdin)
		var err error
		var line string
		var cmds []string
		exp := regexp.MustCompile(`\s+`)
	LOOP:
		for {
			prompt()
			line, err = rd.ReadString('\n')
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
				if len(cmds) >= 2 {
					j := 1
					for j < len(cmds) {
						err = agent.Download(cmds[j], nil, true)
						if err != nil {
							fmt.Println(err)
						}
						j++
					}
					err = nil
				} else {
					err = insufficientArgErr
				}
			case "add":
				if len(cmds) >= 2 {
					j := 1
					for j < len(cmds) {
						err = agent.AddTask(cmds[1])
						if err != nil {
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
					err = agent.DeleteTask(cmds[1])
				} else if len(cmds) > 2 {
					err = agent.DeleteTasks(cmds[1:])
				} else {
					err = insufficientArgErr
				}
			case "purge":
				if len(cmds) == 2 {
					err = agent.PurgeTask(cmds[1])
				} else {
					err = insufficientArgErr
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
