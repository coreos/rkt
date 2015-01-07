package metadata

import "fmt"

const (
	MetadataSvcIP      = "169.254.169.255"
	MetadataSvcPubPort = 80
	MetadataSvcPrvPort = 2375
)

func MetadataSvcPrvURL() string {
	return fmt.Sprintf("http://127.0.0.1:%v", MetadataSvcPrvPort)
}

func MetadataSvcPubURL() string {
	return fmt.Sprintf("http://%v:%v", MetadataSvcIP, MetadataSvcPubPort)
}
