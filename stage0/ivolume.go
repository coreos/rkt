package stage0

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/coreos/rkt/Godeps/_workspace/src/code.google.com/p/go-uuid/uuid"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
)

// InjectedVolume is an arbitrary volume that is injected into pod when the container
// starts and is not defined in the manifest
type InjectedVolume struct {
	types.Volume
	Dest string
}

func InjectedVolumeFromString(s string) (*InjectedVolume, error) {
	var vol InjectedVolume

	v, err := url.ParseQuery(strings.Replace(s, ",", "&", -1))
	if err != nil {
		return nil, err
	}

	for key, val := range v {
		if len(val) > 1 {
			return nil, fmt.Errorf("label %s with multiple values %q", key, val)
		}
		// TOOD(philips): make this less hardcoded
		switch key {
		case "kind":
			vol.Kind = val[0]
		case "source":
			vol.Source = val[0]
		case "readOnly":
			ro, err := strconv.ParseBool(val[0])
			if err != nil {
				return nil, err
			}
			vol.ReadOnly = &ro
		case "dest":
			vol.Dest = val[0]
		default:
			return nil, fmt.Errorf("unknown volume parameter %q", key)
		}
	}
	vol.Name = types.ACName(uuid.New())
	return &vol, nil
}

func (iv *InjectedVolume) toMountPoint() types.MountPoint {
	ro := false
	if iv.ReadOnly != nil {
		ro = *iv.ReadOnly
	}
	return types.MountPoint{
		Name:     iv.Name,
		Path:     iv.Dest,
		ReadOnly: ro,
	}
}

type InjectedVolumes []InjectedVolume

func (ivs InjectedVolumes) toMountPoints() []types.MountPoint {
	out := make([]types.MountPoint, len(ivs))
	for i := range ivs {
		out[i] = ivs[i].toMountPoint()
	}
	return out
}

func (ivs InjectedVolumes) toVolumes() []types.Volume {
	out := make([]types.Volume, len(ivs))
	for i := range ivs {
		out[i] = ivs[i].Volume
	}
	return out
}

// before injecting the volumes, make sure injected volumes do not conflict
// with any of the app mount points and volumes and each other
func (ivs InjectedVolumes) checkPodConflicts(pm schema.PodManifest) error {

	// check that volumes do not conflict with each other first
	ivexist := make(map[string]InjectedVolume)
	for _, v := range ivs {
		if iv, ok := ivexist[v.Dest]; ok {
			return fmt.Errorf(
				"injected volume destination %v conflicts with the injected volume %v", v, iv)
		}
		ivexist[v.Dest] = v
	}

	// next, check if injected volumes conflict with the mount points defined in the spec
	if len(pm.Apps) == 0 {
		return nil
	}
	mpexist := make(map[string]types.MountPoint)
	for _, app := range pm.Apps {
		if app.App == nil || len(app.App.MountPoints) == 0 {
			continue
		}
		for _, mp := range app.App.MountPoints {
			mpexist[mp.Path] = mp
		}
	}

	for _, iv := range ivs {
		if mp, ok := mpexist[iv.Dest]; ok {
			return fmt.Errorf(
				"injected volume destination %v conflicts with the existing mount point %v", iv.Dest, mp)
		}
	}
	return nil
}

func (ivs InjectedVolumes) inject(pm schema.PodManifest) (*schema.PodManifest, error) {
	if len(ivs) == 0 {
		return &pm, nil
	}

	if err := ivs.checkPodConflicts(pm); err != nil {
		return nil, err
	}

	// inject a volume and a mount point for every app
	pm.Volumes = append(pm.Volumes, ivs.toVolumes()...)
	for i := range pm.Apps {
		if pm.Apps[i].App == nil {
			continue
		}
		pm.Apps[i].App.MountPoints = append(pm.Apps[i].App.MountPoints, ivs.toMountPoints()...)
	}
	return &pm, nil
}
