package bcodec

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
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
	// available tags can be found at https://github.com/anacrolix/torrent/blob/master/bencode/tags.go
	tmp := struct {
		Info              *TorrentInfo `bencode:"info"`
		Announce          string       `bencode:"announce"`
		AnnounceList      [][]string   `bencode:"announce-list,omitempty"`
		Comment           *string      `bencode:"comment,omitempty"`
		CreationTimestamp *int64       `bencode:"creation date,omitempty"`
		HttpSeeds         []string     `bencode:"httpseeds,omitempty"`
		DhtNode           []*DhtNode   `bencode:"nodes,omitempty"`
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
	// TODO make this always available as the total length of the file
	LenBytes int64
	Files    []*FileSpec
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
	x.Files = tmp.Files
	// TODO verify that value of length field / aggregated length value of all files involved is identical to
	// the product of piece length and # piece hashes present in torrent file, instead of blindly trust the
	// length values
	if tmp.LenBytes != nil {
		// overflow?
		if *tmp.LenBytes < 0 {
			return fmt.Errorf("got negative total size of file content")
		}
		x.LenBytes = *tmp.LenBytes
	} else {
		totalBytes, err := totalFileSizeBytes(x.Files)
		if err != nil {
			return err
		}
		x.LenBytes = totalBytes
	}
	// compute info hash. hash.Hash.Write() never return an error
	h := sha1.New()
	_, _ = h.Write(raw)
	buf := make([]byte, 0, 20)
	x.Hash = h.Sum(buf)
	return nil
}

func totalFileSizeBytes(files []*FileSpec) (int64, error) {
	var res int64 = 0
	for _, f := range files {
		res += f.LenBytes
		// overflow?
		if res < 0 {
			return 0, fmt.Errorf("int64 overflow when computing total size of file content")
		}
	}
	return res, nil
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
			return fmt.Errorf("path list segment for FileSpec is not valid UTF-8 string: %w", err)
		}
	}
	// all path segments are valid UTF-8 encoded strings, create os specific file path
	x.Path = filepath.Join(tmp.Path...)
	x.LenBytes = tmp.LenBytes
	return nil
}

type TrackerRsp struct {
	FailureReason *string
	WarningMsg    *string
	PollInterval  *time.Duration
	TrackerID     *string
	SeederCnt     *int
	LeecherCnt    *int
	PeerAddrs     PeerAddrs
}

func (x *TrackerRsp) UnmarshalBencode(raw []byte) error {
	tmp := struct {
		FailureReason          *string   `bencode:"failure reason,omitempty"`
		WarningMsg             *string   `bencode:"warning message,omitempty"`
		PollIntervalSeconds    *int64    `bencode:"interval,omitempty"`
		MinPollIntervalSeconds *int64    `bencode:"min interval,omitempty"`
		TrackerID              *string   `bencode:"tracker id,omitempty"`
		SeederCnt              *int      `bencode:"complete,omitempty"`
		LeecherCnt             *int      `bencode:"incomplete,omitempty"`
		PeerAddrs              PeerAddrs `bencode:"peers,omitempty"`
	}{}
	if err := bencode.Unmarshal(raw, &tmp); err != nil {
		return fmt.Errorf("error decoding anonymous struct for TrackerRsp: %w", err)
	}
	if ptr := tmp.FailureReason; ptr != nil {
		if err := validateUtf8Str(*ptr); err != nil {
			return fmt.Errorf("failure reason presents in tracker response but is invalid UTF-8 sring: %w", err)
		}
		x.FailureReason = ptr
		// no need to parse other attributes as they will be absent in failure case
		return nil
	}
	if ptr := tmp.WarningMsg; ptr != nil {
		if err := validateUtf8Str(*ptr); err != nil {
			return fmt.Errorf("warning message presents in tracker response but is invalid UTF-8 sring: %w", err)
		}
		x.WarningMsg = ptr
	}
	var pollIntervalSeconds int64 = 0
	if ptr := tmp.PollIntervalSeconds; ptr != nil {
		pollIntervalSeconds = *ptr
	}
	if ptr := tmp.MinPollIntervalSeconds; ptr != nil {
		pollIntervalSeconds = *ptr
	}
	duration := time.Duration(pollIntervalSeconds) * time.Second
	x.PollInterval = &duration
	x.TrackerID = tmp.TrackerID
	x.SeederCnt = tmp.SeederCnt
	x.LeecherCnt = tmp.LeecherCnt
	x.PeerAddrs = tmp.PeerAddrs
	return nil
}

// peer address in form of concatenation of hostname and port
type PeerAddrs []string

// bittorrent peer address. NOTE only use this in bdecode
//type PeerAddr struct {
//	// peer ID seems useless so exclude it for now
//	Hostname string
//	Port     int
//}

func (x *PeerAddrs) UnmarshalBencode(raw []byte) error {
	// parse peer list in binary mode first as this is preferred by trackers, if parsing encountered error
	// then continue parsing in list-of-dictionary mode
	// for a peer list to be in binary mode the length of decoded peer list string must be divisible by 6.
	if peersStr := ""; bencode.Unmarshal(raw, &peersStr) == nil && len(peersStr)%6 == 0 {
		// possibly binary mode
		var tmp []string
		err := false
		ln := len(peersStr)
		for idx := 0; idx < ln; idx += 6 {
			// read ip address in ipv4 format. TODO see how endian-ness can impact parsing result
			hostname := fmt.Sprintf("%d.%d.%d.%d", peersStr[idx], peersStr[idx+1], peersStr[idx+2], peersStr[idx+3])
			if net.ParseIP(hostname) == nil {
				fmt.Fprintf(os.Stderr, "error parsing ip address in peer list: %s peer list may be in list-of-dictionary mode", hostname)
				err = true
				break
			}
			port := int(binary.BigEndian.Uint16([]byte(peersStr[idx+4 : idx+6])))
			tmp = append(tmp, net.JoinHostPort(hostname, strconv.Itoa(port)))
		}
		if !err {
			*x = tmp
			return nil
		}
		// otherwise proceed to parsing via list-of-dictionary mode
	} else if peersStr != "" {
		// here the raw data represents a (malformed) bencoded string instead of dictionary
		return fmt.Errorf("malformed peer list in binary mode: decoded peer list string doesn't have length divisible by 6")
	}
	type PeerAddr struct {
		Hostname string `bencode:"ip"`
		Port     int    `bencode:"port"`
	}
	tmp := []*PeerAddr{}
	if err := bencode.Unmarshal(raw, &tmp); err != nil {
		// raw is either malformed or encoded in binary mode
		return fmt.Errorf("error decoding peer list in both binary and list-of-dictionary mode: %w", err)
	}
	for _, a := range tmp {
		// TODO validation against hostname and port
		*x = append(*x, net.JoinHostPort(a.Hostname, strconv.Itoa(a.Port)))
	}
	return nil
}
