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

## Syntax

The options for *kubernetai* are identical to the *kubernetes* plugin.  Please see the documentation for the [*kubernetes* plugin](https://github.com/coredns/coredns/blob/master/plugin/kubernetes/README.md), for syntax and option definitions.

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

### fallthrough

*Fallthrough* in *kubernetai* will fall-through to the next *kubernetai* stanza (in the order they are in the Corefile),
or to the next plugin (if it's the last *kubernetai* stanza).

Here is an example Corefile that makes two connections to a single minikube instance.
Because both servers are actually the same server, they both have the same zone, which means that queries for `cluster.local`
will always first get processed by the first stanza. The *fallthrough* in the first stanza allows processing to go to the next stanza if the service is not found.

~~~ txt
. {
    errors
    log
    kubernetai cluster.local {
      endpoint http://192.168.99.100:8080
      namespaces default
      fallthrough
    }
    kubernetai cluster.local {
      endpoint http://192.168.99.100:8080
    }
}
~~~

The first *kubernetai* stanza exposes only the `default` namespace.
When we query for a service in the `default` namespace, the kubernetes instance in the first stanza answers.
When we query for a service in any other namespace, the first stanza falls through to the second, and the second connection answers.
