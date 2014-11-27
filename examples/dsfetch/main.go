package store

import (
	"fmt"
	"log"
	"os"

	"github.com/coreos-inc/rkt/store"
)

func main() {
	ds := NewStore(".")
	r := NewRemote(os.Args[1], nil)
	err := ds.Get(r)
	if err != nil && r.File == "" {
		fmt.Println("Cache miss, downloading")
		r, err = r.Download(*ds)
		if err != nil {
			log.Fatalf("downloading: %v", err)
		}
	}
	out, err := ds.stores[objectType].Read(r.File)
	if err != nil {
		log.Fatalf("get: %v", err)
	}
	fmt.Printf("%v\n\n", string(out[:255]))
	ds.Dump(true)
}
