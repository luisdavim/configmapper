package config

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type Config struct {
	URLMap  URLMap  `mapstructure:"urlMap,omitempty"`
	FileMap FileMap `mapstructure:"fileMap,omitempty"`
	Watcher Watcher `mapstructure:"watcher,omitempty"`
}

type SignalMap map[string]SignalMapping

type SignalMapping struct {
	ProcessName string `mapstructure:"processName,omitempty"`
	Signal      string `mapstructure:"signal,omitempty"`
}

type FileMap map[string]FileMapping

// FileMapping allows mappting filepaths to k8s resources, OS signals and/or URLs
type FileMapping struct {
	// ResourceMapping can map a file to a Kubernetes Secret or ConfigMap
	// when the file changes the Kubernetes resource is updated with the file contentes
	ResourceMapping `mapstructure:",squash"`
	// SignalMapping can map a file to a process, when the file changes the process is sent the specied signal
	SignalMapping `mapstructure:",squash"`
	// URL where to post the data from the watched file
	URL string `mapstructure:"url,omitempty"`
}

type ResourceMapping struct {
	ResourceType string `mapstructure:"type,omitempty"`
	Namespace    string `mapstructure:"namespace,omitempty"`
	Name         string `mapstructure:"name,omitempty"`
	Key          string `mapstructure:"key,omitempty"`
}

type URLMap map[string]URLMapping

// URLMapping allows mapping URLs to k8s resources
type URLMapping struct {
	// ResourceMapping can map a URL to a Kubernetes Secret or ConfigMap
	// The Kubernetes resource is updated with the data fetched from the URL
	ResourceMapping `mapstructure:",squash"`
	// Interval defines how often to poll the URL
	Interval metav1.Duration `mapstructure:"interval"`
}

type Watcher struct {
	ConfigMaps    bool            `mapstructure:"configMaps,omitempty"`
	Secrets       bool            `mapstructure:"secrets,omitempty"`
	Namespaces    string          `mapstructure:"namespaces,omitempty"`
	RequiredLabel string          `mapstructure:"requiredLabel,omitempty"`
	LabelSelector string          `mapstructure:"labelSelector,omitempty"`
	DefaultPath   string          `mapstructure:"defaultPath,omitempty"`
	Interval      metav1.Duration `mapstructure:"interval,omitempty"`
	SignalMapping `mapstructure:",squash"`
}
