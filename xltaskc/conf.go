package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/zyxar/xltask/xl"
	"io/ioutil"
	"path"
)

const (
	version = "0.1a"
)

var id string
var conf_file string

type config struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

func (c *config) load() error {
	co, err := ioutil.ReadFile(conf_file)
	if err != nil {
		return err
	}
	return json.Unmarshal(co, c)
}

func (c *config) save() error {
	r, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(conf_file, r, 0644)
}

func (c *config) GetId() string {
	return c.Account
}
func (c *config) GetPass() string {
	return c.Password
}
func (c *config) SetPass(pass string) error {
	c.Password = pass
	return nil
}

func (c *config) Boost() bool {
	return false
}

var printVer bool
var conf *config

func printVersion() {
	fmt.Println("xltaskc version:", version)
}

func initConf() {
	conf = &config{}
	flag.StringVar(&conf.Account, "login", "", "login account")
	flag.StringVar(&conf.Account, "l", "", "login account")
	flag.BoolVar(&printVer, "version", false, "print version")
	flag.Parse()
	conf_file = path.Join(xl.XLTASK_HOME, "config.json")
}
