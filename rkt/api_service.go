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
	"bufio"
	"container/ring"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/rkt/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/rkt/Godeps/_workspace/src/google.golang.org/grpc"
	"github.com/coreos/rkt/api/v1"
	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/store"
	"github.com/coreos/rkt/version"
)

const (
	journalctlTimeFormat = "2006-01-02 15:04:05"
	defaultEventCount    = 128 // The default number of events to keeps in memory.
)

var (
	cmdAPIService = &cobra.Command{
		Use:   "api-service [--listen-port=PORT]",
		Short: "Run API service",
		Run:   runWrapper(runAPIService),
	}

	flagAPIServiceListenClientURL string
)

func init() {
	cmdRkt.AddCommand(cmdAPIService)
	cmdAPIService.Flags().StringVar(&flagAPIServiceListenClientURL, "--listen-client-url", common.APIServiceListenClientURL, "address to listen on client API requests")
}

// eventBuffer records all the events and listeners.
// TODO(yifan): Move to separate package.
type eventBuffer struct {
	mu        sync.Mutex
	current   *ring.Ring                 // current marks the position for coming event.
	head      *ring.Ring                 // head marks the oldest event in the buffer.
	listeners map[chan struct{}]struct{} // channels of the listeners.
}

func newEventBuffer(size int) *eventBuffer {
	ring := ring.New(size)

	return &eventBuffer{
		current:   ring,
		head:      ring,
		listeners: make(map[chan struct{}]struct{}),
	}
}

// aggregateEvents returns all the events started from pointed by 'current' to the latest event.
// It also returns a pointer that points to the position of the next future event.
func (eb *eventBuffer) aggregateEvents(start *ring.Ring) (*ring.Ring, []*v1.Event) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	var events []*v1.Event
	var current *ring.Ring

	for current = start; current != eb.current; current = current.Next() {
		events = append(events, current.Value.(*v1.Event))
	}
	return current, events
}

// registerListener registers an event listener and return historicall events if
// readAllEvents is true.
// It also returns a pointer that points to the position of the next future event.
func (eb *eventBuffer) registerListener(ln chan struct{}, readAllEvents bool) (*ring.Ring, []*v1.Event) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	var events []*v1.Event

	// Copy all happend events if necessary, this needs to be done
	// at the same time as registering to avoid race.
	current := eb.current
	if readAllEvents {
		current = eb.head
	}
	for ; current != eb.current; current = current.Next() {
		events = append(events, current.Value.(*v1.Event))
	}

	// Register the listener.
	eb.listeners[ln] = struct{}{}

	return current, events
}

// deregisterListener removes the listener from the listener map.
func (eb *eventBuffer) deregisterListener(ln chan struct{}) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	delete(eb.listeners, ln)
}

// addEvent adds a new event to the event buffer, and wakes up all the listeners.
func (eb *eventBuffer) addEvent(event *v1.Event) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.current.Value = event
	eb.current = eb.current.Next()

	// When meet the buffer limit, move the buffer head to drop the oldest event.
	if eb.current == eb.head {
		eb.head = eb.head.Next()
	}

	// Broadcast.
	for ch := range eb.listeners {
		ch <- struct{}{}
	}
}

// v1APIServer implements v1.APIServer interface.
type v1APIServer struct {
	store       *store.Store
	eventBuffer *eventBuffer
}

var _ v1.PublicAPIServer = &v1APIServer{}
var _ v1.InternalAPIServer = &v1APIServer{}

func newV1APIServer() (*v1APIServer, error) {
	s, err := store.NewStore(globalFlags.Dir)
	if err != nil {
		return nil, err
	}

	return &v1APIServer{
		store:       s,
		eventBuffer: newEventBuffer(defaultEventCount),
	}, nil
}

func (s *v1APIServer) GetInfo(context.Context, *v1.GetInfoRequest) (*v1.GetInfoResponse, error) {
	return &v1.GetInfoResponse{
		Info: &v1.Info{
			RktVersion:  version.Version,
			AppcVersion: schema.AppContainerVersion.String(),
		},
	}, nil
}

type valueGetter interface {
	Get(string) (string, bool)
}

// findKeyValue returns true if the actualKVs contains all the key-value
// pais listed in requiredKVs, otherwise it returns false.
func findKeyValue(actualKVs valueGetter, requiredKVs []*v1.KeyValue) bool {
	for _, requiredKV := range requiredKVs {
		actualValue, ok := actualKVs.Get(requiredKV.Key)
		if !ok || actualValue != requiredKV.Value {
			return false
		}
	}
	return true
}

func findString(actual string, expected []string, checkFunc func(a, b string) bool) bool {
	for _, v := range expected {
		if checkFunc(actual, v) {
			return true
		}
	}
	return false
}

func stringEqual(a, b string) bool {
	return a == b
}

func containsString(actual, expected []string, checkFunc func(a, b string) bool) bool {
	for _, v := range actual {
		if findString(v, expected, checkFunc) {
			return true
		}
	}
	return false
}

// filterPod returns true if the pod doesn't satisfy the filter, which means
// it should be filtered and not be returned.
// It returns false if the filter is nil or the pod satisfies the filter, which
// means it should be returned.
func filterPod(pod *v1.Pod, manifest *schema.PodManifest, filter *v1.PodFilter) bool {
	// No filters, return directly.
	if filter == nil {
		return false
	}

	// Filter according to the state.
	if len(filter.States) > 0 {
		foundState := false
		for _, state := range filter.States {
			if pod.State == state {
				foundState = true
				break
			}
		}
		if !foundState {
			return true
		}
	}

	// Filter according to the app names.
	if len(filter.AppNames) > 0 {
		var names []string
		for _, app := range pod.Apps {
			names = append(names, app.Name)
		}
		if !containsString(names, filter.AppNames, stringEqual) {
			return true
		}
	}

	// Filter according to the image names.
	if len(filter.ImageNames) > 0 {
		var names []string
		for _, app := range pod.Apps {
			names = append(names, app.Image.Name)
		}
		if !containsString(names, filter.ImageNames, stringEqual) {
			return true
		}
	}

	// Filter according to the network names.
	if len(filter.NetworkNames) > 0 {
		var names []string
		for _, network := range pod.Networks {
			names = append(names, network.Name)
		}
		if !containsString(names, filter.NetworkNames, stringEqual) {
			return true
		}
	}

	// Filter according to the annotations.
	if len(filter.Annotations) > 0 {
		if !findKeyValue(manifest.Annotations, filter.Annotations) {
			return true
		}
	}

	return false
}

// getBasicPod returns *v1.Pod with basic pod information, it also returns a *schema.PodManifest
// object.
func getBasicPod(p *pod) (*v1.Pod, *schema.PodManifest, error) {
	// Get pod manifest.
	data, err := p.readFile("pod")
	if err != nil {
		log.Printf("Failed to read pod manifest for pod %q: %v", p.uuid, err)
		return nil, nil, err
	}

	var manifest schema.PodManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		log.Printf("Failed to unmarshal pod manifest for pod %q: %v", p.uuid, err)
		return nil, nil, err
	}

	// Get pod's state.
	var state v1.PodState
	switch p.getState() {
	case Embryo:
		state = v1.PodState_POD_STATE_EMBRYO
	case Preparing:
		state = v1.PodState_POD_STATE_PREPARING
	case AbortedPrepare:
		state = v1.PodState_POD_STATE_ABORTED_PREPARE
	case Prepared:
		state = v1.PodState_POD_STATE_PREPARED
	case Running:
		state = v1.PodState_POD_STATE_RUNNING
	case Deleting:
		state = v1.PodState_POD_STATE_DELETING
	case Exited:
		state = v1.PodState_POD_STATE_EXITED
	case Garbage:
		state = v1.PodState_POD_STATE_GARBAGE
	default:
		state = v1.PodState_POD_STATE_UNDEFINED
	}

	// Get pod's pid if it's running.
	pid := -1
	if state == v1.PodState_POD_STATE_RUNNING {
		pid, err = p.getPID()
		if err != nil {
			log.Printf("Failed to get PID for pod %q: %v", p.uuid, err)
			return nil, nil, err
		}
	}

	// Get app list.
	var apps []*v1.App
	applist, err := p.getApps()
	if err != nil {
		log.Printf("Failed to get app list for pod %q: %v", p.uuid, err)
		return nil, nil, err
	}

	for _, app := range applist {
		img := &v1.Image{
			Format: &v1.ImageFormat{
				// Only support appc image now. Even it's docker image,
				// it will be transformed to appc before storing in the
				// disk store.
				Type:    v1.ImageType_IMAGE_TYPE_APPC,
				Version: schema.AppContainerVersion.String(),
			},
			Id: app.Image.ID.String(),
			// Only image format and image ID are returned in 'ListPods()'.
		}

		apps = append(apps, &v1.App{
			Name:  app.Name.String(),
			Image: img,
			// State and exit code are not returned in 'ListPods()'.
		})
	}

	// Get network info.
	var networks []*v1.Network
	for _, n := range p.nets {
		networks = append(networks, &v1.Network{
			Name: n.NetName,
			// There will be IPv6 support soon so distinguish between v4 and v6
			Ipv4: n.IP.String(),
		})
	}

	return &v1.Pod{
		Id:       p.uuid.String(),
		Pid:      int32(pid),
		State:    state,
		Apps:     apps,
		Manifest: data,
		Networks: networks,
	}, &manifest, nil
}

func (s *v1APIServer) ListPods(ctx context.Context, request *v1.ListPodsRequest) (*v1.ListPodsResponse, error) {
	var pods []*v1.Pod
	if err := walkPods(includeMostDirs, func(p *pod) {
		// Get basic pod info.
		pod, manifest, err := getBasicPod(p)
		if err != nil { // Do not return partial pods.
			return
		}

		// Filter the pod.
		if !filterPod(pod, manifest, request.Filter) {
			pods = append(pods, pod)
		}
	}); err != nil {
		log.Printf("Failed to list pod: %v", err)
		return nil, err
	}
	return &v1.ListPodsResponse{Pods: pods}, nil
}

func (s *v1APIServer) InspectPod(ctx context.Context, request *v1.InspectPodRequest) (*v1.InspectPodResponse, error) {
	uuid, err := types.NewUUID(request.Id)
	if err != nil {
		log.Printf("Invalid pod id %q: %v", request.Id, err)
		return nil, err
	}

	p, err := getPod(uuid)
	if err != nil {
		log.Printf("Failed to get pod %q: %v", request.Id, err)
		return nil, err
	}
	defer p.Close()

	// Get basic pod info.
	pod, _, err := getBasicPod(p)
	if err != nil {
		return nil, err
	}

	statusDir, err := p.getStatusDir()
	if err != nil {
		log.Printf("Failed to get pod exit status directory: %v", err)
		return nil, err
	}

	// Fill the extra pod info that are not available in ListPods().
	for _, app := range pod.Apps {
		s, err := p.readIntFromFile(filepath.Join(statusDir, app.Name))
		if err != nil {
			if os.IsNotExist(err) {
				// If status file does not exit, that means the
				// app has not exited yet.
				app.State = v1.AppState_APP_STATE_RUNNING
			} else {
				log.Printf("Failed to read status for app %q: %v", app.Name, err)
				app.State = v1.AppState_APP_STATE_UNDEFINED
			}
			continue
		}
		app.State = v1.AppState_APP_STATE_EXITED
		app.ExitCode = int32(s)
	}
	return &v1.InspectPodResponse{Pod: pod}, nil
}

// aciInfoToV1APIImage takes an aciInfo object and construct the v1.Image
// object. It will also get and return the image manifest.
// Note that v1.Image.Manifest field is not set by this function.
func (s *v1APIServer) aciInfoToV1APIImage(aciInfo *store.ACIInfo) (*v1.Image, *schema.ImageManifest, error) {
	imgManifest, err := s.store.GetImageManifest(aciInfo.BlobKey)
	if err != nil {
		log.Printf("Failed to get image manifest for image %q: %v", aciInfo.AppName, err)
		return nil, nil, err
	}

	data, err := json.Marshal(imgManifest)
	if err != nil {
		log.Printf("Failed to marshal image manifest for image %q: %v", aciInfo.AppName, err)
		return nil, nil, err
	}

	version, ok := imgManifest.Labels.Get("version")
	if !ok {
		version = "latest"
	}

	return &v1.Image{
		Format: &v1.ImageFormat{
			// Only support appc image now. Even it's docker image,
			// it will be transformed to appc before storing in the
			// disk store.
			Type:    v1.ImageType_IMAGE_TYPE_APPC,
			Version: schema.AppContainerVersion.String(),
		},
		Id:              aciInfo.BlobKey,
		Name:            imgManifest.Name.String(),
		Version:         version,
		ImportTimestamp: aciInfo.ImportTime.Unix(),
		Manifest:        data,
	}, imgManifest, nil
}

// filterImage returns true if the image doesn't satisfy the filter, which means
// it should be filtered and not be returned.
// It returns false if the filter is nil or the pod satisfies the filter, which means
// it should be returned.
func filterImage(image *v1.Image, manifest *schema.ImageManifest, filter *v1.ImageFilter) bool {
	// No filters, return directly.
	if filter == nil {
		return false
	}

	// Filter according to the IDs.
	if len(filter.Ids) > 0 {
		if !findString(image.Id, filter.Ids, stringEqual) {
			return true
		}
	}

	// Filter according to the image name prefixes.
	if len(filter.Prefixes) > 0 {
		if !findString(image.Name, filter.Prefixes, strings.HasPrefix) {
			return true
		}
	}

	// Filter according to the image base name.
	if len(filter.BaseNames) > 0 {
		if !findString(image.Name, filter.BaseNames, func(a, b string) bool { return path.Base(a) == b }) {
			return true
		}
	}

	// Filter according to the image keywords.
	if len(filter.Keywords) > 0 {
		if !findString(image.Name, filter.Keywords, strings.Contains) {
			return true
		}
	}

	// Filter according to the imported time.
	if filter.ImportedAfter > 0 {
		if image.ImportTimestamp <= filter.ImportedAfter {
			return true
		}
	}
	if filter.ImportedBefore > 0 {
		if image.ImportTimestamp >= filter.ImportedBefore {
			return true
		}
	}

	// Filter according to the image labels.
	if len(filter.Labels) > 0 {
		if !findKeyValue(manifest.Labels, filter.Labels) {
			return true
		}
	}

	// Filter according to the annotations.
	if len(filter.Annotations) > 0 {
		if !findKeyValue(manifest.Annotations, filter.Annotations) {
			return true
		}
	}

	return false
}

func (s *v1APIServer) ListImages(ctx context.Context, request *v1.ListImagesRequest) (*v1.ListImagesResponse, error) {
	aciInfos, err := s.store.GetAllACIInfos(nil, false)
	if err != nil {
		log.Printf("Failed to get all ACI infos: %v", err)
		return nil, err
	}

	var images []*v1.Image
	for _, aciInfo := range aciInfos {
		image, manifest, err := s.aciInfoToV1APIImage(aciInfo)
		if err != nil {
			continue
		}

		// Filter images.
		if !filterImage(image, manifest, request.Filter) {
			images = append(images, image)
		}
	}

	return &v1.ListImagesResponse{Images: images}, nil
}

func (s *v1APIServer) InspectImage(ctx context.Context, request *v1.InspectImageRequest) (*v1.InspectImageResponse, error) {
	aciInfo, err := s.store.GetACIInfoWithBlobKey(request.Id)
	if err != nil {
		log.Printf("Failed to get ACI info for image ID %q: %v", request.Id, err)
		return nil, err
	}

	image, _, err := s.aciInfoToV1APIImage(aciInfo)
	if err != nil {
		return nil, err
	}

	return &v1.InspectImageResponse{Image: image}, nil
}

// filterEvents returns true if the event doesn't satisfy the filter, which means
// it should be filtered and not be returned.
// It returns false if the filter is nil or the event satisfies the filter, which
// means it should be returned.
func filterEvent(event *v1.Event, filter *v1.EventFilter) bool {
	// No filters, return directly.
	if filter == nil {
		return false
	}

	// Filter according to the types.
	if len(filter.Types) > 0 {
		foundType := false
		for _, v := range filter.Types {
			if event.Type == v {
				foundType = true
				break
			}
		}
		if !foundType {
			return true
		}
	}

	// Filter according to the ids.
	if len(filter.Ids) > 0 {
		if !findString(event.Id, filter.Ids, stringEqual) {
			return true
		}
	}

	// Filter according to the names.
	if len(filter.Names) > 0 {
		if !findString(event.From, filter.Names, stringEqual) {
			return true
		}
	}

	// Filter according to the since_time.
	if filter.SinceTime > 0 {
		if event.Time < filter.SinceTime {
			return true
		}
	}

	// Filter according to the until_time.
	if filter.UntilTime > 0 {
		if event.Time > filter.UntilTime {
			return true
		}
	}

	return false
}

func sendEvents(server v1.PublicAPI_ListenEventsServer, events []*v1.Event, filter *v1.EventFilter) error {
	var filteredEvents []*v1.Event

	for _, e := range events {
		if !filterEvent(e, filter) {
			filteredEvents = append(filteredEvents, e)
		}
	}

	if len(filteredEvents) > 0 {
		if err := server.Send(&v1.ListenEventsResponse{Events: filteredEvents}); err != nil {
			log.Printf("Failed to send events: %v", err)
			return err
		}
	}
	return nil
}

func (s *v1APIServer) ListenEvents(request *v1.ListenEventsRequest, server v1.PublicAPI_ListenEventsServer) error {
	filter := request.Filter
	eb := s.eventBuffer

	// Register the listener and get all history events if necessary.
	readAllEvents := (filter != nil && filter.SinceTime > 0)
	listener := make(chan struct{})

	current, events := eb.registerListener(listener, readAllEvents)
	defer eb.deregisterListener(listener)

	// After getting any history events, send them.
	if err := sendEvents(server, events, filter); err != nil {
		return err
	}

	// Set up timer if 'until_time' is given.
	timer := make(<-chan time.Time)
	if filter != nil && filter.UntilTime > 0 {
		// When duration is negative, timer will return immediately.
		duration := time.Duration(filter.UntilTime-time.Now().Unix()) * time.Second
		timer = time.After(duration)
	}

	// If 'until_time' is reached, send all events and return.
	// If new events happen, send them.
	for {
		select {
		case <-timer:
			current, events = eb.aggregateEvents(current)
			return sendEvents(server, events, filter)
		case <-listener:
			current, events = eb.aggregateEvents(current)
			if err := sendEvents(server, events, filter); err != nil {
				return err
			}
		}
	}
}

// TODO(yifan): Replace forking/execing 'journalctl' with journal API.
func (s *v1APIServer) GetLogs(request *v1.GetLogsRequest, server v1.PublicAPI_GetLogsServer) error {
	cmd := exec.Command("journalctl")
	if request.PodId == "" {
		return fmt.Errorf("No pod id")
	}
	if request.Lines > 0 {
		cmd.Args = append(cmd.Args, "-n", strconv.Itoa(int(request.Lines)))
	}
	if request.Follow {
		cmd.Args = append(cmd.Args, "-f")
	}
	if request.SinceTime > 0 {
		since := time.Unix(request.SinceTime, 0)
		cmd.Args = append(cmd.Args, "--since", since.Format(journalctlTimeFormat))
	}
	if request.UntilTime > 0 {
		until := time.Unix(request.UntilTime, 0)
		cmd.Args = append(cmd.Args, "--until", until.Format(journalctlTimeFormat))
	}
	cmd.Args = append(cmd.Args, "-M", "rkt-"+request.PodId)

	if request.AppName != "" {
		cmd.Args = append(cmd.Args, "-u", request.AppName)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	cmd.Start()

	// If 'follow' is true, then accumulates the logs every 100ms.
	tick := time.Tick(time.Millisecond * 100)
	lineCh := make(chan string, 1024)
	done := make(chan struct{})

	// Start a goroutine to accumulate the logs instead of sending requests
	// for each log line.
	go func() {
		var lines []string
		for {
			select {
			case <-tick:
				// Don't send empty responses.
				if len(lines) == 0 {
					continue
				}

				if err := server.Send(&v1.GetLogsResponse{Lines: lines}); err != nil {
					log.Printf("Failed to send response: %v", err)
					return
				}
				lines = lines[:0]
			case line, ok := <-lineCh:
				if !ok {
					// Channel is closed, this means there's no future lines.
					if err := server.Send(&v1.GetLogsResponse{Lines: lines}); err != nil {
						log.Printf("Failed to send response: %v", err)
					}
					close(done)
					return
				}
				lines = append(lines, line)
			}
		}
	}()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		// TODO(yifan): Truncate the line prefix to return pure logs from the apps.
		lineCh <- scanner.Text()
	}
	close(lineCh)

	// Wait for the finish of the send before leaving the function to avoid
	// 'PublicAPI_GetLogsServer' being empty.
	<-done

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

// TODO(yifan): Maybe use REST?
// Note that AddEvent() can be block for a while. The sender should set a timeout for the request.
func (s *v1APIServer) AddEvent(ctx context.Context, request *v1.AddEventRequest) (*v1.AddEventResponse, error) {
	if request.Event == nil {
		log.Printf("No events in the request")
		return nil, fmt.Errorf("No events in the request")
	}
	s.eventBuffer.addEvent(request.Event)
	return &v1.AddEventResponse{}, nil
}

func runAPIService(cmd *cobra.Command, args []string) (exit int) {
	log.Print("API service starting...")

	unixl, err := common.UnixListener(common.APIServiceEventSock)
	if err != nil {
		log.Print(err.Error())
		return 1
	}
	defer unixl.Close()

	tcpl, err := net.Listen("tcp", flagAPIServiceListenClientURL)
	if err != nil {
		log.Print("Error listening on url: %v", flagAPIServiceListenClientURL, err)
	}
	defer tcpl.Close()

	internalServer := grpc.NewServer()
	publicServer := grpc.NewServer() // TODO(yifan): Add TLS credential option.

	v1APIServer, err := newV1APIServer()
	if err != nil {
		log.Print("Failed to create API service: %v", err)
		return 1
	}

	v1.RegisterInternalAPIServer(internalServer, v1APIServer)
	v1.RegisterPublicAPIServer(publicServer, v1APIServer)

	go internalServer.Serve(unixl)
	go publicServer.Serve(tcpl)

	log.Print("API service running...")

	signal.Notify(exitCh, syscall.SIGINT, syscall.SIGTERM)
	<-exitCh

	log.Print("API service exiting...")

	return
}
