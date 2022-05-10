package bcodec

import (
	"crypto/sha1"
	"fmt"
	"path/filepath"
	"time"
	"unicode/utf8"

	"github.com/anacrolix/torrent/bencode"
)

// TODO fill in missing fields and corresponding b-decode logic

// meta info file content
type Torrent struct {
	Info         *TorrentInfo
	Trackers     []string
	Comment      *string
	CreationDate *time.Time
	HttpSeeds    []string
	DhtNodes     []*DhtNode
}

func (x *Torrent) UnmarshalBencode(raw []byte) error {
	// TODO should we worry about GC? of anonymous struct like such?
	tmp := struct {
		Info              *TorrentInfo `bencode:"info"`
		Announce          string       `bencode:"announce"`
		AnnounceList      [][]string   `bencode:"announce-list"`
		Comment           *string      `bencode:"comment"`
		CreationTimestamp *int64       `bencode:"creation date"`
		HttpSeeds         []string     `bencode:"httpseeds"`
		DhtNode           []*DhtNode   `bencode:"nodes"`
	}{}
	if err := bencode.Unmarshal(raw, &tmp); err != nil {
		return fmt.Errorf("error decoding anonymous struct for Torrent: %w", err)
	}
	// validate that all text strings are valid UTF-8 encoded strings
	if tmp.Comment != nil {
		if err := validateUtf8Str(*tmp.Comment); err != nil {
			return fmt.Errorf("Torrent comment is invalid UTF-8 string: %w", err)
		}
	}
	for _, s := range tmp.HttpSeeds {
		if err := validateUtf8Str(s); err != nil {
			return fmt.Errorf("url in Torrent http seeds is invalid UTF-8 string: %w", err)
		}
	}
	if err := validateUtf8Str(tmp.Announce); err != nil {
		return fmt.Errorf("Torrent announce url is invalid UTF-8 string: %w", err)
	}
	// collect unique trackers
	visited := map[string]struct{}{
		tmp.Announce: {},
	}
	uniq_trackers := []string{tmp.Announce}
	for _, ls := range tmp.AnnounceList {
		for _, s := range ls {
			if err := validateUtf8Str(s); err != nil {
				return fmt.Errorf("url in Torrent announce-list is invalid UTF-8 string: %w", err)
			}
			if _, ok := visited[s]; !ok {
				uniq_trackers = append(uniq_trackers, s)
				visited[s] = struct{}{}
			}
		}
	}
	// convert creation date timestamp
	if tmp.CreationTimestamp != nil {
		t := time.Unix(*tmp.CreationTimestamp, 0)
		x.CreationDate = &t
	}
	x.Info = tmp.Info
	x.Trackers = uniq_trackers
	x.Comment = tmp.Comment
	x.HttpSeeds = tmp.HttpSeeds
	x.DhtNodes = tmp.DhtNode
	return nil
}

func validateUtf8Str(s string) error {
	if !utf8.ValidString(s) {
		return fmt.Errorf("found invalid UTF-8 string. Bytes: %v", []byte(s))
	}
	return nil
}

// meta info dictionary content
type TorrentInfo struct {
	Name          string
	Hash          []byte // info dictionary sha1 hash
	PieceLenBytes int64
	Pieces        []byte
	LenBytes      *int64
	Files         []*FileSpec
}

func (x *TorrentInfo) UnmarshalBencode(raw []byte) error {
	// attention: no way unmarshal the struct directly, otherwise we run into infinite recursion and stack overflow
	tmp := struct {
		Name          string      `bencode:"name"`
		PieceLenBytes int64       `bencode:"piece length"`
		Pieces        string      `bencode:"pieces"`
		LenBytes      *int64      `bencode:"length,omitempty"`
		Files         []*FileSpec `bencode:"files,omitempty"`
	}{}
	if err := bencode.Unmarshal(raw, &tmp); err != nil {
		return fmt.Errorf("error decoding TorrentInfo struct: %w", err)
	}
	// validate value of name key is valid UTF-8 encoded string
	if err := validateUtf8Str(tmp.Name); err != nil {
		return fmt.Errorf("TorrentInfo name is not valid UTF-8 string: %w", err)
	}
	if len(tmp.Pieces)%20 != 0 {
		return fmt.Errorf("TorrentInfo pieces byte string length is not a multiple of 20")
	}
	x.Name = tmp.Name
	x.PieceLenBytes = tmp.PieceLenBytes
	x.Pieces = []byte(tmp.Pieces)
	x.LenBytes = tmp.LenBytes
	x.Files = tmp.Files
	// compute info hash. hash.Hash.Write() never return an error
	h := sha1.New()
	h.Write(raw)
	buf := make([]byte, 0, 20)
	x.Hash = h.Sum(buf)
	return nil
}

type DhtNode struct {
	Host string
	Port int64
}

func (x *DhtNode) UnmarshalBencode(raw []byte) error {
	var i interface{}
	if err := bencode.Unmarshal(raw, &i); err != nil {
		return fmt.Errorf("error decoding anonymous list for DhtNode: %w", err)
	}
	tmp := i.([]interface{})
	if len(tmp) != 2 {
		return fmt.Errorf("inadequate info in anonymous list to represent a DhtNode: %v", tmp)
	}
	if host, ok := tmp[0].(string); !ok {
		return fmt.Errorf("invalid DhtNode hostname: %v", tmp[0])
	} else {
		x.Host = host
	}
	if port, ok := tmp[1].(int64); !ok {
		return fmt.Errorf("invalid DhtNode port: %v", tmp[1])
	} else {
		x.Port = port
	}
	return nil
}

type FileSpec struct {
	LenBytes int64
	Path     string
}

func (x *FileSpec) UnmarshalBencode(raw []byte) error {
	tmp := struct {
		LenBytes int64    `bencode:"length"`
		Path     []string `bencode:"path"`
	}{}
	if err := bencode.Unmarshal(raw, &tmp); err != nil {
		return fmt.Errorf("error decoding anonymous struct for FileSpec: %w", err)
	}
	if len(tmp.Path) == 0 {
		return fmt.Errorf("got zero-length path list for FileSpec")
	}
	for _, s := range tmp.Path {
		if err := validateUtf8Str(s); err != nil {
			return fmt.Errorf("path list segment for FileSpec is not valid UTF-8 string. Bytes: %v", []byte(s))
		}
	}
	// all path segments are valid UTF-8 encoded strings, create os specific file path
	x.Path = filepath.Join(tmp.Path...)
	x.LenBytes = tmp.LenBytes
	return nil
}
