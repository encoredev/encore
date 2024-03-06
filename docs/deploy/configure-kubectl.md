---
seotitle: Configure kubectl to access your Encore Kubernetes cluster
seodesc: Learn how to configure kubectl to access your Encore Kubernetes cluster.
title: Configure kubectl
---

Encore automatically provisions and manages Kubernetes clusters for you, but sometimes it's useful to manually inspect
clusters using the [kubectl](https://kubernetes.io/docs/reference/kubectl/) cli. To do this, you need to configure `kubectl` to connect and authenticate through
encore. You can do this by running the following command in your app directory:

```shell
encore kubernetes configure -e <environment>
```

Where `<environment>` is the name of the environment you want to configure `kubectl` for.

This will configure `kubectl` to use `encore` to authenticate the cluster and proxy your traffic to the correct 
cluster. You can now use `kubectl` as you normally would, for example:

```shell
kubectl get pods
```
