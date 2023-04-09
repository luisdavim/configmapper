package config

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type Config struct {
	URLMap  URLMap  `mapstructure:"urlMap,omitempty"`
	FileMap FileMap `mapstructure:"fileMap,omitempty"`
	Watcher Watcher `mapstructure:"watcher,omitempty"`
}

type FileMap map[string]FileMapping

type FileMapping struct {
	ResourceType string `mapstructure:"type,omitempty"`
	Namespace    string `mapstructure:"namespace,omitempty"`
	Name         string `mapstructure:"name,omitempty"`
}

type URLMap map[string]URLMapping

type URLMapping struct {
	FileMapping `mapstructure:",squash"`
	Interval    metav1.Duration `mapstructure:"interval"`
	Key         string
}

type Watcher struct {
	ConfigMaps    bool   `mapstructure:"configMaps,omitempty"`
	Secrets       bool   `mapstructure:"secrets,omitempty"`
	Namespaces    string `mapstructure:"namespaces,omitempty"`
	RequiredLabel string `mapstructure:"requiredLabel,omitempty"`
	LabelSelector string `mapstructure:"labelSelector,omitempty"`
	DefaultPath   string `mapstructure:"defaultPath,omitempty"`
}
