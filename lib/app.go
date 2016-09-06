// Copyright 2016 The rkt Authors
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

package rkt

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/appc/spec/schema"
	pkgPod "github.com/coreos/rkt/pkg/pod"
)

// AppState defines the state of the app.
type AppState string

const (
	AppStateUnknown AppState = "unknown"
	AppStateCreated AppState = "created"
	AppStateRunning AppState = "running"
	AppStateExited  AppState = "exited"
)

type (
	// Mount defines the mount point.
	Mount struct {
		// Name of the mount.
		Name string `json:"name"`
		// Container path of the mount.
		ContainerPath string `json:"container_path"`
		// Host path of the mount.
		HostPath string `json:"host_path"`
		// Whether the mount is read-only.
		ReadOnly bool `json:"read_only"`
		// TODO(yifan): What about 'SelinuxRelabel bool'?
	}

	// App defines the app object.
	App struct {
		// Name of the app.
		Name string `json:"name"`
		// State of the app, can be created, running, exited, or unknown.
		State AppState `json:"state"`
		// Creation time of the container, nanoseconds since epoch.
		CreatedAt *int64 `json:"created_at,omitempty"`
		// Start time of the container, nanoseconds since epoch.
		StartedAt *int64 `json:"started_at,omitempty"`
		// Finish time of the container, nanoseconds since epoch.
		FinishedAt *int64 `json:"finished_at,omitempty"`
		// Exit code of the container.
		ExitCode *int `json:"exit_code,omitempty"`
		// Image ID of the container.
		ImageID string `json:"image_id"`
		// Mount points of the container.
		Mounts []*Mount `json:"mounts,omitempty"`
		// Annotations of the container.
		Annotations map[string]string `json:"annotations,omitempty"`
	}

	// Apps is a list of apps.
	Apps struct {
		AppList []App `json:"app_list,omitempty"`
	}
)

// AppsForPod returns the apps of the pod with the given uuid in the given data directory.
// If appName is non-empty, then only the app with the given name will be returned.
func AppsForPod(uuid, dataDir string, appName string) ([]*App, error) {
	p, err := pkgPod.PodFromUUIDString(dataDir, uuid)
	if err != nil {
		return nil, err
	}
	defer p.Close()

	_, podManifest, err := p.PodManifest()
	if err != nil {
		return nil, err
	}

	var apps []*App
	for _, ra := range podManifest.Apps {
		if appName != "" && appName != ra.Name.String() {
			continue
		}

		app, err := newApp(&ra, podManifest, p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot get app status: %v", err)
			continue
		}

		apps = append(apps, app)
	}

	return apps, nil
}

// newApp constructs the App object with the runtime app and pod manifest.
func newApp(ra *schema.RuntimeApp, podManifest *schema.PodManifest, pod *pkgPod.Pod) (*App, error) {
	app := &App{
		Name:        ra.Name.String(),
		ImageID:     ra.Image.ID.String(),
		Annotations: make(map[string]string),
	}

	// Generate mounts
	for _, mnt := range ra.App.MountPoints {
		name := mnt.Name.String()
		containerPath := mnt.Path
		readOnly := mnt.ReadOnly

		var hostPath string
		for _, vol := range podManifest.Volumes {
			if vol.Name != mnt.Name {
				continue
			}

			hostPath = vol.Source
			if vol.ReadOnly != nil && !readOnly {
				readOnly = *vol.ReadOnly
			}
			break
		}

		if hostPath == "" { // This should not happen.
			return nil, fmt.Errorf("cannot find corresponded volume for mount %v", mnt)
		}

		app.Mounts = append(app.Mounts, &Mount{
			Name:          name,
			ContainerPath: containerPath,
			HostPath:      hostPath,
			ReadOnly:      readOnly,
		})
	}

	// Generate annotations.
	for _, anno := range ra.Annotations {
		app.Annotations[anno.Name.String()] = anno.Value
	}

	// Generate state.
	if err := appState(app, pod); err != nil {
		return nil, fmt.Errorf("error getting app's state: %v", err)
	}

	return app, nil
}

func appState(app *App, pod *pkgPod.Pod) error {
	app.State = AppStateUnknown

	appInfoDir, err := appInfoDir(pod, app.Name)
	if err != nil {
		return err
	}

	appStartedFile, err := appStartedFile(pod, app.Name)
	if err != nil {
		return err
	}

	appExitedFile, err := appExitedFile(pod, app.Name)
	if err != nil {
		return err
	}

	defer func() {
		if pod.AfterRun() {
			// If the pod is hard killed, set the app to 'exited' state.
			// Other than this case, status file is guaranteed to be written.
			if app.State != AppStateExited {
				app.State = AppStateExited
				t, err := pod.GCMarkedTime()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Cannot get GC marked time: %v", err)
				}
				if !t.IsZero() {
					finishedAt := t.UnixNano()
					app.FinishedAt = &finishedAt
				}
			}
		}
	}()

	// Check if the app is created.
	fi, err := os.Stat(appInfoDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("cannot stat app creation file: %v", err)
		}
		return nil
	}

	app.State = AppStateCreated
	createdAt := fi.ModTime().UnixNano()
	app.CreatedAt = &createdAt

	// Check if the app is started.
	fi, err = os.Stat(appStartedFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("cannot stat app started file: %v", err)
		}
		return nil
	}

	app.State = AppStateRunning
	startedAt := fi.ModTime().UnixNano()
	app.StartedAt = &startedAt

	// Check if the app is exited.
	fi, err = os.Stat(appExitedFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("cannot stat app exited file: %v", err)
		}
		return nil
	}

	app.State = AppStateExited
	finishedAt := fi.ModTime().UnixNano()
	app.FinishedAt = &finishedAt

	// Read exit code.
	exitCode, err := readExitCode(appExitedFile)
	if err != nil {
		return fmt.Errorf("cannot read exit code: %v", err)
	}
	app.ExitCode = &exitCode

	return nil
}

func readExitCode(path string) (int, error) {
	var exitCode int

	b, err := ioutil.ReadFile(path)
	if err != nil {
		return -1, fmt.Errorf("cannot read app exited file: %v", err)
	}
	if _, err := fmt.Sscanf(string(b), "%d", &exitCode); err != nil {
		return -1, fmt.Errorf("cannot parse exit code: %v", err)
	}
	return exitCode, nil
}

func appStatusDir(pod *pkgPod.Pod) (string, error) {
	stage1RootfsPath, err := pod.Stage1RootfsPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(stage1RootfsPath, "/rkt/status"), nil
}

func appInfoDir(pod *pkgPod.Pod, appName string) (string, error) {
	return filepath.Join(pod.Path(), "/appsinfo", appName), nil
}

func appStartedFile(pod *pkgPod.Pod, appName string) (string, error) {
	statusDir, err := appStatusDir(pod)
	if err != nil {
		return "", err
	}
	return filepath.Join(statusDir, fmt.Sprintf("%s-started", appName)), nil
}

func appExitedFile(pod *pkgPod.Pod, appName string) (string, error) {
	statusDir, err := appStatusDir(pod)
	if err != nil {
		return "", err
	}
	return filepath.Join(statusDir, appName), nil
}
