package aci

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"

	"github.com/coreos/rocket/Godeps/_workspace/src/golang.org/x/crypto/openpgp"
)

// TODO(jonboulle): support detached signatures

// LoadSignedData reads PGP encrypted data from the given Reader, using the
// provided keyring (EntityList). The entire decrypted bytestream is
// returned, and/or any error encountered.
// TODO(jonboulle): support symmetric decryption
func LoadSignedData(signed io.Reader, kr openpgp.EntityList) ([]byte, error) {
	md, err := openpgp.ReadMessage(signed, kr, nil, nil)
	if err != nil {
		return nil, err
	}
	if md.IsSymmetricallyEncrypted {
		return nil, errors.New("symmetric encryption not yet supported")
	}

	// Signature cannot be verified until body is read
	data, err := ioutil.ReadAll(md.UnverifiedBody)
	if err != nil {
		return nil, fmt.Errorf("error reading body: %v", err)
	}
	if md.IsSigned && md.SignedBy != nil {
		// Once EOF has been seen, the following fields are
		// valid. (An authentication code failure is reported as a
		// SignatureError error when reading from UnverifiedBody.)
		//
		if md.SignatureError != nil {
			return nil, fmt.Errorf("signature error: %v", md.SignatureError)
		}
		log.Println("message signature OK")
	}
	return data, nil
}
