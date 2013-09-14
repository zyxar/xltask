package bt

import (
	"bytes"
	"code.google.com/p/bencode-go"
	"crypto/sha1"
	"errors"
	"io"
	"os"
)

type FileDict struct {
	Length int64
	Path   []string
	Md5sum string
}

type InfoDict struct {
	PieceLength int64 "piece length"
	Pieces      string
	Private     int64
	Name        string
	// Single File Mode
	Length int64
	Md5sum string
	// Multiple File mode
	Files []FileDict
}

type MetaInfo struct {
	Info         InfoDict
	InfoHash     string
	Announce     string
	CreationDate string "creation date"
	Comment      string
	CreatedBy    string "created by"
	Encoding     string
}

func getString(m map[string]interface{}, k string) string {
	if v, ok := m[k]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func GetMetaInfo(torrent string) (metaInfo *MetaInfo, err error) {
	var input io.ReadCloser
	if input, err = os.Open(torrent); err != nil {
		return
	}

	// We need to calcuate the sha1 of the Info map, including every value in the
	// map. The easiest way to do this is to read the data using the Decode
	// API, and then pick through it manually.
	var m interface{}
	m, err = bencode.Decode(input)
	input.Close()
	if err != nil {
		err = errors.New("Couldn't parse torrent file phase 1: " + err.Error())
		return
	}

	topMap, ok := m.(map[string]interface{})
	if !ok {
		err = errors.New("Couldn't parse torrent file phase 2.")
		return
	}

	infoMap, ok := topMap["info"]
	if !ok {
		err = errors.New("Couldn't parse torrent file. info")
		return
	}
	var b bytes.Buffer
	if err = bencode.Marshal(&b, infoMap); err != nil {
		return
	}
	hash := sha1.New()
	hash.Write(b.Bytes())

	var m2 MetaInfo
	err = bencode.Unmarshal(&b, &m2.Info)
	if err != nil {
		return
	}

	m2.InfoHash = string(hash.Sum(nil))
	m2.Announce = getString(topMap, "announce")
	m2.CreationDate = getString(topMap, "creation date")
	m2.Comment = getString(topMap, "comment")
	m2.CreatedBy = getString(topMap, "created by")
	m2.Encoding = getString(topMap, "encoding")

	metaInfo = &m2
	return
}
