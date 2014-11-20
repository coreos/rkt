package types

import (
	"encoding/json"
	"errors"
	"fmt"
)

// TODO(jonboulle): flesh this out

type File struct {
	Hash          *Hash             `json:"hash,omitempty"`
	Type          FileType          `json:"type"`
	Mode          FileMode          `json:"mode"`
	Uid           uint32            `json:"uid"`
	Gid           uint32            `json:"gid"`
	Xattrs        map[string]string `json:"xattrs,omitempty"`
	Mtime         Date              `json:"mtime"`
	Ctime         Date              `json:"ctime"`
	DevMajor      *DevNumber         `json:"devMajor,omitempty"`
	DevMinor      *DevNumber         `json:"devMinor,omitempty"`
	SymlinkTarget string            `json:"symlinkTarget,omitempty"`
}

func (f File) assertValid() error {
	if f.DevMajor != nil || f.DevMinor != nil {
		switch {
		case f.DevMajor == nil:
			return errors.New("both or neither of DevMinor and DevMajor must be set")
		case f.DevMinor ==  nil:
			return errors.New("both or neither of DevMinor and DevMajor must be set")
		case f.Type != "char" && f.Type != "block":
			return errors.New("FileType must be 3 or 4 if DevMajor/DevMinor are set")
		default:
		}
	}
	return nil
}

type file File

func (f File) MarshalJSON() ([]byte, error) {
	if err := f.assertValid(); err != nil {
		return nil, err
	}
	return json.Marshal(file(f))
}

func (f *File) UnmarshalJSON(data []byte) error {
	var jf file
	if err := json.Unmarshal(data, &jf); err != nil {
		return err
	}
	nf := File(jf)
	if err := nf.assertValid(); err != nil {
		return err
	}
	*f = nf
	return nil
}

type FileType string

func (t FileType) String() string {
	return string(t)
}

func (t FileType) assertValid() error {
	s := t.String()
	switch s {
	case "directory", "file", "char", "block", "symlink", "fifo":
		return nil
	case "":
		return errors.New("FileType must be set")
	default:
		msg := fmt.Sprintf("bad FileType: %s", s)
		return errors.New(msg)
	}
}

func (t FileType) MarshalJSON() ([]byte, error) {
	if err := t.assertValid(); err != nil {
		return nil, err
	}
	return json.Marshal(t.String())
}

func (t *FileType) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}
	ft := FileType(s)
	if err := ft.assertValid(); err != nil {
		return err
	}
	*t = ft
	return nil
}

// TODO(jonboulle): sanity check me
type FileMode string

// TODO(jonboulle): sanity check me
type DevNumber string

/*

   "files": {
       "/usr/bin/ls": {
           "hash": "sha1-3e86b59982e49066c5d813af1c2e2579cbf573de",
           "type": "0",
           "mode": "0755",
           "uid": 1000,
           "gid": 500,
           "xattrs": {
               "foo": "bar"
           },
           "mtime": "2010-07-10T17:14:51.5678Z",
           "ctime": "2010-07-10T17:20:03.9212Z",
           "devMajor": "",
           "devMinor": "",
           "symlinkTarget": ""
       },
*/
