package config

type Config struct {
	FileMap FileMap `mapstructure:"fileMap.omitempty"`
	Watcher Watcher `mapstructure:"watcher,omitempty"`
}

type FileMap map[string]FileMapping

type FileMapping struct {
	ResourceType string `mapstructure:"type,omitempty"`
	Namespace    string `mapstructure:"namespace,omitempty"`
	Name         string `mapstructure:"name,omitempty"`
}

type Watcher struct {
	ConfigMaps    bool   `mapstructure:"configMaps,omitempty"`
	Secrets       bool   `mapstructure:"secrets,omitempty"`
	Namespaces    string `mapstructure:"namespaces,omitempty"`
	RequiredLabel string `mapstructure:"labelSelector,omitempty"`
	DefaultPath   string `mapstructure:"defaultPath,omitempty"`
}
