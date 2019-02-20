# Protocol Documentation
<a name="top"/>

## Table of Contents

- [api.proto](#api.proto)
    - [App](#v1alpha.App)
    - [Event](#v1alpha.Event)
    - [EventFilter](#v1alpha.EventFilter)
    - [GetInfoRequest](#v1alpha.GetInfoRequest)
    - [GetInfoResponse](#v1alpha.GetInfoResponse)
    - [GetLogsRequest](#v1alpha.GetLogsRequest)
    - [GetLogsResponse](#v1alpha.GetLogsResponse)
    - [GlobalFlags](#v1alpha.GlobalFlags)
    - [Image](#v1alpha.Image)
    - [ImageFilter](#v1alpha.ImageFilter)
    - [ImageFormat](#v1alpha.ImageFormat)
    - [Info](#v1alpha.Info)
    - [InspectImageRequest](#v1alpha.InspectImageRequest)
    - [InspectImageResponse](#v1alpha.InspectImageResponse)
    - [InspectPodRequest](#v1alpha.InspectPodRequest)
    - [InspectPodResponse](#v1alpha.InspectPodResponse)
    - [KeyValue](#v1alpha.KeyValue)
    - [ListImagesRequest](#v1alpha.ListImagesRequest)
    - [ListImagesResponse](#v1alpha.ListImagesResponse)
    - [ListPodsRequest](#v1alpha.ListPodsRequest)
    - [ListPodsResponse](#v1alpha.ListPodsResponse)
    - [ListenEventsRequest](#v1alpha.ListenEventsRequest)
    - [ListenEventsResponse](#v1alpha.ListenEventsResponse)
    - [Network](#v1alpha.Network)
    - [Pod](#v1alpha.Pod)
    - [PodFilter](#v1alpha.PodFilter)
  
    - [AppState](#v1alpha.AppState)
    - [EventType](#v1alpha.EventType)
    - [ImageType](#v1alpha.ImageType)
    - [PodState](#v1alpha.PodState)
  
  
    - [PublicAPI](#v1alpha.PublicAPI)
  

- [Scalar Value Types](#scalar-value-types)



<a name="api.proto"/>
<p align="right"><a href="#top">Top</a></p>

## api.proto



<a name="v1alpha.App"/>

### App
App describes the information of an app that&#39;s running in a pod.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  | Name of the app, required. |
| image | [Image](#v1alpha.Image) |  | Image used by the app, required. However, this may only contain the image id if it is returned by ListPods(). |
| state | [AppState](#v1alpha.AppState) |  | State of the app. optional, non-empty only if it&#39;s returned by InspectPod(). |
| exit_code | [sint32](#sint32) |  | Exit code of the app. optional, only valid if it&#39;s returned by InspectPod() and the app has already exited. |
| annotations | [KeyValue](#v1alpha.KeyValue) | repeated | Annotations for this app. |






<a name="v1alpha.Event"/>

### Event
Event describes the events that will be received via ListenEvents().


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| type | [EventType](#v1alpha.EventType) |  | Type of the event, required. |
| id | [string](#string) |  | ID of the subject that causes the event, required. If the event is a pod or app event, the id is the pod&#39;s uuid. If the event is an image event, the id is the image&#39;s id. |
| from | [string](#string) |  | Name of the subject that causes the event, required. If the event is a pod event, the name is the pod&#39;s name. If the event is an app event, the name is the app&#39;s name. If the event is an image event, the name is the image&#39;s name. |
| time | [int64](#int64) |  | Timestamp of when the event happens, it is the seconds since epoch, required. |
| data | [KeyValue](#v1alpha.KeyValue) | repeated | Data of the event, in the form of key-value pairs, optional. |






<a name="v1alpha.EventFilter"/>

### EventFilter
EventFilter defines the condition that the returned events needs to satisfy in ListImages().
The condition are combined by &#39;AND&#39;.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| types | [EventType](#v1alpha.EventType) | repeated | If not empty, then only returns the events that have the listed types. |
| ids | [string](#string) | repeated | If not empty, then only returns the events whose &#39;id&#39; is included in the listed ids. |
| names | [string](#string) | repeated | If not empty, then only returns the events whose &#39;from&#39; is included in the listed names. |
| since_time | [int64](#int64) |  | If set, then only returns the events after this timestamp. If the server starts after since_time, then only the events happened after the start of the server will be returned. If since_time is a future timestamp, then no events will be returned until that time. |
| until_time | [int64](#int64) |  | If set, then only returns the events before this timestamp. If it is a future timestamp, then the event stream will be closed at that moment. |






<a name="v1alpha.GetInfoRequest"/>

### GetInfoRequest
Request for GetInfo().






<a name="v1alpha.GetInfoResponse"/>

### GetInfoResponse
Response for GetInfo().


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| info | [Info](#v1alpha.Info) |  | Required. |






<a name="v1alpha.GetLogsRequest"/>

### GetLogsRequest
Request for GetLogs().


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| pod_id | [string](#string) |  | ID of the pod which we will get logs from, required. |
| app_name | [string](#string) |  | Name of the app within the pod which we will get logs from, optional. If not set, then the logs of all the apps within the pod will be returned. |
| lines | [int32](#int32) |  | Number of most recent lines to return, optional. |
| follow | [bool](#bool) |  | If true, then a response stream will not be closed, and new log response will be sent via the stream, default is false. |
| since_time | [int64](#int64) |  | If set, then only the logs after the timestamp will be returned, optional. |
| until_time | [int64](#int64) |  | If set, then only the logs before the timestamp will be returned, optional. |






<a name="v1alpha.GetLogsResponse"/>

### GetLogsResponse
Response for GetLogs().


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| lines | [string](#string) | repeated | List of the log lines that returned, optional as the response can contain no logs. |






<a name="v1alpha.GlobalFlags"/>

### GlobalFlags
GlobalFlags describes the flags that passed to rkt api service when it is launched.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| dir | [string](#string) |  | Data directory. |
| system_config_dir | [string](#string) |  | System configuration directory. |
| local_config_dir | [string](#string) |  | Local configuration directory. |
| user_config_dir | [string](#string) |  | User configuration directory. |
| insecure_flags | [string](#string) |  | Insecure flags configurates what security features to disable. |
| trust_keys_from_https | [bool](#bool) |  | Whether to automatically trust gpg keys fetched from https |






<a name="v1alpha.Image"/>

### Image
Image describes the image&#39;s information.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| base_format | [ImageFormat](#v1alpha.ImageFormat) |  | Base format of the image, required. This indicates the original format for the image as nowadays all the image formats will be transformed to ACI. |
| id | [string](#string) |  | ID of the image, a string that can be used to uniquely identify the image, e.g. sha512 hash of the ACIs, required. |
| name | [string](#string) |  | Name of the image in the image manifest, e.g. &#39;coreos.com/etcd&#39;, optional. |
| version | [string](#string) |  | Version of the image, e.g. &#39;latest&#39;, &#39;2.0.10&#39;, optional. |
| import_timestamp | [int64](#int64) |  | Timestamp of when the image is imported, it is the seconds since epoch, optional. |
| manifest | [bytes](#bytes) |  | JSON-encoded byte array that represents the image manifest, optional. |
| size | [int64](#int64) |  | Size is the size in bytes of this image in the store. |
| annotations | [KeyValue](#v1alpha.KeyValue) | repeated | Annotations on this image. |
| labels | [KeyValue](#v1alpha.KeyValue) | repeated | Labels of this image. |






<a name="v1alpha.ImageFilter"/>

### ImageFilter
ImageFilter defines the condition that the returned images need to satisfy in ListImages().
The conditions are combined by &#39;AND&#39;, and different filters are combined by &#39;OR&#39;.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ids | [string](#string) | repeated | If not empty, the images that have any of the ids will be returned. |
| prefixes | [string](#string) | repeated | if not empty, the images that have any of the prefixes in the name will be returned. |
| base_names | [string](#string) | repeated | If not empty, the images that have any of the base names will be returned. For example, both &#39;coreos.com/etcd&#39; and &#39;k8s.io/etcd&#39; will be returned if &#39;etcd&#39; is included, however &#39;k8s.io/etcd-backup&#39; will not be returned. |
| keywords | [string](#string) | repeated | If not empty, the images that have any of the keywords in the name will be returned. For example, both &#39;kubernetes-etcd&#39;, &#39;etcd:latest&#39; will be returned if &#39;etcd&#39; is included, |
| labels | [KeyValue](#v1alpha.KeyValue) | repeated | If not empty, the images that have all of the labels will be returned. |
| imported_after | [int64](#int64) |  | If set, the images that are imported after this timestamp will be returned. |
| imported_before | [int64](#int64) |  | If set, the images that are imported before this timestamp will be returned. |
| annotations | [KeyValue](#v1alpha.KeyValue) | repeated | If not empty, the images that have all of the annotations will be returned. |
| full_names | [string](#string) | repeated | If not empty, the images that have any of the exact full names will be returned. |






<a name="v1alpha.ImageFormat"/>

### ImageFormat
ImageFormat defines the format of the image.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| type | [ImageType](#v1alpha.ImageType) |  | Type of the image, required. |
| version | [string](#string) |  | Version of the image format, required. |






<a name="v1alpha.Info"/>

### Info
Info describes the information of rkt on the machine.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| rkt_version | [string](#string) |  | Version of rkt, required, in the form of Semantic Versioning 2.0.0 (https://semver.org/). |
| appc_version | [string](#string) |  | Version of appc, required, in the form of Semantic Versioning 2.0.0 (https://semver.org/). |
| api_version | [string](#string) |  | Latest version of the api that&#39;s supported by the service, required, in the form of Semantic Versioning 2.0.0 (https://semver.org/). |
| global_flags | [GlobalFlags](#v1alpha.GlobalFlags) |  | The global flags that passed to the rkt api service when it&#39;s launched. |






<a name="v1alpha.InspectImageRequest"/>

### InspectImageRequest
Request for InspectImage().


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | Required. |






<a name="v1alpha.InspectImageResponse"/>

### InspectImageResponse
Response for InspectImage().


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| image | [Image](#v1alpha.Image) |  | Required. |






<a name="v1alpha.InspectPodRequest"/>

### InspectPodRequest
Request for InspectPod().


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | ID of the pod which we are querying status for, required. |






<a name="v1alpha.InspectPodResponse"/>

### InspectPodResponse
Response for InspectPod().


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| pod | [Pod](#v1alpha.Pod) |  | Required. |






<a name="v1alpha.KeyValue"/>

### KeyValue



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Key | [string](#string) |  | Key part of the key-value pair. |
| value | [string](#string) |  | Value part of the key-value pair. |






<a name="v1alpha.ListImagesRequest"/>

### ListImagesRequest
Request for ListImages().


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| filters | [ImageFilter](#v1alpha.ImageFilter) | repeated | Optional. |
| detail | [bool](#bool) |  | Optional. |






<a name="v1alpha.ListImagesResponse"/>

### ListImagesResponse
Response for ListImages().


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| images | [Image](#v1alpha.Image) | repeated | Required. |






<a name="v1alpha.ListPodsRequest"/>

### ListPodsRequest
Request for ListPods().


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| filters | [PodFilter](#v1alpha.PodFilter) | repeated | Optional. |
| detail | [bool](#bool) |  | Optional. |






<a name="v1alpha.ListPodsResponse"/>

### ListPodsResponse
Response for ListPods().


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| pods | [Pod](#v1alpha.Pod) | repeated | Required. |






<a name="v1alpha.ListenEventsRequest"/>

### ListenEventsRequest
Request for ListenEvents().


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| filter | [EventFilter](#v1alpha.EventFilter) |  | Optional. |






<a name="v1alpha.ListenEventsResponse"/>

### ListenEventsResponse
Response for ListenEvents().


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| events | [Event](#v1alpha.Event) | repeated | Aggregate multiple events to reduce round trips, optional as the response can contain no events. |






<a name="v1alpha.Network"/>

### Network
Network describes the network information of a pod.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  | Name of the network that a pod belongs to, required. |
| ipv4 | [string](#string) |  | Pod&#39;s IPv4 address within the network, optional if IPv6 address is given. |
| ipv6 | [string](#string) |  | Pod&#39;s IPv6 address within the network, optional if IPv4 address is given. |






<a name="v1alpha.Pod"/>

### Pod
Pod describes a pod&#39;s information.
If a pod is in Embryo, Preparing, AbortedPrepare state,
only id and state will be returned.

If a pod is in other states, the pod manifest and
apps will be returned when &#39;detailed&#39; is true in the request.

A valid pid of the stage1 process of the pod will be returned
if the pod is Running has run once.

Networks are only returned when a pod is in Running.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | ID of the pod, in the form of a UUID. |
| pid | [sint32](#sint32) |  | PID of the stage1 process of the pod. |
| state | [PodState](#v1alpha.PodState) |  | State of the pod. |
| apps | [App](#v1alpha.App) | repeated | List of apps in the pod. |
| networks | [Network](#v1alpha.Network) | repeated | Network information of the pod. Note that a pod can be in multiple networks. |
| manifest | [bytes](#bytes) |  | JSON-encoded byte array that represents the pod manifest of the pod. |
| annotations | [KeyValue](#v1alpha.KeyValue) | repeated | Annotations on this pod. |
| cgroup | [string](#string) |  | Cgroup of the pod, empty if the pod is not running. |
| created_at | [int64](#int64) |  | Timestamp of when the pod is created, nanoseconds since epoch. Zero if the pod is not created. |
| started_at | [int64](#int64) |  | Timestamp of when the pod is started, nanoseconds since epoch. Zero if the pod is not started. |
| gc_marked_at | [int64](#int64) |  | Timestamp of when the pod is moved to exited-garbage/garbage, in nanoseconds since epoch. Zero if the pod is not moved to exited-garbage/garbage yet. |






<a name="v1alpha.PodFilter"/>

### PodFilter
PodFilter defines the condition that the returned pods need to satisfy in ListPods().
The conditions are combined by &#39;AND&#39;, and different filters are combined by &#39;OR&#39;.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ids | [string](#string) | repeated | If not empty, the pods that have any of the ids will be returned. |
| states | [PodState](#v1alpha.PodState) | repeated | If not empty, the pods that have any of the states will be returned. |
| app_names | [string](#string) | repeated | If not empty, the pods that all of the apps will be returned. |
| image_ids | [string](#string) | repeated | If not empty, the pods that have all of the images(in the apps) will be returned |
| network_names | [string](#string) | repeated | If not empty, the pods that are in all of the networks will be returned. |
| annotations | [KeyValue](#v1alpha.KeyValue) | repeated | If not empty, the pods that have all of the annotations will be returned. |
| cgroups | [string](#string) | repeated | If not empty, the pods whose cgroup are listed will be returned. |
| pod_sub_cgroups | [string](#string) | repeated | If not empty, the pods whose these cgroup belong to will be returned. i.e. the pod&#39;s cgroup is a prefix of the specified cgroup |





 


<a name="v1alpha.AppState"/>

### AppState
AppState defines the possible states of the app.

| Name | Number | Description |
| ---- | ------ | ----------- |
| APP_STATE_UNDEFINED | 0 |  |
| APP_STATE_RUNNING | 1 |  |
| APP_STATE_EXITED | 2 |  |



<a name="v1alpha.EventType"/>

### EventType
EventType defines the type of the events that will be received via ListenEvents().

| Name | Number | Description |
| ---- | ------ | ----------- |
| EVENT_TYPE_UNDEFINED | 0 |  |
| EVENT_TYPE_POD_PREPARED | 1 | Pod events. |
| EVENT_TYPE_POD_PREPARE_ABORTED | 2 |  |
| EVENT_TYPE_POD_STARTED | 3 |  |
| EVENT_TYPE_POD_EXITED | 4 |  |
| EVENT_TYPE_POD_GARBAGE_COLLECTED | 5 |  |
| EVENT_TYPE_APP_STARTED | 6 | App events. |
| EVENT_TYPE_APP_EXITED | 7 | (XXX)yifan: Maybe also return exit code in the event object? |
| EVENT_TYPE_IMAGE_IMPORTED | 8 | Image events. |
| EVENT_TYPE_IMAGE_REMOVED | 9 |  |



<a name="v1alpha.ImageType"/>

### ImageType
ImageType defines the supported image type.

| Name | Number | Description |
| ---- | ------ | ----------- |
| IMAGE_TYPE_UNDEFINED | 0 |  |
| IMAGE_TYPE_APPC | 1 |  |
| IMAGE_TYPE_DOCKER | 2 |  |
| IMAGE_TYPE_OCI | 3 |  |



<a name="v1alpha.PodState"/>

### PodState
PodState defines the possible states of the pod.
See https://github.com/rkt/rkt/blob/master/Documentation/devel/pod-lifecycle.md for a detailed
explanation of each state.

| Name | Number | Description |
| ---- | ------ | ----------- |
| POD_STATE_UNDEFINED | 0 |  |
| POD_STATE_EMBRYO | 1 | States before the pod is running. Pod is created, ready to entering &#39;preparing&#39; state. |
| POD_STATE_PREPARING | 2 | Pod is being prepared. On success it will become &#39;prepared&#39;, otherwise it will become &#39;aborted prepared&#39;. |
| POD_STATE_PREPARED | 3 | Pod has been successfully prepared, ready to enter &#39;running&#39; state. it can also enter &#39;deleting&#39; if it&#39;s garbage collected before running. |
| POD_STATE_RUNNING | 4 | State that indicates the pod is running. Pod is running, when it exits, it will become &#39;exited&#39;. |
| POD_STATE_ABORTED_PREPARE | 5 | States that indicates the pod is exited, and will never run. Pod failed to prepare, it will only be garbage collected and will never run again. |
| POD_STATE_EXITED | 6 | Pod has exited, it now can be garbage collected. |
| POD_STATE_DELETING | 7 | Pod is being garbage collected, after that it will enter &#39;garbage&#39; state. |
| POD_STATE_GARBAGE | 8 | Pod is marked as garbage collected, it no longer exists on the machine. |


 

 


<a name="v1alpha.PublicAPI"/>

### PublicAPI
PublicAPI defines the read-only APIs that will be supported.
These will be handled over TCP sockets.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| GetInfo | [GetInfoRequest](#v1alpha.GetInfoRequest) | [GetInfoResponse](#v1alpha.GetInfoRequest) | GetInfo gets the rkt&#39;s information on the machine. |
| ListPods | [ListPodsRequest](#v1alpha.ListPodsRequest) | [ListPodsResponse](#v1alpha.ListPodsRequest) | ListPods lists rkt pods on the machine. |
| InspectPod | [InspectPodRequest](#v1alpha.InspectPodRequest) | [InspectPodResponse](#v1alpha.InspectPodRequest) | InspectPod gets detailed pod information of the specified pod. |
| ListImages | [ListImagesRequest](#v1alpha.ListImagesRequest) | [ListImagesResponse](#v1alpha.ListImagesRequest) | ListImages lists the images on the machine. |
| InspectImage | [InspectImageRequest](#v1alpha.InspectImageRequest) | [InspectImageResponse](#v1alpha.InspectImageRequest) | InspectImage gets the detailed image information of the specified image. |
| ListenEvents | [ListenEventsRequest](#v1alpha.ListenEventsRequest) | [ListenEventsResponse](#v1alpha.ListenEventsRequest) | ListenEvents listens for the events, it will return a response stream that will contain event objects. |
| GetLogs | [GetLogsRequest](#v1alpha.GetLogsRequest) | [GetLogsResponse](#v1alpha.GetLogsRequest) | GetLogs gets the logs for a pod, if the app is also specified, then only the logs of the app will be returned. If &#39;follow&#39; in the &#39;GetLogsRequest&#39; is set to &#39;true&#39;, then the response stream will not be closed after the first response, the future logs will be sent via the stream. |

 



## Scalar Value Types

| .proto Type | Notes | C++ Type | Java Type | Python Type |
| ----------- | ----- | -------- | --------- | ----------- |
| <a name="double" /> double |  | double | double | float |
| <a name="float" /> float |  | float | float | float |
| <a name="int32" /> int32 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint32 instead. | int32 | int | int |
| <a name="int64" /> int64 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint64 instead. | int64 | long | int/long |
| <a name="uint32" /> uint32 | Uses variable-length encoding. | uint32 | int | int/long |
| <a name="uint64" /> uint64 | Uses variable-length encoding. | uint64 | long | int/long |
| <a name="sint32" /> sint32 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int32s. | int32 | int | int |
| <a name="sint64" /> sint64 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int64s. | int64 | long | int/long |
| <a name="fixed32" /> fixed32 | Always four bytes. More efficient than uint32 if values are often greater than 2^28. | uint32 | int | int |
| <a name="fixed64" /> fixed64 | Always eight bytes. More efficient than uint64 if values are often greater than 2^56. | uint64 | long | int/long |
| <a name="sfixed32" /> sfixed32 | Always four bytes. | int32 | int | int |
| <a name="sfixed64" /> sfixed64 | Always eight bytes. | int64 | long | int/long |
| <a name="bool" /> bool |  | bool | boolean | boolean |
| <a name="string" /> string | A string must always contain UTF-8 encoded or 7-bit ASCII text. | string | String | str/unicode |
| <a name="bytes" /> bytes | May contain any arbitrary sequence of bytes. | string | ByteString | str |

