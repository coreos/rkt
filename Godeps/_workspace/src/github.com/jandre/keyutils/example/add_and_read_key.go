package main

import (
	"log"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/jandre/keyutils"
)

func main() {
	id, err := keyutils.AddKey(keyutils.USER, "test123", "hello", keyutils.KEY_SPEC_USER_KEYRING)

	if err != nil {
		log.Fatal("Error adding key:", err)
	}
	log.Println("Added key test123 with serial:", id)
	val, err := keyutils.ReadKey(id)

	if err != nil {
		log.Fatal("Error reading key:", err)
	}

	log.Println("Read:", val)
}
