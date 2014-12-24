package cas

import (
	"encoding/json"

	"github.com/appc/spec/schema"
)

// AppIndex is needed to retrieve all acis matching an app name.
// ACIInfoKey is the key of the related entry in the ACIInfo index.
type AppIndex struct {
	ACIInfoKey string
	im         *schema.ImageManifest
}

func NewAppIndex(im *schema.ImageManifest, aciInfoKey string) *AppIndex {
	a := &AppIndex{
		im:         im,
		ACIInfoKey: aciInfoKey,
	}
	return a
}

func (a AppIndex) Marshal() ([]byte, error) {
	return json.Marshal(a)
}

func (a *AppIndex) Unmarshal(data []byte) error {
	return json.Unmarshal(data, a)
}

// TODO Also this should probably be implemented with a db.
// The key is a composition of the name hash + "-" + ACIInfoKey
// In this way we can retrieve from the store all the aci with the same name
func (a AppIndex) Hash() string {
	return ShortSHA512(a.im.Name.String()) + "-" + a.ACIInfoKey
}

func (a AppIndex) Type() int64 {
	return appIndexType
}
