// Copyright 2015 The rkt Authors
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
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
	"strings"

	"github.com/coreos/rkt/store"
)

const (
	defaultTimeLayout = "2006-01-02 15:04:05.999 -0700 MST"

	keyField        = "key"
	appNameField    = "appname"
	importTimeField = "importtime"
	latestField     = "latest"
)

var (
	// map of valid fields and related flag value
	imagesAllFields = map[string]struct{}{
		keyField:        struct{}{},
		appNameField:    struct{}{},
		importTimeField: struct{}{},
		latestField:     struct{}{},
	}

	// map of valid fields and related header name
	ImagesFieldHeaderMap = map[string]string{
		keyField:        "KEY",
		appNameField:    "APPNAME",
		importTimeField: "IMPORTTIME",
		latestField:     "LATEST",
	}

	// map of valid sort fields containing the mapping between the provided field name
	// and the related aciinfo's field name.
	ImagesFieldAciInfoMap = map[string]string{
		keyField:        "blobkey",
		appNameField:    "appname",
		importTimeField: "importtime",
		latestField:     "latest",
	}

	ImagesSortableFields = map[string]struct{}{
		appNameField:    struct{}{},
		importTimeField: struct{}{},
	}
)

type ImagesFields []string

func (ifs *ImagesFields) Set(s string) error {
	*ifs = []string{}
	fields := strings.Split(s, ",")
	seen := map[string]struct{}{}
	for _, f := range fields {
		// accept any case
		f = strings.ToLower(f)
		_, ok := imagesAllFields[f]
		if !ok {
			return fmt.Errorf("unknown field %q", f)
		}
		if _, ok := seen[f]; ok {
			return fmt.Errorf("duplicated field %q", f)
		}
		*ifs = append(*ifs, f)
		seen[f] = struct{}{}
	}

	return nil
}

func (ifs *ImagesFields) String() string {
	return strings.Join(*ifs, ",")
}

func (ifs *ImagesFields) Type() string {
	return "ImagesFields"
}

type ImagesSortFields []string

func (isf *ImagesSortFields) Set(s string) error {
	*isf = []string{}
	fields := strings.Split(s, ",")
	seen := map[string]struct{}{}
	for _, f := range fields {
		// accept any case
		f = strings.ToLower(f)
		_, ok := ImagesSortableFields[f]
		if !ok {
			return fmt.Errorf("unknown field %q", f)
		}
		if _, ok := seen[f]; ok {
			return fmt.Errorf("duplicated field %q", f)
		}
		*isf = append(*isf, f)
		seen[f] = struct{}{}
	}

	return nil
}

func (isf *ImagesSortFields) String() string {
	return strings.Join(*isf, ",")
}
func (isf *ImagesSortFields) Type() string {
	return "ImagesSortFields"
}

type ImagesSortAsc bool

func (isa *ImagesSortAsc) Set(s string) error {
	switch s {
	case "asc":
		*isa = true
	case "desc":
		*isa = false
	default:
		return fmt.Errorf("wrong sort order")
	}
	return nil
}

func (isa *ImagesSortAsc) String() string {
	if *isa {
		return "asc"
	}
	return "desc"
}
func (isa *ImagesSortAsc) Type() string {
	return "ImagesSortAsc"
}

var (
	cmdImages = &cobra.Command{
		Use:   "images",
		Short: "List images in the local store",
		Run: func(cmd *cobra.Command, args []string) {
			subCmdExitCode = runImages(cmd, args)
		},
	}
	flagImagesFields     ImagesFields
	flagImagesSortFields ImagesSortFields
	flagImagesSortAsc    ImagesSortAsc
)

func init() {
	// Set defaults
	flagImagesFields = []string{keyField, appNameField, importTimeField, latestField}
	flagImagesSortFields = []string{importTimeField}
	flagImagesSortAsc = true

	cmdImages.Flags().VarP(&flagImagesFields, "fields", "f", `comma separated list of fields to display. Accepted values: "key", "appname", "importtime", "latest"`)
	cmdImages.Flags().VarP(&flagImagesSortFields, "sort", "s", `sort the output according to the provided comma separated list of fields. Accepted valies: "appname", "importtime"`)
	cmdImages.Flags().VarP(&flagImagesSortAsc, "order", "o", `choose the sorting order if at least one sort field is provided (--sort). Accepted values: "asc", "desc"`)
	cmdImages.Flags().BoolVarP(&flagNoLegend, "no-legend", "n", false, "suppress a legend with the list")
	rktCmd.AddCommand(cmdImages)
}

func runImages(cmd *cobra.Command, args []string) (exit int) {
	if !flagNoLegend {
		headerFields := []string{}
		for _, f := range flagImagesFields {
			headerFields = append(headerFields, ImagesFieldHeaderMap[f])
		}
		fmt.Fprintf(tabOut, "%s\n", strings.Join(headerFields, "\t"))
	}

	s, err := store.NewStore(flagDataDir)
	if err != nil {
		stderr("images: cannot open store: %v\n", err)
		return 1
	}

	sortAciinfoFields := []string{}
	for _, f := range flagImagesSortFields {
		sortAciinfoFields = append(sortAciinfoFields, ImagesFieldAciInfoMap[f])
	}
	aciInfos, err := s.GetAllACIInfos(sortAciinfoFields, bool(flagImagesSortAsc))
	if err != nil {
		stderr("images: unable to get aci infos: %v", err)
		return
	}

	for _, aciInfo := range aciInfos {
		im, err := s.GetImageManifest(aciInfo.BlobKey)
		if err != nil {
			// ignore aciInfo with missing image manifest as it can be deleted in the meantime
			continue
		}
		version, ok := im.Labels.Get("version")
		for _, f := range flagImagesFields {
			switch f {
			case keyField:
				fmt.Fprintf(tabOut, "%s", aciInfo.BlobKey)
			case appNameField:
				fmt.Fprintf(tabOut, "%s", aciInfo.AppName)
				if ok {
					fmt.Fprintf(tabOut, ":%s", version)
				}
			case importTimeField:
				fmt.Fprintf(tabOut, "%s", aciInfo.ImportTime.Format(defaultTimeLayout))
			case latestField:
				fmt.Fprintf(tabOut, "%t", aciInfo.Latest)
			}
			fmt.Fprintf(tabOut, "\t")

		}
		fmt.Fprintf(tabOut, "\n")
	}

	tabOut.Flush()
	return 0
}
