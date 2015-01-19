package cas

import (
	"encoding/json"
	"time"

	"github.com/appc/spec/schema"
)

// ACIInfo is used to store informations about an ACI
// Im is the ACI's image manifest. This avoids extracting it from the ACI every
// time it's needed
// BlobKey is the key in the blob store of the related ACI file.
// Time is the time this ACI was imported in the store
// Latest defines if the ACI was imported using the latest pattern (no version
// label provided on ACI discovery)
type ACIInfo struct {
	Im      *schema.ImageManifest
	BlobKey string
	Time    time.Time
	Latest  bool
}

func NewACIInfo(im *schema.ImageManifest, blobKey string, latest bool, t time.Time) *ACIInfo {
	return &ACIInfo{
		Im:      im,
		BlobKey: blobKey,
		Latest:  latest,
		Time:    t,
	}
}

func (a ACIInfo) Marshal() ([]byte, error) {
	return json.Marshal(a)
}

func (a *ACIInfo) Unmarshal(data []byte) error {
	return json.Unmarshal(data, a)
}

func (a ACIInfo) Hash() string {
	return a.BlobKey
}

func (a ACIInfo) Type() int64 {
	return aciInfoType
}
