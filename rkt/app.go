// Copyright 2015 CoreOS, Inc.
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

//+build linux

package main

import (
	"flag"
	"fmt"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema/types"
)

var (
	Apps rktApps // global apps
)

type rktApp struct {
	image string   // the image as-supplied by the user on the cli
	args  []string // any arguments the user supplied for this app
	asc   string   // signature file override for image verification (if fetching occurs)

	imageID types.Hash // resolved image identifier
}

// this needs to be a struct for the type defines like appAsc to work correctly with receivers
type rktApps struct {
	apps []rktApp
}

// reset creates a new slice for al.apps, needed by tests
func (al *rktApps) reset() {
	al.apps = make([]rktApp, 0)
}

// count returns the number of apps in al
func (al *rktApps) count() int {
	return len(al.apps)
}

// create creates a new app in al and returns a pointer to it
func (al *rktApps) create(img string) {
	al.apps = append(al.apps, rktApp{image: img})
}

// last returns a pointer to the top app in al
func (al *rktApps) last() *rktApp {
	if len(al.apps) == 0 {
		return nil
	} else {
		return &al.apps[len(al.apps)-1]
	}
}

// appendArg appends another argument onto the app
func (a *rktApp) appendArg(arg string) {
	a.args = append(a.args, arg)
}

// sets the ascii-armored signature file path for the app
func (a *rktApp) setAsc(asc string) {
	a.asc = asc
}

// gets the ascii-armored signature file path for the app
func (a *rktApp) getAsc() string {
	return a.asc
}

// parseApps looks through the args for support of per-app argument lists delimited with "--" and "---".
// Between per-app argument lists flags.Parse() is called using the supplied FlagSet.
// Anything not consumed by flags.Parse() and not found to be a per-app argument list is treated as an image.
func (al *rktApps) parse(args []string, flags *flag.FlagSet) error {
	al.reset()
	nAppsLastAppArgs := al.count()

	// valid args here may either be:
	// not-"--"; flags handled by *flags or an image specifier
	// "--"; app arguments begin
	// "---"; conclude app arguments
	// between "--" and "---" pairs anything is permitted.
	inAppArgs := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if inAppArgs {
			switch a {
			case "---":
				// conclude this app's args
				inAppArgs = false
			default:
				// keep appending to this app's args
				al.last().appendArg(a)
			}
		} else {
			switch a {
			case "--":
				// begin app's args
				inAppArgs = true

				// catch some likely mistakes
				if nAppsLastAppArgs == al.count() {
					if al.count() == 0 {
						return fmt.Errorf("an image is required before any app arguments")
					}
					return fmt.Errorf("only one set of app arguments allowed per image")
				}
				nAppsLastAppArgs = al.count()
			case "---":
				// ignore triple dashes since they aren't images
				// TODO(vc): I don't think ignoring this is appropriate, probably should error; it implies malformed argv.
				// "---" is not an image separator, it's an optional argument list terminator.
				// encountering it outside of inAppArgs is likely to be "--" typoed
			default:
				// consume any potential inter-app flags
				if err := flags.Parse(args[i:]); err != nil {
					return err
				}
				nInterFlags := (len(args[i:]) - flags.NArg())

				if nInterFlags > 0 {
					// XXX(vc): flag.Parse() annoyingly consumes the "--", reclaim it here if necessary
					if args[i+nInterFlags-1] == "--" {
						nInterFlags--
					}

					// advance past what flags.Parse() consumed
					i += nInterFlags - 1 // - 1 because of i++
				} else {
					// flags.Parse() didn't want this arg, treat as image
					al.create(a)
				}
			}
		}
	}

	return nil
}

// these convenience functions just return typed lists containing just the named member
// TODO(vc): these probably go away when we just pass rktApps to stage0

// getImages returns a list of the images in al, one per app.
// The order reflects the app order in al.
func (al *rktApps) getImages() []string {
	il := []string{}
	for _, a := range al.apps {
		il = append(il, a.image)
	}
	return il
}

// getArgs returns a list of lists of arguments in al, one list of args per app.
// The order reflects the app order in al.
func (al *rktApps) getArgs() [][]string {
	aal := [][]string{}
	for _, a := range al.apps {
		aal = append(aal, a.args)
	}
	return aal
}

// getImageIDs returns a list of the imageIDs in al, one per app.
// The order reflects the app order in al.
func (al *rktApps) getImageIDs() []types.Hash {
	hl := []types.Hash{}
	for _, a := range al.apps {
		hl = append(hl, a.imageID)
	}
	return hl
}

// Value interface implementations for the various per-app fields we provide flags for

// appAsc is for aci --signature overrides
type appAsc rktApps

func (al *appAsc) Set(s string) error {
	app := (*rktApps)(al).last()
	if app == nil {
		return fmt.Errorf("--signature must follow an image")
	}

	if app.getAsc() != "" {
		return fmt.Errorf("--signature specified multiple times for the same image")
	}

	app.setAsc(s)

	return nil
}

func (al *appAsc) String() string {
	app := (*rktApps)(al).last()
	if app == nil {
		return ""
	}
	return app.getAsc()
}

// TODO(vc): --mount, --set-env, etc.
