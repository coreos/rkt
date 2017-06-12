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

package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/opencontainers/image-spec/image"
	"github.com/opencontainers/image-spec/schema"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// supported validation types
var validateTypes = []string{
	typeImageLayout,
	typeImage,
	typeManifest,
	typeManifestList,
	typeConfig,
}

type validateCmd struct {
	stdout *log.Logger
	stderr *log.Logger
	typ    string // the type to validate, can be empty string
	refs   []string
}

func newValidateCmd(stdout, stderr *log.Logger) *cobra.Command {
	v := &validateCmd{
		stdout: stdout,
		stderr: stderr,
	}

	cmd := &cobra.Command{
		Use:   "validate FILE...",
		Short: "Validate one or more image files",
		Run:   v.Run,
	}

	cmd.Flags().StringVar(
		&v.typ, "type", "",
		fmt.Sprintf(
			`Type of the file to validate. If unset, oci-image-tool will try to auto-detect the type. One of "%s".`,
			strings.Join(validateTypes, ","),
		),
	)

	cmd.Flags().StringSliceVar(
		&v.refs, "ref", nil,
		`A set of refs pointing to the manifests to be validated. Each reference must be present in the "refs" subdirectory of the image. Only applicable if type is image or imageLayout.`,
	)

	return cmd
}

func (v *validateCmd) Run(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		v.stderr.Printf("no files specified")
		if err := cmd.Usage(); err != nil {
			v.stderr.Println(err)
		}
		os.Exit(1)
	}

	var exitcode int
	for _, arg := range args {
		err := v.validatePath(arg)

		if err == nil {
			v.stdout.Printf("%s: OK", arg)
			continue
		}

		var errs []error
		if verr, ok := errors.Cause(err).(schema.ValidationError); ok {
			errs = verr.Errs
		} else if serr, ok := errors.Cause(err).(*schema.SyntaxError); ok {
			v.stderr.Printf("%s:%d:%d: validation failed: %v", arg, serr.Line, serr.Col, err)
			exitcode = 1
			continue
		} else {
			v.stderr.Printf("%s: validation failed: %v", arg, err)
			exitcode = 1
			continue
		}

		for _, err := range errs {
			v.stderr.Printf("%s: validation failed: %v", arg, err)
		}

		exitcode = 1
	}

	os.Exit(exitcode)
}

func (v *validateCmd) validatePath(name string) error {
	var (
		err error
		typ = v.typ
	)

	if typ == "" {
		if typ, err = autodetect(name); err != nil {
			return errors.Wrap(err, "unable to determine type")
		}
	}

	switch typ {
	case typeImageLayout:
		return image.ValidateLayout(name, v.refs, v.stdout)
	case typeImage:
		return image.Validate(name, v.refs, v.stdout)
	}

	f, err := os.Open(name)
	if err != nil {
		return errors.Wrap(err, "unable to open file")
	}
	defer f.Close()

	switch typ {
	case typeManifest:
		return schema.MediaTypeManifest.Validate(f)
	case typeManifestList:
		return schema.MediaTypeManifestList.Validate(f)
	case typeConfig:
		return schema.MediaTypeImageConfig.Validate(f)
	}

	return fmt.Errorf("type %q unimplemented", typ)
}
