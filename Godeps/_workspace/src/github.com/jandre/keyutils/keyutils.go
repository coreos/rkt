//
// keyutils provides libkeyutils bindings for Go.
//
// To build, it requires libkeyutils binaries and headers, e.g..
// apt-get install libkeyutils-dev
//
package keyutils

/*
#cgo linux LDFLAGS: -lkeyutils

#include <errno.h>
#include <stddef.h>
#include <stdlib.h>
#include <string.h>
#include <keyutils.h>
*/
import "C"
import (
	"bytes"
	"encoding/binary"
	"errors"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

type KeySerial int32
type KeyType string
type KeyPerm int

type KeyDesc struct {
	Serial      KeySerial
	Type        KeyType
	Uid         uint
	Gid         uint
	Permissions uint
	Description string
}

const (
	USER    KeyType = "user"
	KEYRING KeyType = "keyring"
)

const (
	KEY_SPEC_THREAD_KEYRING       KeySerial = KeySerial(C.KEY_SPEC_THREAD_KEYRING)
	KEY_SPEC_USER_KEYRING                   = KeySerial(C.KEY_SPEC_USER_KEYRING)
	KEY_SPEC_PROCESS_KEYRING                = KeySerial(C.KEY_SPEC_PROCESS_KEYRING)
	KEY_SPEC_SESSION_KEYRING                = KeySerial(C.KEY_SPEC_SESSION_KEYRING)
	KEY_SPEC_USER_SESSION_KEYRING           = KeySerial(C.KEY_SPEC_USER_SESSION_KEYRING)
)

const (
	KEY_POS_VIEW    = KeyPerm(C.KEY_POS_VIEW)
	KEY_POS_READ    = KeyPerm(C.KEY_POS_READ)
	KEY_POS_WRITE   = KeyPerm(C.KEY_POS_WRITE)
	KEY_POS_SEARCH  = KeyPerm(C.KEY_POS_SEARCH)
	KEY_POS_LINK    = KeyPerm(C.KEY_POS_LINK)
	KEY_POS_SETATTR = KeyPerm(C.KEY_POS_SETATTR)
	KEY_POS_ALL     = KeyPerm(C.KEY_POS_ALL)

	KEY_USR_VIEW    = KeyPerm(C.KEY_USR_VIEW)
	KEY_USR_READ    = KeyPerm(C.KEY_USR_READ)
	KEY_USR_WRITE   = KeyPerm(C.KEY_USR_WRITE)
	KEY_USR_SEARCH  = KeyPerm(C.KEY_USR_SEARCH)
	KEY_USR_LINK    = KeyPerm(C.KEY_USR_LINK)
	KEY_USR_SETATTR = KeyPerm(C.KEY_USR_SETATTR)
	KEY_USR_ALL     = KeyPerm(C.KEY_USR_ALL)

	KEY_GRP_VIEW    = KeyPerm(C.KEY_GRP_VIEW)
	KEY_GRP_READ    = KeyPerm(C.KEY_GRP_READ)
	KEY_GRP_WRITE   = KeyPerm(C.KEY_GRP_WRITE)
	KEY_GRP_SEARCH  = KeyPerm(C.KEY_GRP_SEARCH)
	KEY_GRP_LINK    = KeyPerm(C.KEY_GRP_LINK)
	KEY_GRP_SETATTR = KeyPerm(C.KEY_GRP_SETATTR)
	KEY_GRP_ALL     = KeyPerm(C.KEY_GRP_ALL)

	KEY_OTH_VIEW    = KeyPerm(C.KEY_OTH_VIEW)
	KEY_OTH_READ    = KeyPerm(C.KEY_OTH_READ)
	KEY_OTH_WRITE   = KeyPerm(C.KEY_OTH_WRITE)
	KEY_OTH_SEARCH  = KeyPerm(C.KEY_OTH_SEARCH)
	KEY_OTH_LINK    = KeyPerm(C.KEY_OTH_LINK)
	KEY_OTH_SETATTR = KeyPerm(C.KEY_OTH_SETATTR)
	KEY_OTH_ALL     = KeyPerm(C.KEY_OTH_ALL)
)

//
// RequestKey() wraps request_key(2).
//
// It returns the serial number of the key found with type = `keyType`
// and description = `desc` in the keyring `keyring`.
//
func RequestKey(keyType KeyType, desc string, keyring KeySerial) (KeySerial, error) {
	cKeyType := C.CString(string(keyType))
	cDesc := C.CString(desc)
	result, err := C.request_key(
		cKeyType,
		cDesc,
		nil,
		C.key_serial_t(int(keyring)))
	C.free(unsafe.Pointer(cKeyType))
	C.free(unsafe.Pointer(cDesc))
	if err != nil {
		return 0, err.(syscall.Errno)
	} else {
		return KeySerial(int(result)), nil
	}
}

//
// AddKeyBytes wraps add_key(2).
//
// It returns the serial number of the added key.
//
func AddKeyBytes(keyType KeyType, desc string, data []byte, keyring KeySerial) (KeySerial, error) {
	cKeyType := C.CString(string(keyType))
	cDesc := C.CString(desc)
	payloadLen := C.size_t(len(data))

	var secret unsafe.Pointer

	if data != nil {
		secret = unsafe.Pointer(&data[0])
	}

	result, err := C.add_key(cKeyType, cDesc, secret, payloadLen, C.key_serial_t(int(keyring)))

	C.free(unsafe.Pointer(cKeyType))
	C.free(unsafe.Pointer(cDesc))

	if err != nil {
		return 0, err.(syscall.Errno)
	} else {
		return KeySerial(int(result)), nil
	}
}

//
// AddKey is a helper for AddKeyBytes() that accepts a data string instead of
// a byte array.
//
func AddKey(keyType KeyType, desc string, data string, keyring KeySerial) (KeySerial, error) {
	bytes := []byte(data)

	return AddKeyBytes(keyType, desc, bytes, keyring)
}

//
// ReadKeyBytes() reads a key with the given serial # using keyctl_read_alloc(3), and returns the bytes read.
//
func ReadKeyBytes(key KeySerial) ([]byte, error) {

	var ptr unsafe.Pointer = nil
	bytes, err := C.keyctl_read_alloc(C.key_serial_t(int(key)), (*unsafe.Pointer)(&ptr))

	if err != nil {
		return nil, err.(syscall.Errno)
	}
	if bytes > 0 && ptr != nil {
		result := C.GoBytes(ptr, bytes)
		C.free(ptr)
		return result, nil
	}
	return nil, nil

}

func parseKeyDesc(keySerial KeySerial, keyDesc string) (*KeyDesc, error) {
	// description is type;uid;gid;permMask;desc
	tokens := strings.SplitN(keyDesc, ";", 5)

	if len(tokens) < 5 {
		return nil, errors.New("malformed key desc string, not enough tokens: " + keyDesc)
	}

	uid, _ := strconv.Atoi(tokens[1])
	gid, _ := strconv.Atoi(tokens[2])
	permissions, _ := strconv.ParseUint(tokens[3], 16, 32)
	desc := tokens[4]

	return &KeyDesc{
		Serial:      keySerial,
		Type:        KeyType(tokens[0]),
		Uid:         uint(uid),
		Gid:         uint(gid),
		Permissions: uint(permissions),
		Description: desc,
	}, nil
}

//
// DescribeKey() wraps keyctl_describe_alloc() to describe a key
//
func DescribeKey(key KeySerial) (*KeyDesc, error) {
	var ptr *C.char = nil
	bytes, err := C.keyctl_describe_alloc(C.key_serial_t(int(key)), (&ptr))

	if err == nil && bytes > 0 && ptr != nil {
		descString := C.GoString(ptr)
		C.free(unsafe.Pointer(ptr))
		return parseKeyDesc(key, descString)
	}

	return nil, err.(syscall.Errno)
}

//
// ReadKey() is a wrapper for ReadKeyBytes() that reads a key with the given serial #, and converts
// whatever is in the output buffer to a string value.
//
func ReadKey(key KeySerial) (string, error) {
	bytes, err := ReadKeyBytes(key)

	if err != nil {
		return "", err
	} else {
		return string(bytes), nil
	}
}

//
// Clear() will call keyctl_clear(3) to clear a keyring.
//
func Clear(keyring KeySerial) error {
	_, err := C.keyctl_clear(C.key_serial_t(keyring))

	if err != nil {
		return err.(syscall.Errno)
	}
	return nil
}

//
// Chown wraps keyctl_chown(3) to change ownership of the key.
//
// See: http://man7.org/linux/man-pages/man3/keyctl_chown.3.html
//
func Chown(key KeySerial, uid uint, gid uint) error {

	_, err := C.keyctl_chown(C.key_serial_t(key), C.uid_t(uid), C.gid_t(gid))

	if err != nil {
		return err.(syscall.Errno)
	}
	return nil
}

//
// Revoke() will call keyctl_revoke(3) to revoke a key.
//
// See: http://man7.org/linux/man-pages/man3/keyctl_revoke.3.html
//
func Revoke(key KeySerial) error {
	_, err := C.keyctl_revoke(C.key_serial_t(key))

	if err != nil {
		return err.(syscall.Errno)
	}
	return nil
}

//
// SetTimeout() will call keyctl_set_timeout(3) to set a `seconds`
// timeout on a key.
//
// See: http://man7.org/linux/man-pages/man3/keyctl_set_timeout.3.html
//
func SetTimeout(key KeySerial, seconds uint) error {
	_, err := C.keyctl_set_timeout(C.key_serial_t(key), C.uint(seconds))

	if err != nil {
		return err.(syscall.Errno)
	}
	return nil
}

//
// SetPerm() will call keyctl_setperm(3) to set permissions on a key.
// mask is a bitwise `or` value of KeyPerm values, e.g.
// KEY_USR_VIEW | KEY_USR_READ
//
// See: http://man7.org/linux/man-pages/man3/keyctl_setperm.3.html
//
func SetPerm(key KeySerial, mask KeyPerm) error {
	_, err := C.keyctl_setperm(C.key_serial_t(key), C.key_perm_t(mask))

	if err != nil {
		return err.(syscall.Errno)
	}
	return nil
}

//
// NewKeyRing() creates a keyring with description `desc`
// under the parent keyring `parentRing`
//
func NewKeyRing(desc string, parentRing KeySerial) (KeySerial, error) {
	return AddKeyBytes(KEYRING, desc, nil, parentRing)
}

//
// ListKeysInKeyRing() will list  all keys in keyring, returning a
// `KeyDesc` for each.
//
func ListKeysInKeyRing(keyring KeySerial) ([]*KeyDesc, error) {

	desc, err := DescribeKey(keyring)

	if err != nil {
		return nil, err
	}

	if desc == nil || desc.Type != KEYRING {
		// it's not a keyring, so makes no sense to list the keys inside of it..
		return nil, errors.New("not a valid keyring: " + string(keyring))
	}

	keyBytes, err := ReadKeyBytes(keyring)

	if err != nil {
		return nil, err
	}

	if len(keyBytes)%4 != 0 {
		return nil, errors.New("Expected a string of key_serial_t")
	}

	keys := make([]*KeyDesc, 0)

	for c := 0; c < len(keyBytes); c += 4 {
		var key int32
		buf := bytes.NewBuffer(keyBytes[c:(c + 4)])
		err = binary.Read(buf, binary.LittleEndian, &key)
		if err != nil {
			return nil, err
		}

		desc, err := DescribeKey(KeySerial(key))

		if err != nil {
			return nil, err
		}

		keys = append(keys, desc)
	}

	return keys, nil

}
