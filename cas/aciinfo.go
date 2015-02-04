package cas

import (
	"database/sql"
	"time"
)

// ACIInfo is used to store informations about an ACI
// BlobKey is the key in the blob and imageManifest store of the related ACI file.
// AppName is the app name provided by the ACI
// ImportTime is the time this ACI was imported in the store
// Latest defines if the ACI was imported using the latest pattern (no version
// label provided on ACI discovery)
type ACIInfo struct {
	BlobKey    string
	AppName    string
	ImportTime time.Time
	Latest     bool
}

func NewACIInfo(blobKey string, latest bool, t time.Time) *ACIInfo {
	return &ACIInfo{
		BlobKey:    blobKey,
		Latest:     latest,
		ImportTime: t,
	}
}

// GetAciInfo tries to retrieve an aciinfo with the given blobkey. found will be
// false if not aciinfo exists
func getACIInfos(tx *sql.Tx, where string, args ...interface{}) ([]*ACIInfo, bool, error) {
	aciinfos := []*ACIInfo{}
	found := false
	rows, err := tx.Query("SELECT * from aciinfo WHERE "+where, args...)
	if err != nil {
		return nil, false, err
	}
	for rows.Next() {
		found = true
		aciinfo := &ACIInfo{}
		if err := rows.Scan(&aciinfo.BlobKey, &aciinfo.AppName, &aciinfo.ImportTime, &aciinfo.Latest); err != nil {
			return nil, false, err
		}
		aciinfos = append(aciinfos, aciinfo)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	return aciinfos, found, err
}

func GetACIInfosWithAppName(tx *sql.Tx, appname string) ([]*ACIInfo, bool, error) {
	return getACIInfos(tx, "appname == $1", appname)
}

// WriteACIInfo adds or updates the provided aciinfo.
func WriteACIInfo(tx *sql.Tx, aciinfo *ACIInfo) error {
	// ql doesn't have an INSERT OR UPDATE function so
	// it's faster to remove and reinsert the row
	_, err := tx.Exec("DELETE from aciinfo where blobkey == $1", aciinfo.BlobKey)
	if err != nil {
		return err
	}
	_, err = tx.Exec("INSERT into aciinfo values ($1, $2, $3, $4)", aciinfo.BlobKey, aciinfo.AppName, aciinfo.ImportTime, aciinfo.Latest)
	if err != nil {
		return err
	}

	return nil
}
