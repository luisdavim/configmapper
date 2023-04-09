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
- Filter based on labels

## Configuration

The tool can be configured using a `yaml` file nameed `configmapper.yaml`:

```yaml
# fileMap maps file paths to k8s ConfigMaps or Secrets
fileMap:
  "/tmp/config.yaml":
    type: ConfigMap
    name: my-cm
    namespace: foo
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
