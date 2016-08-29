# Reference (distribution) based image access on a CAS for both appc ACI and OCI images

## Introduction

Docker images can be fetched using a docker image name, every image can have multiple images names associated (references).
Also the OCI image spec will probably use this reference based logic for their (to be defined) distribution methods.

The appc spec could be used using a reference like approach like the above but currently the rkt aci store uses a mix of reference like (for file, http, docker) and name/labels based (an appc image string) logic.
This proposal tries to define a reference image access for every kind of image input string (that is a distribution, see below) and a CAS approach storing image data (blobs). This will fix some inconsistent behaviors and open issues relative to the current store logic.

## Definitions

### Distribution

An image can be retrieved in different ways. The way to fetch an image is called *distribution*.
(https://github.com/coreos/rkt/pull/2953) introduces the distribution concept. Every distribution will have a distribution URI. In this way using only one string (no multiple fields or options) an user can easily ask for a specific image.

### Image

* OCI: In OCI an image is made of differente blobs (manifest, config, layers). The main blob that defines an image is its manifest. The manifest is a content addressable object. The OCI image manifest contains the digests list of the layers and the config.
* Appc ACI: In appc an image is the ACI. The ACI is a content addressable object. It contains the image rootfs and the manifest. An ACI can have dependencies, these dependencies will form a DAG (direct acyclic graph) and are defined in the ACI's manifest by name and labels so image discovery (appc name based distribution) is required to fetch the dependencies.

So every image can be primarily accessed by its digest. For an ACI is the ACI digest, for OCI images is the manifest digest.

### Reference

A reference is a string associated to an image.

For example, in docker (since actually the unique distribution method is the docker registry) the reference is [REGISTRY_HOST[:REGISTRY_PORT]/]NAME[:TAG] abbreviated to NAME[:TAG] if the registry is the default one (registry-1.docker.io)

Instead, in rkt, there are multiple distribution methods: The currently supported are:

* Appc image name: example.com/app01:v.1.0
* File
* Http URL
* Docker URL: docker:// (it currently uses docker2aci to fetch from a docker registry and convert docker v1, v2, oci images)

In future, additional distribution methods could be introduced for both appc and OCI image specs.

### Image access

As seen above an image can be accessed in two ways:
* By its digest
* By a reference (a ditribution string). An image can have one or more references.


## Current rkt ACI store state

Currently the rkt ACI store is made as a local discoverable image store that mixes a reference based methods for some distributions and a local discovery for appc distribution:

Images can be accessed in different ways:
* via their image hash (digest in OCI terms).
* based on the provided distribution string (https://github.com/coreos/rkt/pull/2953):
    * For file, http, docker url a reference based approach is used (looking at the store remote table)
    * For appc image string the store GetACI(name, labels) function is used. This function will read the ACI manifest name and labels entries to find the best matching image.

While using a different approach for appc image string (name + labels), looked like a good approach and did its job quite well, different inconsistent behaviors and downsides have emerged:

* Doing its own app string local discovery, it can happen that the provided images can be different from the one that will be fetched via discovery. To minimize this the store will find the best matching ACI using the ACI import time and if the image was discovered without providing a version label (latest pattern).
* You can fetch an image by any distribution type (aciarchive aks via file, http URLS) and access it by app name,labels (appc distribution). This has some strange effects. Some examples:

Suppose an image containing in its manifest the name `exmaple.com/app01` and some labels like `version`, `os`, `arch`, `anotherlabel01`, `anotherlabel02`.

* Fetch image by appc distribution. For example you can fetch `example.com/app01,version=v1.0.0,os=linux,arch=amd64`. But then from the store you'll fetch the same image using multiple combinations of the image manifest name and labels like:
```
example.com/app01
example.com/app01,version=v1.0.0
example.com/app01,os=linux
example.com/app01,arch=amd64
example.com/app01,version=v1.0.0,os=linux,arch=amd64
example.com/app01,anotherlabel01=value01
```

So this is a different behavior than the one provided by appc discovery. For example is not predicatable that using `example.com/app01` will return any image (discovery may not return an image when only the app name is provided) or the same image (and not another one).

The same happens if you fetch an image via an aci archive distribution.

In addition the store saves, in the remote table, two different kind of information:
 * Fetch url (http://, docker://) -> hash relation. This is a reference to an ACI.
 * Http (and in future docker2aci) caching information (Etag, cache-control max-age). These are used by the fetchers when going to the remote (http, docker2aci) and should livello outsider the image store.

This is causing some issues and bad UX:

* The references don't have a first class role and are not showed during image listing.

* Removing an image can be done by its hash or ACI app name. Using the ACI app name (the name inside the ACI manifest) is a source of confusion (there can be multiple app with the same name, the name saved in the manifest desn't match the string used for fetching the image like for file, http and docker images).

Multiple other issues that will be difficult to fix with the current store logic:

* Problems handling the latest pattern: https://github.com/coreos/rkt/issues/2276
* Also if a bad pattern and against the spec, with rkt using --insecure-options=image an image with labels different from the required ones in the appc image string will be fetched in the store. When asking for it, the GetACI methods won’t find any image.

Given that a refrence + CAS logic will be needed anyway to support native docker v2.2 manifest and OCI images, this proposal tries to apply the same logic also for appc ACI handling.


## reference based image access on a CAS for both appc ACI and OCI images proposal

The proposal is to use a reference (distribution) based image access on a CAS for multiple image types.

Blobs and a references will be stored in an unified local store (for all the image formats). This start from the assumption that digest collisions have a very lower probability to happen (like in git).

Given the above distribution concept becomes natural to use the distribution string (URI or user friendly string) as a reference.

With this logic to access an image from the store only few methods (GetRef and ReadBlob) are needed. These methos can be the unique methods that should be provided by a readonly store (assumong there's no need to query it in an indexed way).

Instead a RW store needs additional methods and logic to be easily and fastly querable and save additional fields to an image.

This is the logic chosen to implement this proposal:

* Implement a basic read write CAS + reference store with no hidden logic, with only these additions:
 * Store the blob mediaType (to avoid guessing the blob type and for easier querying of all the blobs by type).
 * Store the blob size and import time.
 * Some methods to set additional custom data to a blob
 * Keep just basic consistency avoiding setting a ref on an non existent blob and removing a blob with references.

All the additional logic needed to easily write, update, query images will be done by specific methods (that will in part be different for ACI and OCI images) that will use the above store facilities:

* Save image informations setting additional custom data to a blob:
 * Common to all image types: for example last used time.
 * Image format specific: for example ACI signing information.
* Fast retrieval of blobs, refs, images information.
* OCI and ACI images garbage collection options (inside `rkt image gc`):
 * Since OCI images are are defined by a manifest that links other objects (layers and config) and these objects can be shared between multiple manifests it's not possible to remove these objects when removing the manifest. There’s the need to clean unreferenced layers inside a Garbage Collection logic.
 * For the same reason also ACI images can be a dependency of other ACIs, so a Garbage Collection is required.

In addition, for ACIs, there's the need to cache their manifest to avoid walking the ACI tar every time a manifest is needed, this is done with an ACI manifest cache.

The images in the store can be managed using the ref or the image digest. There won’t be any store logic like the previous (GetACI) that will read the name and labels from the image manifest and use them to choose an image.

Since a reference can be to different image formats (OCI, ACI) the store and also the image digest (OCI manifest or ACI) can be used to access the store.

An unique store will be used for both refs and blobs (having an hash collision between different blobs like the OCI blobs or the ACI is very difficult).

The store will also support multiple hash algorithms (appc wants sha512, OCI image spec requires sha256 with optional sha384 and sha512 support)


### Different behaviors/downsides

#### Possible duplicated ACI fetches

Using appc name string discovery, the same image could be returned, depending on how meta tags are defined, using different image strings:

`example.com/app01:v1.0.0`
`example.com/app01:v1.0.0,anotherlabel=value`

With the current store implementation the image will be fetched one time, imported in the store and when fetching the other image string, store.GetACI will find an image matching the required name and labels in their manifest and won’t fetch it again.

Since with appc the image hash isn’t know via discovery but only calculated after the image has been fetched, using the ref approach, the image will be fetched for every different image string. This can be mitigated by using, based on the transport, transport caching information (like http Etag and cache-control max-age).

With OCI this won’t happen since the image manifest digest and its layers/config digests are known ahead of time.

#### The image manifest can contain different image name and labels than the one showed by the app image name string reference if using --insecure-options=image. Since the manifest is not used this is ignored.

#### Long ref names

While in docker the ref name is usually short since it’s just of one type ([REGISTRY_HOST[:REGISTRY_PORT]/]NAME[:TAG] abbreviated to NAME[:TAG]) here the ref names should be longer (big file paths, http url).
A solution will be to show them ellipsized by default with an option to show the full ref.


## References format

Every reference will be based on the distribution URL scheme (https://github.com/coreos/rkt/pull/2953 ) and will be saved in the refs store using the store reproducible/comparable string value.

For simplicty, the below examples shows the references as user friendly strings.

## Examples

### First time image fetch

`rkt fetch example.com/app01:v1.0.0`

`rkt image list`



| Ref (or Distribution?)   | digest/hash                 |
|--------------------------|-----------------------------|
| example.com/app01:v1.0.0 | sha512-a1a4aa408a5862237... |


Now we can run the image

Via hash:

`rkt run sha512-a1a4aa408a5862237`

Via ref:

`rkt run example.com/app01:v1.0.0`



### Latest pattern

NOTE: with appc/rkt fetching example.com/app01 or example.com/app01:latest is different. With the former, appc discovery will add, only for the discovery phase, a version=latest label. The latter, instead, means asking for an image with version=latest in it’s manifest since it’ll be checked (if not providing --insecure-options=image) and fail it the image manifest has a version != latest.

`rkt fetch example.com/app01`

`rkt image list`

| Ref (or Distribution?) | digest/hash                 |
|------------------------|-----------------------------|
| example.com/app01      | sha512-a1a4aa408a5862237... |


Now we fetch  example.com/app01:v1.0.0 that is the same image as example.com/app01 (no version defined so latest pattern)

`rkt fetch example.com/app01:v1.0.0`

`rkt image list`

| Ref (or Distribution?)   | digest/hash                 |
|--------------------------|-----------------------------|
| example.com/app01        | sha512-a1a4aa408a5862237... |
| example.com/app01:v1.0.0 | sha512-a1a4aa408a5862237... |

We have two references to the same image digest

Now suppose that example.com/app01:v1.0.1 has been released and the latest pattern will fetch it:

`rkt fetch example.com/app01`

`rkt image list`

| Ref (or Distribution?)   | digest/hash                 |
|--------------------------|-----------------------------|
| example.com/app01        | sha512-a651db8c1e151c489... |
| example.com/app01:v1.0.0 | sha512-a1a4aa408a5862237... |



If, also if bad, the example.com/app01:v1.0.0 discovered image as been replaced, running again
rkt fetch example.com/app01:v1.0.0 will fetch an image with a different digest

`rkt image list`

| Ref (or Distribution?)   | digest/hash                  |
|--------------------------|------------------------------|
| example.com/app01        | sha512-a651db8c1e151c489...  |
| example.com/app01:v1.0.0 | sha512-f437d9384ff8931121... |


The old sha512-a1a4aa408a5862237 is not referenced but still exists in the store.
The idea is to not show them by default abd add a (-a, --all) flag to show them:

`rkt image list -a`


| Ref (or Distribution?)   | digest/hash                  |
|--------------------------|------------------------------|
| example.com/app01:latest | sha512-a651db8c1e151c489...  |
| example.com/app01:v1.0.0 | sha512-f437d9384ff8931121... |
| <none>                   | sha512-a1a4aa408a5862237     |


It can be executed anyway but only using its image digest.

Fetch the example.com/app01:v1.0.0 from file:


`rkt fetch ./app01.aci`

`rkt image list`


| Ref (or Distribution?)      | digest/hash                  |
|-----------------------------|------------------------------|
| example.com/app01:latest    | sha512-a651db8c1e151c489...  |
| example.com/app01:v1.0.0    | sha512-f437d9384ff8931121... |
| /absolute/path/to/app01.aci | sha512-f437d9384ff8931121... |

We end with two different reference types (appc string and file path) to the same image sha512-f437d9384ff8931121


### ACI dependencies example

TODO


## Read Only stores
Using the cas + ref approach, a readonly store could be created using a file system layout but it’ll be difficult to manage (and probably slow): See https://github.com/coreos/rkt/issues/695#issuecomment-236515325 . So the proposed solution is to use the store just a rkt internal cache and use different distribution types to fetch an image from an external on disk source.


## Transport caching

See https://github.com/coreos/rkt/pull/2964

## Additional Changes to the store implementation

* Remove the use of sha512 half size digest. This is a big source of confusion:
 * Store doesn’t contains the full digest information
 * For this reason the code uses the “key” term instead of digest and this propagates all around the code

## Different behaviors

The current changes have some different behavior on the current rkt UX:

* Fetching an image via an aci-archive distribution won't give the ability to subsequently access it using the app discovery string. This is IMHO a good thing (see the above inconsitent behaviors description). As an addition:
 * Considering Name based distribution, will be possible to add a set-ref command to let the user set additional references. The settable references should be only for Name based distributions since it doesn't makes great sense adding a referece for a location based distribution. In this way an user can fetch location based references and set a name based reference on the image (this can be also considered like an image **import**). 
* Now fetching a file will behave like any other aci-archive distribution (http). Previously the files was always fetched also if already available in the store. This can be good if we assume that an user would like to always fetch the file (for example if it has been replaced with a new ACI) so there can be two possible solutions:
 * Add a special handling logic for aci-archive with file URL to skip getting the references from the store.
 * *preferred* Change the current image fetching logic (see below)
* `rkt image list` output will slightly change (removed name and latest fields, added ref fields with multiple entries for the same image having multiple refs)
* `rkt image rm` won’t remove the image by name (name doesn’t exists anymore) but it should do two different things:
 * Remove image by hash: will remove the image and all the associated refs.
 * Remove ref: will just remove the associated ref (not the image).

## Migration from the old store
* The new store will use other data locations and the old store data will be not used anymore (it should also be removed). Since the rkt store should be meant as a local cache the effect will be that all the images will be fetched again. IMHO this isn’t an issue (it’s not like the docker local graph store where users can put their built images).
 * Possible migration: It'll be possible to migrate ACI blobs from the old to the new store. Additionally, using the remote table information some refs (file, http, docker) could be converted to the related refs. What will not possible to get are the references for the `appc` distribution type since now the source appc image string is not saved in the store, so these will require the be refetched to set their reference.

## Rktnetes notes
NOTE: IMHO the current k8s image handling logic is too tied to the docker logic and needs to be improved to fit multiple image specs

The rkt apiservice will return for the `ListImage` api call the image information, where image is defined at the start of the document (OCI manifest digest and aci digests). Different thing should be defined on the api service side to handle different image types.

## Related changes

### Image fetching behavior changes
The current image fetching logic IMHO does not provide a perfect UX and this will noted when starting using this new reference based logic with OCI images. Also the aci-archive url fetching will improve when changing it. A future PR/proposal will be opened explaining how to change it.
