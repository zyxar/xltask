package main

import (
	"flag"
	"fmt"
)

const (
	version = "0.1a"
)

var id string

type config struct {
	Account string `json:"account"`
}

var printVer bool

func printVersion() {
	fmt.Println("xltaskc version:", version)
}

func initConf() {
	flag.StringVar(&id, "login", "", "login account")
	flag.StringVar(&id, "l", "", "login account")
	flag.BoolVar(&printVer, "version", false, "print version")
	flag.Parse()
}
