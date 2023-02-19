package filewatcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	konfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/luisdavim/configmapper/pkg/config"
	"github.com/luisdavim/configmapper/pkg/utils"
)

type Watcher struct {
	config config.FileMap
	fw     *fsnotify.Watcher
	log    zerolog.Logger
	k8s    client.Client
}

func New(cfg config.FileMap) (*Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	kfg, err := konfig.GetConfig()
	if err != nil {
		return nil, err
	}
	c, err := client.New(kfg, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	w := &Watcher{
		config: cfg,
		fw:     watcher,
		log:    zerolog.New(os.Stderr).With().Timestamp().Str("name", "filewatcher").Logger().Level(zerolog.InfoLevel),
		k8s:    c,
	}

	curNS, _ := utils.GetInClusterNamespace()
	for file, c := range cfg {
		if c.Name == "" {
			w.log.Error().Msgf("no resource name for %s", file)
			continue
		}
		if c.Namespace == "" {
			c.Namespace = curNS
			w.config[file] = c
		}
		err := watcher.Add(file)
		if err != nil {
			return nil, err
		}
	}
	return w, nil
}

func (w *Watcher) Start(ctx context.Context) error {
	defer w.fw.Close()
	if len(w.config) == 0 {
		return nil
	}
	for {
		select {
		case event := <-w.fw.Events:
			// k8s configmaps use symlinks, we need this workaround.
			// original configmap file is removed
			if event.Has(fsnotify.Remove) {
				// remove the watch since the file is removed
				if err := w.fw.Remove(event.Name); err != nil {
					w.log.Err(err).Msg("removing file watch")
				}
				// add a new watcher pointing to the new symlink/file
				if err := w.fw.Add(event.Name); err != nil {
					w.log.Err(err).Msg("updating file watch")
				}
				err := w.updateConfig(event.Name)
				w.log.Err(err).Msg("updating config")
				continue
			}
			// also allow normal files to be modified and reloaded.
			if event.Has(fsnotify.Write) {
				err := w.updateConfig(event.Name)
				w.log.Err(err).Msg("updating config")
				continue
			}
		case err := <-w.fw.Errors:
			w.log.Err(err).Msg("file watch")
		case <-ctx.Done():
			return nil
		}
	}
}

func (w *Watcher) updateConfig(name string) error {
	cfg, ok := w.config[name]
	if !ok {
		var found bool
		for n := range w.config {
			if strings.HasPrefix(name, n) {
				cfg = w.config[n]
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("config for %s not found", name)
		}
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	if strings.EqualFold(cfg.ResourceType, "secret") {
		obj := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: cfg.Name, Namespace: cfg.Namespace}}
		op, err := controllerutil.CreateOrUpdate(context.TODO(), w.k8s, obj, func() error {
			if obj.Data == nil {
				obj.Data = make(map[string][]byte)
			}
			obj.Data[filepath.Base(name)] = data
			return nil
		})

		w.log.Err(err).Str("operation", string(op)).Msgf("Secret: %s/%s", cfg.Namespace, cfg.Name)
		return err
	}

	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: cfg.Name, Namespace: cfg.Namespace}}
	op, err := controllerutil.CreateOrUpdate(context.TODO(), w.k8s, obj, func() error {
		if obj.Data == nil {
			obj.Data = make(map[string]string)
		}
		obj.Data[filepath.Base(name)] = string(data)
		return nil
	})
	w.log.Err(err).Str("operation", string(op)).Msgf("ConfigMap: %s/%s", cfg.Namespace, cfg.Name)

	return err
}
