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
	"bytes"
	"fmt"
	"strings"

	dist "github.com/coreos/rkt/common/distribution"
	"github.com/coreos/rkt/common/mediatype"
	rktflag "github.com/coreos/rkt/rkt/flag"
	"github.com/coreos/rkt/rkt/image"
	"github.com/coreos/rkt/store/casref/rwcasref"

	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/lastditch"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

const (
	defaultTimeLayout = "2006-01-02 15:04:05.999 -0700 MST"

	ref        = "ref"
	digest     = "digest"
	imageType  = "image type"
	importTime = "import time"
	lastUsed   = "last used"
	size       = "size"
)

// Convenience methods for formatting fields
func l(s string) string {
	return strings.ToLower(strings.Replace(s, " ", "", -1))
}
func u(s string) string {
	return strings.ToUpper(s)
}

var (
	// map of valid fields and related header name
	ImagesFieldHeaderMap = map[string]string{
		l(ref):        u(ref),
		l(digest):     u(digest),
		l(imageType):  u(imageType),
		l(importTime): u(importTime),
		l(lastUsed):   u(lastUsed),
		l(size):       u(size),
	}

	// map of valid sort fields containing the mapping between the provided field name
	// and the related aciinfo's field name.
	ImagesFieldAciInfoMap = map[string]string{
		l(ref):        "reference",
		l(digest):     l(digest),
		l(imageType):  l(imageType),
		l(importTime): l(importTime),
		l(lastUsed):   l(lastUsed),
		l(size):       l(size),
	}

	ImagesSortableFields = map[string]struct{}{
		l(importTime): {},
		l(lastUsed):   {},
		l(size):       {},
	}
)

type ImagesSortAsc bool

func (isa *ImagesSortAsc) Set(s string) error {
	switch strings.ToLower(s) {
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
	return "imagesSortAsc"
}

var (
	cmdImageList = &cobra.Command{
		Use:   "list",
		Short: "List images in the local store",
		Long:  `Optionally, allows the user to specify the fields and sort order.`,
		Run:   runWrapper(runImages),
	}
	flagImagesFields     *rktflag.OptionList
	flagImagesSortFields *rktflag.OptionList
	flagImagesSortAsc    ImagesSortAsc
)

func init() {
	sortFields := []string{l(importTime), l(lastUsed), l(size)}

	fields := []string{l(ref), l(digest), l(imageType), l(size)}

	// Set defaults
	var err error
	flagImagesFields, err = rktflag.NewOptionList(fields, strings.Join(fields, ","))
	if err != nil {
		stderr.FatalE("", err)
	}
	flagImagesSortFields, err = rktflag.NewOptionList(sortFields, l(importTime))
	if err != nil {
		stderr.FatalE("", err)
	}
	flagImagesSortAsc = true

	cmdImage.AddCommand(cmdImageList)
	cmdImageList.Flags().Var(flagImagesFields, "fields", fmt.Sprintf(`comma-separated list of fields to display. Accepted values: %s`,
		flagImagesFields.PermissibleString()))
	cmdImageList.Flags().Var(flagImagesSortFields, "sort", fmt.Sprintf(`sort the output according to the provided comma-separated list of fields. Accepted values: %s`,
		flagImagesSortFields.PermissibleString()))
	cmdImageList.Flags().Var(&flagImagesSortAsc, "order", `choose the sorting order if at least one sort field is provided (--sort). Accepted values: "asc", "desc"`)
	cmdImageList.Flags().BoolVar(&flagNoLegend, "no-legend", false, "suppress a legend with the list")
	cmdImageList.Flags().BoolVar(&flagFullOutput, "full", false, "use long output format")
}

func runImages(cmd *cobra.Command, args []string) int {
	var errors []error
	tabBuffer := new(bytes.Buffer)
	tabOut := getTabOutWithWriter(tabBuffer)

	if !flagNoLegend {
		var headerFields []string
		for _, f := range flagImagesFields.Options {
			headerFields = append(headerFields, ImagesFieldHeaderMap[f])
		}
		fmt.Fprintf(tabOut, "%s\n", strings.Join(headerFields, "\t"))
	}

	s, err := rwcasref.NewStore(storeDir())
	if err != nil {
		stderr.PrintE("cannot open store", err)
		return 1
	}

	ts, err := newTreeStore(s)
	if err != nil {
		stderr.PrintE("cannot open tree store", err)
		return 1
	}

	var sortAciinfoFields []string
	for _, f := range flagImagesSortFields.Options {
		sortAciinfoFields = append(sortAciinfoFields, ImagesFieldAciInfoMap[f])
	}

	// Get all blob infos for MediaTypeACI
	blobInfos, err := s.GetBlobsInfosByMediaType(string(mediatype.ACI))
	if err != nil {
		stderr.PrintE("cannot get ACI blob infos", err)
		return 1
	}

	tsSizes := map[string]int64{}
	for _, blobInfo := range blobInfos {
		infos, err := ts.GetInfosByImageDigest(blobInfo.Digest)
		if err != nil {
			stderr.Error(err)
			return 1
		}
		var treeStoreSize int64
		for _, i := range infos {
			treeStoreSize += i.Size
		}
		tsSizes[blobInfo.Digest] = treeStoreSize
	}

	refs, err := s.GetAllRefs()
	if err != nil {
		stderr.PrintE("unable to get aci infos", err)
		return 1
	}
	digestRefsMap := map[string][]string{}
	for _, ref := range refs {
		curRefs := digestRefsMap[string(ref.Digest)]
		dist, err := dist.NewDistribution(ref.ID)
		if err != nil {
			errors = append(errors, fmt.Errorf("ref %q cannot be converted to a distribution", ref.ID))
			continue
		}
		digestRefsMap[string(ref.Digest)] = append(curRefs, image.DistSimpleString(dist))
	}

	for _, blobInfo := range blobInfos {
		var fieldValues []string
		showRefs := false
		fieldPos := 0
		refPos := 0

		for _, f := range flagImagesFields.Options {
			fieldValue := ""
			switch f {
			case l(ref):
				// TODO(sgotti) ellipsize long refs? how?
				fieldValue = "<none>"
				if _, ok := digestRefsMap[blobInfo.Digest]; ok {
					showRefs = true
					refPos = fieldPos
				}
			case l(digest):
				digest := blobInfo.Digest
				if !flagFullOutput {
					// The short hash form is [HASH_ALGO]-[FIRST 12 CHAR]
					// For example, sha512-123456789012
					pos := strings.Index(digest, "-")
					trimLength := pos + 13
					if pos > 0 && trimLength < len(digest) {
						digest = digest[:trimLength]
					}
				}
				fieldValue = digest
			case l(imageType):
				switch blobInfo.MediaType {
				case string(mediatype.ACI):
					fieldValue = "ACI"
				case string(mediatype.OCIManifest):
					fieldValue = "OCI"
				default:
					fieldValue = "<unknown>"
				}
			case l(size):
				totalSize := blobInfo.Size + tsSizes[blobInfo.Digest]
				if flagFullOutput {
					fieldValue = fmt.Sprintf("%d", totalSize)
				} else {
					fieldValue = humanize.IBytes(uint64(totalSize))
				}
			}
			fieldValues = append(fieldValues, fieldValue)
			fieldPos++
		}

		if showRefs {
			for _, ref := range digestRefsMap[blobInfo.Digest] {
				fieldValues[refPos] = ref
				fmt.Fprintf(tabOut, "%s\n", strings.Join(fieldValues, "\t"))
			}
		} else {
			fmt.Fprintf(tabOut, "%s\n", strings.Join(fieldValues, "\t"))
		}
	}

	if len(errors) > 0 {
		sep := "----------------------------------------"
		stderr.Printf("%d error(s) encountered when listing images:", len(errors))
		stderr.Print(sep)
		for _, err := range errors {
			stderr.Error(err)
			stderr.Print(sep)
		}
		stderr.Print("misc:")
		stderr.Printf("  rkt's appc version: %s", schema.AppContainerVersion)
		// make a visible break between errors and the listing
		stderr.Print("")
	}
	tabOut.Flush()
	stdout.Print(tabBuffer.String())
	return 0
}

func newImgListLoadError(err error, imj []byte, blobKey string) error {
	var lines []string
	im := lastditch.ImageManifest{}
	imErr := im.UnmarshalJSON(imj)
	if imErr == nil {
		lines = append(lines, fmt.Sprintf("Unable to load manifest of image %s (spec version %s) because it is invalid:", im.Name, im.ACVersion))
		lines = append(lines, fmt.Sprintf("  %v", err))
	} else {
		lines = append(lines, "Unable to load manifest of an image because it is invalid:")
		lines = append(lines, fmt.Sprintf("  %v", err))
		lines = append(lines, "  Also, failed to get any information about invalid image manifest:")
		lines = append(lines, fmt.Sprintf("    %v", imErr))
	}
	lines = append(lines, "ID of the invalid image:")
	lines = append(lines, fmt.Sprintf("  %s", blobKey))
	return fmt.Errorf("%s", strings.Join(lines, "\n"))
}
