package cas

import (
	"database/sql"
	"io/ioutil"
	"os"
	"testing"
)

func TestWriteACIInfo(t *testing.T) {
	dir, err := ioutil.TempDir("", tstprefix)
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	ds, err := NewStore(dir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)

	}
	if err = ds.db.Do(func(tx *sql.Tx) error {
		aciinfo := &ACIInfo{
			BlobKey: "key01",
			AppName: "name01",
		}
		if err := WriteACIInfo(tx, aciinfo); err != nil {
			return err
		}
		if err := WriteACIInfo(tx, aciinfo); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	aciinfos := []*ACIInfo{}
	ok := false
	if err = ds.db.Do(func(tx *sql.Tx) error {
		aciinfos, ok, err = GetACIInfosWithAppName(tx, "name01")
		return err
	}); err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if len(aciinfos) != 1 {
		t.Errorf("wrong number of records return, wanted: 1, got: %d", len(aciinfos))
	}
}
