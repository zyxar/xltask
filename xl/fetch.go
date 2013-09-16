package xl

import (
	"log"
	"os"
	"os/exec"
)

type Fetcher interface {
	Fetch(uri, gdriveid, filename string, echo bool) error
}

type wget struct {
}

type curl struct {
}

type Aria2 struct {
}

type axel struct {
}

var DefaultFetcher Fetcher

func init() {
	DefaultFetcher = &Aria2{}
}

func (w wget) Fetch(uri, gdriveid, filename string, echo bool) error {
	cmd := exec.Command("wget", "--header=Cookie: gdriveid="+gdriveid, uri, "-O", filename)
	if echo {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	err := cmd.Start()
	if err != nil {
		log.Println(err)
	}
	return cmd.Wait()
}

func (c curl) Fetch(uri, gdriveid, filename string, echo bool) error {
	cmd := exec.Command("curl", "-L", uri, "--cookie", "gdriveid="+gdriveid, "--output", filename)
	if echo {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	err := cmd.Start()
	if err != nil {
		log.Println(err)
	}
	return cmd.Wait()
}

func (a Aria2) Fetch(uri, gdriveid, filename string, echo bool) error {
	cmd := exec.Command("aria2c", "--header=Cookie: gdriveid="+gdriveid, uri, "--out", filename, "--file-allocation=none", "-s5", "-x5", "-c", "--summary-interval=0", "--follow-torrent=false")
	if echo {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	err := cmd.Start()
	if err != nil {
		log.Println(err)
	}
	return cmd.Wait()
}

func (a axel) Fetch(uri, gdriveid, filename string, echo bool) error {
	cmd := exec.Command("axel", "--header=Cookie: gdriveid="+gdriveid, uri, "--output", filename)
	if echo {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	err := cmd.Start()
	if err != nil {
		log.Println(err)
	}
	return cmd.Wait()
}
