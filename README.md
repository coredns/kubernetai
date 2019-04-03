# kubernetai

## Name

*Kubernetai* - serve multiple Kubernetes within a Server.

## Description

*Kubernetai* (koo-ber-NET-eye) is the plural form of Kubernetes.
In a nutshell, *Kubernetai* is an external plugin for CoreDNS that holds multiple
[*kubernetes*](https://github.com/coredns/coredns/tree/master/plugin/kubernetes) plugin
configurations.  It allows one CoreDNS server to connect to more than one Kubernetes server at a time.

With *kubernetai*, you can define multiple *kubernetes* blocks in your Corefile. All options are
exactly the same as the built in *kubernetes* plugin, you just name them `kubernetai` instead of
`kubernetes`.

Note that in Kubernetes, ClusterIP Services cannot be routed outside the cluster.  Therefore to accomodate cross-cluster access, you'll need to use Service Endpoint IPs.  For that reason, for *kubernetai* to serve IPs that are routable to clients outside the cluster, you should use Headless Services instead of ClusterIP Services.  Of course, for this to work, you will also need to ensure that Service Endpoint IPs are routable across the clusters, which is possible to do, but not necessarily the case by default.

## Syntax

The options for *kubernetai* are identical to the *kubernetes* plugin.  Please see the documentation for the [*kubernetes* plugin](https://github.com/coredns/coredns/blob/master/plugin/kubernetes/README.md), for syntax and option definitions.

## External Plugin

*Kubernetai* is an *external* plugin, which means it is not included in CoreDNS releases.  To use *kubernetai*, you'll need to build a CoreDNS image with *kubernetai* (replacing *kubernetes*). In a nutshell you'll need to:
* Clone https://github.com/coredns/coredns
* Add this plugin to [plugin.cfg](https://github.com/coredns/coredns/blob/master/plugin.cfg) per instructions therein.
* `make -f Makefile.release DOCKER=your-docker-repo release`
* `make -f Makefile.release DOCKER=your-docker-repo docker`
* `make -f Makefile.release DOCKER=your-docker-repo docker-push`

## Examples

For example, the following Corefile will connect to three different Kubernetes clusters.

~~~ txt
. {
    errors
    log
    kubernetai cluster.local {
      endpoint http://192.168.99.100
    }
    kubernetai assemblage.local {
      endpoint http://192.168.99.101
    }
    kubernetai conglomeration.local {
      endpoint http://192.168.99.102
    }
}
~~~

### Fallthrough

*Fallthrough* in *kubernetai* will fall-through to the next *kubernetai* stanza (in the order they are in the Corefile),
or to the next plugin (if it's the last *kubernetai* stanza).  This can be used to provide a kind of cross-cluster fault tolerance when you have common services deployed across multiple clusters.

Here is an example Corefile that makes a connection to the local cluster, but also a remote cluster that uses the same domain name. Because both *kubernetai* stanzas serve the same zone, queries for `cluster.local`
will always first get processed by the first stanza. The *fallthrough* in the first stanza allows processing to go to the next stanza if the service is not found in the first.

The `ignore empty_service` tells *kubernetai* not to create records for ClusterIP Services that have no ready endpoints.  In this example, if a ClusterIP Service in the local cluster has no ready endpoints, clients will be directed to the remote Service. This essentially allows the Services in the local cluster to be defined as ClusterIP services, however the Services in the remote cluster must still be headless.

~~~ txt
. {
    errors
    log
    kubernetai cluster.local {
      ignore empty_service
      fallthrough
    }
    kubernetai cluster.local {
      endpoint https://remote-k8s-cluster
    }
}
~~~
