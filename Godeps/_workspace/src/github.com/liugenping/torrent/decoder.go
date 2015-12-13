/*
 * Copyright (c) 2014 Mark Samman <https://github.com/marksamman/bencode>
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 */

package torrent

import (
	"bufio"
	"errors"
	"io"
	"strconv"
)

type decoder struct {
	bufio.Reader
}

func (decoder *decoder) readIntUntil(until byte) (interface{}, error) {
	res, err := decoder.ReadSlice(until)
	if err != nil {
		return nil, err
	}

	str := string(res[:len(res)-1])

	if value, err := strconv.ParseInt(str, 10, 64); err == nil {
		return value, nil
	} else if value, err := strconv.ParseUint(str, 10, 64); err == nil {
		return value, nil
	}
	return nil, err
}

func (decoder *decoder) readInt() (interface{}, error) {
	return decoder.readIntUntil('e')
}

func (decoder *decoder) readInterfaceType(identifier byte) (item interface{}, err error) {
	switch identifier {
	case 'i':
		item, err = decoder.readInt()
	case 'l':
		item, err = decoder.readList()
	case 'd':
		item, err = decoder.readDictionary()
	default:
		if err := decoder.UnreadByte(); err != nil {
			return nil, err
		}
		item, err = decoder.readString()
	}
	return item, err
}

func (decoder *decoder) readList() ([]interface{}, error) {
	var list []interface{}
	for {
		ch, err := decoder.ReadByte()
		if err != nil {
			return nil, err
		}

		if ch == 'e' {
			break
		}

		item, err := decoder.readInterfaceType(ch)
		if err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, nil
}

func (decoder *decoder) readString() (string, error) {
	len, err := decoder.readIntUntil(':')
	if err != nil {
		return "", err
	}

	var stringLength int64
	var ok bool
	if stringLength, ok = len.(int64); !ok {
		return "", errors.New("string length may not exceed the size of int64")
	}

	if stringLength < 0 {
		return "", errors.New("string length can not be a negative number")
	}

	buffer := make([]byte, stringLength)
	_, err = io.ReadFull(decoder, buffer)
	return string(buffer), err
}

func (decoder *decoder) readDictionary() (map[string]interface{}, error) {
	dict := make(map[string]interface{})
	for {
		key, err := decoder.readString()
		if err != nil {
			return nil, err
		}

		ch, err := decoder.ReadByte()
		if err != nil {
			return nil, err
		}

		item, err := decoder.readInterfaceType(ch)
		if err != nil {
			return nil, err
		}

		dict[key] = item

		nextByte, err := decoder.ReadByte()
		if err != nil {
			return nil, err
		}

		if nextByte == 'e' {
			break
		} else if err := decoder.UnreadByte(); err != nil {
			return nil, err
		}
	}
	return dict, nil
}

// Decode takes an io.Reader and parses it as bencode,
// on failure, err will be a non-nil value
func Decode(reader io.Reader) (map[string]interface{}, error) {
	decoder := decoder{*bufio.NewReader(reader)}
	if firstByte, err := decoder.ReadByte(); err != nil {
		return make(map[string]interface{}), nil
	} else if firstByte != 'd' {
		return nil, errors.New("bencode data must begin with a dictionary")
	}
	return decoder.readDictionary()
}
