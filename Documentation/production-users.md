# Production users

This document tracks people and use cases for rkt in production. [Join the community](https://github.com/rkt/rkt/), and help us keep the list up-to-date.

## [BlaBlaCar](https://www.blablacar.com/)

BlaBlaCar is a trusted car-pooling service based in France. They've [blogged about](http://blablatech.com/blog/why-and-how-blablacar-went-full-containers) the stability of rkt being a big appeal.

* Media report from [LeMagIT](http://www.lemagit.fr/actualites/4500272608/avec-rkt-10-CoreOS-estime-que-sa-technologie-de-conteneur-est-mure-pour-la-production): "rkt has become the container technology of choice for the French carpool specialist Blablacar. The company, which has adopted early container technologies, now relies on rkt and CoreOS for 90% of its applications."
* Media report from [Silicon.fr](http://www.silicon.fr/blablacar-generalise-conteneurs-rocket-159564.html), "Rocket tackled the limitations we had identified with Docker. In particular, the solution eliminated the use of a daemon (process running in the background, Editor's note) and approached the network part in a very modular way"

## [Kubernetes and CoreOS Container Linux]()

rkt is used by Container Linux to execute the Kubernetes on node agent called the "kubelet". This enables users to be able to have cluster controlled versioning of this critical component. There are many documented production users of Kubernetes and Container Linux including all of the users of [CoreOS Tectonic](https://coreos.com/tectonic).

## [Kumul.us](https://kumul.us/)

Kumulus Technologies offers classes and services to optimize your Cloud. They've [blogged about](https://kumul.us/docker-youve-failed-me-again-rkt-please-save-me/) rkt's pod native features and Kubernetes support being a reason for their adoption.

## [Kinvolk.io](https://kinvolk.io/)

Kinvolk are a professional consulting team and active contributors to systemd, rkt, and the Linux kernel. rkt helps them easily [test various Linux kernels and configurations](https://kinvolk.io/blog/2017/02/using-custom-rkt-stage1-images-to-test-against-various-kernel-versions/).

## [Adfin.com](http://www.adfin.com/)

Per Milosz Tanski, [from HN comments](https://news.ycombinator.com/item?id=12366932). "The same experiences we switched to using rkt, supervised by upstart (and now systemd). We have an "application" state template in our salt config and every docker update something would cause all of them to fail. Thankful the "application" state template abstracted running container enough were we switched from docker -> rkt under the covers without anybody noticing, except now we no longer fearing of container software updates."

## Individual Use Cases

These are blog posts and stories from people evaluating rkt. Although most aren't (yet) production use cases, the links offer useful information and shared experience in deploying rkt.

- [Adrian de Jonge](https://medium.com/@adriaandejonge/moving-from-docker-to-rkt-310dc9aec938#.earel7ndf) moving from docker to rkt
- [Kushal Das, Cloud Engineer at RedHat](https://kushaldas.in/posts/using-rkt-and-systemd.html) using rkt and systemd
- [Julia Evans, Infrastructure Engineer at Stripe](https://jvns.ca/blog/2016/11/03/what-happens-when-you-run-a-rkt-container/) what happens when you run a rkt container
- [Nicholas Dziuba](https://www.reddit.com/r/coreos/comments/5vhrlw/thesis_with_rkt_evaluation/) writing his thesis on the security model of rkt
- [John Pettigrew, Developer at Clevyr ](https://pettigrew.rocks/2016/05/30/a-beginners-guide-to-rkt-containers/) a beginner’s guide to rkt containers
- [Joey Zwicker, at Co-founder of Pachyderm.io](https://news.ycombinator.com/item?id=12103201) rkt support is on the roadmap. See [Does Pachyderm only work with docker containers?](http://docs.pachyderm.io/en/stable/FAQ.html)
- [Fernando Ike, Sales Engineer at Highwinds](http://www.slideshare.net/fernandoike/docker-baleias-vs-rkt-foguetes) docker vs rkt comparison
- [Scott Lowe, Developer at VMWare](http://www.slideshare.net/lowescott/getting-started-with-containers) getting started with containers
- [Yutaka Matsubara, CTO at Abby](http://www.slideshare.net/YutakaMatsubara/rocket-46800960) rkt
- [Thuc Le Dong, Developer at Silicon Straits](http://www.slideshare.net/ledongthuc/rkt-container-engine) overview of the rkt container engine
- [Avi Deitcher, Business Technology Consultant](http://blog.atomicinc.com/2016/10/14/can-rktkubernetes-provide-a-real-alternative-to-docker-2896/)
