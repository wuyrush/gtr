package bcodec

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/anacrolix/torrent/bencode"
)

// TODO update LSP gopls to recognize transition of interface{} -> any
// https://github.com/golang/go/commit/2580d0e08d5e9f979b943758d3c49877fb2324cb
// https://news.ycombinator.com/item?id=29557066

func TestBedecode(t *testing.T) {
	type C struct {
		name   string
		target bencode.Unmarshaler
		data   []byte
		verify func(t *testing.T, target bencode.Unmarshaler, err error)
	}
	tcs := []*C{
		{
			name:   "FileSpec",
			target: &FileSpec{},
			data:   []byte("d6:lengthi123e4:pathl3:foo3:bar7:qux.mp4ee"),
			verify: func(t *testing.T, target bencode.Unmarshaler, err error) {
				assert.Nil(t, err)
				f := target.(*FileSpec)
				assert.Equal(t, int64(123), f.LenBytes)
				assert.Equal(t, filepath.Join("foo", "bar", "qux.mp4"), f.Path)
			},
		},
		{
			name:   "FileSpec: nil pointer",
			target: (*FileSpec)(nil),
			data:   []byte(""),
			verify: func(t *testing.T, target bencode.Unmarshaler, err error) {
				assert.NotNil(t, err)
			},
		},
		{
			name:   "FileSpec: zero length path list",
			target: &FileSpec{},
			data:   []byte("d6:lengthi123ee"),
			verify: func(t *testing.T, target bencode.Unmarshaler, err error) {
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), "got zero-length path list for FileSpec")
			},
		},
		{
			name:   "FileSpec: invalid utf8 string in path list",
			target: &FileSpec{},
			data:   []byte("d6:lengthi123e4:pathl3:\xbd\xb2\xb9ee"),
			verify: func(t *testing.T, target bencode.Unmarshaler, err error) {
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), "path list segment for FileSpec is not valid UTF-8 string")
			},
		},
		{
			name:   "DhtNode",
			target: &DhtNode{},
			data:   []byte("l9:127.0.0.1i6881ee"),
			verify: func(t *testing.T, target bencode.Unmarshaler, err error) {
				assert.Nil(t, err)
				x := target.(*DhtNode)
				assert.Equal(t, "127.0.0.1", x.Host)
				assert.Equal(t, int64(6881), x.Port)
			},
		},
		{
			name:   "DhtNode: incomplete list",
			target: &DhtNode{},
			data:   []byte("l9:127.0.0.1e"),
			verify: func(t *testing.T, target bencode.Unmarshaler, err error) {
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), "inadequate info in anonymous list to represent a DhtNode")
			},
		},
		{
			name:   "TorrentInfo: single file",
			target: &TorrentInfo{},
			data:   []byte("d6:lengthi2048e4:name3:foo6:pieces40:\xbd\xf1\x3d\xff\x92\xe2\x8c\x98\xbd\xb2\x3d\xbc\x20\xe2\x8c\x98\xbd\xb2\x3d\xbc\xbd\xf1\x3d\xff\x92\xe2\x8c\x98\xbd\xb2\x3d\xbc\x20\xe2\x8c\x98\xbd\xb2\x3d\xbc12:piece lengthi1024ee"),
			verify: func(t *testing.T, target bencode.Unmarshaler, err error) {
				assert.Nil(t, err)
				x := target.(*TorrentInfo)
				assert.Equal(t, "foo", x.Name)
				assert.True(t, len(x.Hash) == 20)
				assert.Equal(t, int64(1024), x.PieceLenBytes)
				assert.Equal(
					t,
					[]byte("\xbd\xf1\x3d\xff\x92\xe2\x8c\x98\xbd\xb2\x3d\xbc\x20\xe2\x8c\x98\xbd\xb2\x3d\xbc\xbd\xf1\x3d\xff\x92\xe2\x8c\x98\xbd\xb2\x3d\xbc\x20\xe2\x8c\x98\xbd\xb2\x3d\xbc"),
					x.Pieces)
				assert.NotNil(t, x.LenBytes)
				assert.Equal(t, int64(2048), *x.LenBytes)
				assert.Nil(t, x.Files)
			},
		},
		{
			name:   "TorrentInfo: multiple files",
			target: &TorrentInfo{},
			data:   []byte("d5:filesld6:lengthi123e4:pathl3:foo3:bar7:qux.mp4eed6:lengthi456e4:pathl3:ham4:eggs7:hot.avieee4:name3:foo12:piece lengthi1024e6:pieces40:\xbd\xf1\x3d\xff\x92\xe2\x8c\x98\xbd\xb2\x3d\xbc\x20\xe2\x8c\x98\xbd\xb2\x3d\xbc\xbd\xf1\x3d\xff\x92\xe2\x8c\x98\xbd\xb2\x3d\xbc\x20\xe2\x8c\x98\xbd\xb2\x3d\xbce"),
			verify: func(t *testing.T, target bencode.Unmarshaler, err error) {
				assert.Nil(t, err)
				x := target.(*TorrentInfo)
				assert.Equal(t, "foo", x.Name)
				// assert.True(t, len(x.Hash) == 20)
				assert.Equal(t, []byte("\x7a\xe2\x52\xce\x0d\x5b\x2a\x2f\x7f\x01\x38\x76\x3b\x0e\xfe\x40\xd6\x6d\xb2\xe0"), x.Hash)
				assert.Equal(t, int64(1024), x.PieceLenBytes)
				assert.Equal(
					t,
					[]byte("\xbd\xf1\x3d\xff\x92\xe2\x8c\x98\xbd\xb2\x3d\xbc\x20\xe2\x8c\x98\xbd\xb2\x3d\xbc\xbd\xf1\x3d\xff\x92\xe2\x8c\x98\xbd\xb2\x3d\xbc\x20\xe2\x8c\x98\xbd\xb2\x3d\xbc"),
					x.Pieces)
				assert.Equal(
					t,
					[]*FileSpec{
						{LenBytes: 123, Path: filepath.Join("foo", "bar", "qux.mp4")},
						{LenBytes: 456, Path: filepath.Join("ham", "eggs", "hot.avi")},
					}, x.Files)
				assert.Nil(t, x.LenBytes)
			},
		},
		{
			name:   "TorrentInfo: corrupted pieces hash",
			target: &TorrentInfo{},
			data:   []byte("d6:lengthi2048e4:name3:foo6:pieces39:\xbd\xf1\x3d\xff\x92\xe2\x8c\x98\xbd\xb2\x3d\xbc\x20\xe2\x8c\x98\xbd\xb2\x3d\xbc\xbd\xf1\x3d\xff\x92\xe2\x8c\x98\xbd\xb2\x3d\xbc\x20\xe2\x8c\x98\xbd\xb2\x3d12:piece lengthi1024ee"),
			verify: func(t *testing.T, target bencode.Unmarshaler, err error) {
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), "TorrentInfo pieces byte string length is not a multiple of 20")
			},
		},
		{
			name:   "Torrent: minimal",
			target: &Torrent{},
			data:   []byte("d8:announce27:http://tracker.net/announce13:announce-listll23:udp://tracker1.net:688127:http://tracker.net/announceee4:infod5:filesld6:lengthi123e4:pathl3:foo3:bar7:qux.mp4eed6:lengthi456e4:pathl3:ham4:eggs7:hot.avieee4:name3:foo12:piece lengthi1024e6:pieces40:\xbd\xf1\x3d\xff\x92\xe2\x8c\x98\xbd\xb2\x3d\xbc\x20\xe2\x8c\x98\xbd\xb2\x3d\xbc\xbd\xf1\x3d\xff\x92\xe2\x8c\x98\xbd\xb2\x3d\xbc\x20\xe2\x8c\x98\xbd\xb2\x3d\xbcee"),
			verify: func(t *testing.T, target bencode.Unmarshaler, err error) {
				assert.Nil(t, err)
				tr := target.(*Torrent)
				assert.Equal(t, []string{"http://tracker.net/announce", "udp://tracker1.net:6881"}, tr.Trackers)
				assert.Equal(t, &Torrent{
					// tracker list is deduped
					Trackers: []string{"http://tracker.net/announce", "udp://tracker1.net:6881"},
					Info: &TorrentInfo{
						Name:          "foo",
						Hash:          []byte("\x7a\xe2\x52\xce\x0d\x5b\x2a\x2f\x7f\x01\x38\x76\x3b\x0e\xfe\x40\xd6\x6d\xb2\xe0"),
						PieceLenBytes: 1024,
						Pieces:        []byte("\xbd\xf1\x3d\xff\x92\xe2\x8c\x98\xbd\xb2\x3d\xbc\x20\xe2\x8c\x98\xbd\xb2\x3d\xbc\xbd\xf1\x3d\xff\x92\xe2\x8c\x98\xbd\xb2\x3d\xbc\x20\xe2\x8c\x98\xbd\xb2\x3d\xbc"),
						Files: []*FileSpec{
							{LenBytes: 123, Path: filepath.Join("foo", "bar", "qux.mp4")},
							{LenBytes: 456, Path: filepath.Join("ham", "eggs", "hot.avi")},
						},
					},
				}, tr)
			},
		},
	}
	for _, c := range tcs {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := bencode.Unmarshal(c.data, c.target)
			c.verify(t, c.target, err)
		})
	}
}

func TestBdecodeNestedStruct(t *testing.T) {
	b := []byte("ld6:lengthi123e4:pathl3:foo3:bar7:qux.mp4eee")
	var fss []*FileSpec
	if err := bencode.Unmarshal(b, &fss); err != nil {
		t.Error(err)
	}
	t.Logf("unmarshalled fss: %v", fss)
	assert.Equal(t, []*FileSpec{{LenBytes: 123, Path: filepath.Join("foo", "bar", "qux.mp4")}}, fss)
}
