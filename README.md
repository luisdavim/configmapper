# ConfigMapper

ConfigMapper is meant to be used as a sidecar and map local files to `ConfigMaps` (or `Secrets`).
It can watch files in the local filesystem and when they change update a `ConfigMap` (or `Secret`).
It can also watch `ConfigMaps` (or `Secrets`) with a specific label selector and create or update files in the local filesystem.

The tool can be configured using a `yaml` file nameed `configmapper.yaml`:

```yaml
# fileMap maps file paths to k8s ConfigMaps or Secrets
fileMap:
  "/tmp/config.yaml":
    type: ConfigMap
    namespace: foo
  "/tmp/secrets.yaml":
    type: Secret
    namespace: foo

# watcher can watch ConfigMap and Secrets to create files in the Pod's FS
watcher:
  configMaps: true
  secrets: true
  labelSelector: "app=foo"
  namespaces: foo
  defaultPath: "/tmp"
```

The watcher config can also be set, using environment variables, for example, `WATCHER_NAMESPACES` can be used to set the list of namespaces to watch.
Environment variables are automatically mapped to the comandline flags and named after the config file paths.

Usage:

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
