# ConfigMapper

ConfigMapper is meant to be used as a sidecar in a Kubernetes `Pod` to map local files to `ConfigMaps` (or `Secrets`).
It can watch files in the local filesystem and when they change, create or update a `ConfigMap` (or `Secret`).
It can also watch `ConfigMaps` (or `Secrets`) with a specific label selector and create or update files in the local filesystem.

## Features

- Create or update ConfigMaps or Secrets from local files
  - Watch the local filesystem to keep ConfigMaps and Secrets up-to-date
- Create or update ConfigMaps or Secrets from URLs
  - Poll URLs and store the response in a ConfigMap or Secret
- Extract files from ConfigMaps and Secrets
- Update or delete local files when the ConfigMap or Secret changes
  - Watch ConfigMaps and Secrets to keep local files up-to-date
- Send a signal to a process when a local file is modified
  - Watch the local filesystem and reload processes
- Filter based on labels

Note that, for the processes reloading functionality, you'll need to set [`shareProcessNamespace: true` on your Pod](https://kubernetes.io/docs/tasks/configure-pod-container/share-process-namespace/) to allow sending signals across containers.

## Configuration

The tool can be configured using a `yaml` file nameed `configmapper.yaml`:

```yaml
# fileMap maps file paths to k8s ConfigMaps, Secrets or processes
fileMap:
  "/tmp/config.yaml":
    type: ConfigMap
    name: my-cm
    namespace: foo
    processName: myExec
    signal: "SIGHUP"
  "/tmp/users.yaml":
    processName: myExec
    signal: "SIGHUP"
  "/tmp/secrets.yaml":
    type: Secret
    name: my-secret
    namespace: foo

# urlMap maps urls to k8s ConfigMaps or Secrets
urlMap:
  "https://fs.example.com/config":
    type: ConfigMap
    name: my-other-cm
    key: config.json
    namespace: foo
    interval: 5m # how frequently to download, defaults to 60s
  "https://fs.example.com/secret":
    type: Secret
    name: my-other-secret
    key: secret.json
    namespace: foo

# watcher can watch ConfigMap and Secrets to create files in the Pod's FS
watcher:
  configMaps: true
  secrets: true
  labelSelector: "app=foo"
  namespaces: foo
  defaultPath: "/tmp"
```

The default path is the local filesystem path where files will be created from the observed `ConfigMaps` and `Secrets`, this can be overridden from each `ConfigMap` (or `Secret`) through an annotation, you can also use annotations to tell the tool to ignore specific resources or to ignore deletes, to kepp the generated file after the resource was deleted:

```yaml
metadata:
  annotations:
    configmapper/target-directory: "/path/to/target/directory"
    configmapper/skip: "false"
    configmapper/ignore-delete: "false"
```

The watcher config can also be set, using environment variables, for example, `WATCHER_NAMESPACES` can be used to set the list of namespaces to watch.
Environment variables are automatically mapped to the comandline flags and named after the config file paths.

## Usage

```console
$ configmapper -h
Watch files, ConfigMaps and Secrets.

Usage:
  configmapper [flags]

Flags:
  -c, --config string           config file (default is $HOME/.configmapper.yaml)
  -p, --default-path string     Default path where to write the files (default "/tmp")
  -h, --help                    help for configmapper
  -l, --label-selector string   Label selector for configMaps and secrets
  -n, --namespaces string       Comma separated list of namespaces to watch (defaults to the Pod's namespace)
      --watch-configmaps        Whether to watch ConfigMaps
      --watch-secrets           Whether to watch secrets
```

## Caveats

### Share Process Namespace

Too use the process reloading functionality, the Pod needs to be configured to [Share Process Namespace](https://kubernetes.io/docs/tasks/configure-pod-container/share-process-namespace/) with `shareProcessNamespace: true`.

### Update delay

When mapping a file that is mounted from ConfigMap or Secret, the changes won't take effect immediately.

This is because the projected values of ConfigMaps and Secrets are not updated exactly when the underlying object changes, but instead they're [updated periodically](https://kubernetes.io/docs/tasks/configure-pod-container/configure-pod-configmap/#mounted-configmaps-are-updated-automatically) according to the `syncFrequency` argument to the [kubelet config](https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/). This defaults to 1 minute.

### Files mounted via `subPath` are never updated

This is a long-standing Kubernetes issue: ConfigMaps and Secrets mounted as files with a `subPath` key do not get updated by the kubelet. See [issue #50345](https://github.com/kubernetes/kubernetes/issues/50345) on Github.

A possible workaround involves [mounting the Secret/ConfigMap without using subPath in a different folder and manually creating a symlink from an initContainer ahead of time to that folder](https://github.com/kubernetes/kubernetes/issues/50345#issuecomment-400647420), or if possible at all switching to not using `subPath`.
