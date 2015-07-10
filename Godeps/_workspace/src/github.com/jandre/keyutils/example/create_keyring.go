package main

import (
	"log"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/jandre/keyutils"
)

//
// create a keyring, then add a key to it, and list the result.
//
func main() {

	keyRingName := "jen's keyring"

	keyring, err := keyutils.NewKeyRing(keyRingName, keyutils.KEY_SPEC_USER_KEYRING)

	if err != nil {
		log.Fatal("Error adding keyring:", err)
	}

	id, err := keyutils.AddKey(keyutils.USER, "ssh key", "ssh key secret data", keyring)

	if err != nil || id == 0 {
		log.Fatal("Error adding key:", err)
	}

	id, err = keyutils.AddKey(keyutils.USER, "password for Github", "my github password", keyring)

	if err != nil || id == 0 {
		log.Fatal("Error adding key:", err)
	}

	if keys, err := keyutils.ListKeysInKeyRing(keyring); err != nil {
		log.Fatal(err)
	} else {
		log.Printf("%s (%d keys):", keyRingName, len(keys))
		for id, key := range keys {
			log.Printf("-- #%d: %s (uid=%d, gid=%d)", id, key.Description, key.Uid, key.Gid)
		}
	}

}
