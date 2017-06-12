// Copyright 2016 The Linux Foundation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package schema

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/pkg/errors"
	"github.com/xeipuuv/gojsonschema"
)

// Validator wraps a media type string identifier
// and implements validation against a JSON schema.
type Validator string

// ValidationError contains all the errors that happened during validation.
type ValidationError struct {
	Errs []error
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%v", e.Errs)
}

// Validate validates the given reader against the schema of the wrapped media type.
func (v Validator) Validate(src io.Reader) error {
	buf, err := ioutil.ReadAll(src)
	if err != nil {
		return errors.Wrap(err, "unable to read the document file")
	}

	sl := gojsonschema.NewReferenceLoaderFileSystem("file:///"+specs[v], fs)
	ml := gojsonschema.NewStringLoader(string(buf))

	result, err := gojsonschema.Validate(sl, ml)
	if err != nil {
		return errors.Wrapf(
			WrapSyntaxError(bytes.NewReader(buf), err),
			"schema %s: unable to validate", v)
	}

	if result.Valid() {
		return nil
	}

	errs := make([]error, 0, len(result.Errors()))
	for _, desc := range result.Errors() {
		errs = append(errs, fmt.Errorf("%s", desc))
	}

	return ValidationError{
		Errs: errs,
	}
}

type unimplemented string

func (v unimplemented) Validate(src io.Reader) error {
	return fmt.Errorf("%s: unimplemented", v)
}
