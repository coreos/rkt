
## Fetching logic with distribution

If and when the distribution concept and implementation in #2953 will land, the next step will be a fetchers refactor to fully cover that.

These is the proposed idea:

## Distribution handlers
* Every distribution will have one or more dedicated handlers:

### Distribution goals:

* The handlers will receive an input distribution string and additional arguments (like the current ability to provide/override a signature with a local file). They will talk with the image store.
* From the input string they'll implement the distribution specific logic to fetch an image (this means calling one or multiple different transport fetching) and import it in the image store.


Examples:


**archive distribution**

base archive distribution handler (may choose a sub handler based on the archive format) -> transport handler to fetch the ArchiveURL (from file://, http:// etc...)

**docker distribution**

docker handler ->  The docker handler may choose to use docker2aci and import an ACI in the store or natively call transport handlers to fetch v2/oci blobs with a transport and import them in the store.

**appc distribution**

appc handler -> The appc handler will call appc discovery and fetch the discovered URLs. An interesting thing is that this fetcher should internally convert the discovered ACI URL to a child distribution of type Archive without rewriting all the fetching logic


# Distributions handler vs transport handlers
The above logic will also creates a better separation between the distribution and the transport layers.

For example there may exist multiple transport plugins (file, http, s3, bittorrent etc...) to be called by an archive distribution.



## Distribution Handlers plugins

With this approach the distributions handler will live in the stage0. If there's the need to make them plugins the main tricky points is that they need to talk with an image store, so it'll be difficult to code them in a different language than go.

## Transport handler and transport plugins

Instead transport logic should be easily coded in a plugin if a good plugin interface will be defined.

### Implementation

The idea (also to handle transport caching and other things) is to implement the transport plugin logic in this way:

* Transport handler: will live in the stage0 and be the entrypoint for the distribution handlers. It'll be just one and transport independent. Its purpose is the prepare the input data to send to a transport plugin, call it and handle plugin outputs.
* Transport plugin: will read the input data provided by the transport handler, use it if needed, return the output data.

### Transport level caching

For future store work I'd like to split out all the information not related to an image.

Now the remote table in the current store serves two purposes:
 * A sort of reference store (url -> digest)
 * transport caching data (now only http etag and Cache Control max-age)

The first one will be covered with the ref store using the distribution concept in #2953 

For the latter I'd like to create a caching transport store facility that transports implementations should use the fetch/save caching data.
Transport caching can be used to fetch objects that given the same transport address may change (like ACIs). To avoid fetching them every time we can use transport provided caching information.
OCI blobs, being referenced by their digest can be considered immutable and so no caching information will be required.
 
The idea (that I'm going to try also in the image store) will be to not use a relational db like ql but a k/v store like boltdb (https://github.com/boltdb/bolt)  (I'll open a new issue to talk about pro/cons of this solution)

For all the transport types the key will be the transport url (http://... file:// etc...) it will be associated with a blob digest and a json documents that will contain the caching information.

**NOTE**
Since the json document can contain arbitrary data, is there the need to standardize it for every transport type so it can be used as input and provided as output by all the transport plugin for that specific transport type?
In this way multiple independent plugin implementations for the same transport will use the same data, if this isn't standardized and every transport plugin uses it's own kind of data since (it will be saved in the transport caching store) if a transport plugin is swapped with another one using diffrent data, this data won't be recognised by that transport plugin causing possible problems.

Example data format: 

**Http caching**
```
{
  digest: …,
  fetchTime: ….,
  ETag: …,
  maxage: ….
}
```
The fetching plugin can check the expiration of an http transport with 2 different logics:

* before fetching: if the provided `fetchTime + maxage < now` don't fetch 
* during real fetching: send the ETag to the server and verify if the http return code is a 304


**File caching**
```
{
  digest: ...
  size: ...
  mtime: ….
}
```

The fetching plugin can check that size and mtime of the file to fetch are different than the saved one

** Docker2aci caching **

Docker2aci can be considered a "special" transport since it will return an aci starting from a docker "URL"

Caching information for docker2aci may contain the current imageID or manifest digest to avoid reconverting a docker image if its not changed (this is useful until native docker v2/oci will be available but will be always needed for v1 registries)

#### Transport level cache garbage collection
Since caching information is decoupled from store information, if a blob is removed from the image store there isn't a function call to the transport caching store to remove them. So a periodic task (also needed for other layer) should be called to clean the caching store from stale data.

## Transport logic

Given all of the above this is the proposed workflow:

* Distribution handler: calls a transport handler (it can be called many times depending on the distribution logic) providing a transport url
 * Transport handler: If needed will fetch the caching data with the key == transport url, check if the provided blob digest exists in the store (since the above decoupling between image store and transport cache store).
 * Transport handler: Call the transport plugin providing the input information (and if needed passing also the transport caching data json document)
  * Transport plugin:  Will use the input data to do the fetching. It's up to the transport plugin to use the caching document. The transport plugin will return if the file as been fetched or if the stage0 should use the cached data.
* Transport plugin: will handle output data, for example write updated caching data in the transport caching store and return the result to the Distribution handler
* Distribution handle: continue with its logic.


