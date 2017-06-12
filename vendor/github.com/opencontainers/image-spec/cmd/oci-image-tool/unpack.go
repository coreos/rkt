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
	"github.com/spf13/cobra"
)

// supported unpack types
var unpackTypes = []string{
	typeImageLayout,
	typeImage,
}

type unpackCmd struct {
	stdout *log.Logger
	stderr *log.Logger
	typ    string // the type to unpack, can be empty string
	ref    string
}

func newUnpackCmd(stdout, stderr *log.Logger) *cobra.Command {
	v := &unpackCmd{
		stdout: stdout,
		stderr: stderr,
	}

	cmd := &cobra.Command{
		Use:   "unpack [src] [dest]",
		Short: "Unpack an image or image source layout",
		Long:  `Unpack the OCI image .tar file or OCI image layout directory present at [src] to the destination directory [dest].`,
		Run:   v.Run,
	}

	cmd.Flags().StringVar(
		&v.typ, "type", "",
		fmt.Sprintf(
			`Type of the file to unpack. If unset, oci-image-tool will try to auto-detect the type. One of "%s"`,
			strings.Join(unpackTypes, ","),
		),
	)

	cmd.Flags().StringVar(
		&v.ref, "ref", "v1.0",
		`The ref pointing to the manifest to be unpacked. This must be present in the "refs" subdirectory of the image.`,
	)

	return cmd
}

func (v *unpackCmd) Run(cmd *cobra.Command, args []string) {
	if len(args) != 2 {
		v.stderr.Print("both src and dest must be provided")
		if err := cmd.Usage(); err != nil {
			v.stderr.Println(err)
		}
		os.Exit(1)
	}

	if v.typ == "" {
		typ, err := autodetect(args[0])
		if err != nil {
			v.stderr.Printf("%q: autodetection failed: %v", args[0], err)
			os.Exit(1)
		}
		v.typ = typ
	}

	var err error
	switch v.typ {
	case typeImageLayout:
		err = image.UnpackLayout(args[0], args[1], v.ref)

	case typeImage:
		err = image.Unpack(args[0], args[1], v.ref)
	}

	if err != nil {
		v.stderr.Printf("unpacking failed: %v", err)
		os.Exit(1)
	}

	os.Exit(0)
}
