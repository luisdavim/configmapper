package filewatcher

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/fsnotify/fsnotify"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/rs/zerolog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	konfig "sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/luisdavim/configmapper/pkg/config"
	"github.com/luisdavim/configmapper/pkg/utils"
)

type Watcher struct {
	config config.FileMap
	fw     *fsnotify.Watcher
	log    zerolog.Logger
	k8s    client.Client
	http   *retryablehttp.Client
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
		http:   retryablehttp.NewClient(),
		k8s:    c,
	}

	curNS, _ := utils.GetInClusterNamespace()
	for file, c := range cfg {
		if c.Name == "" && c.ProcessName == "" {
			w.log.Error().Msgf("no resource or process name for %s", file)
			continue
		}
		if c.Namespace == "" {
			c.Namespace = curNS
			w.config[file] = c
		}
		if c.ResourceType == "" {
			c.ResourceType = "configmap"
			w.config[file] = c
		}
		if c.Signal == 0 {
			c.Signal = syscall.SIGHUP
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
	defer func() { _ = w.fw.Close() }()
	if len(w.config) == 0 {
		return nil
	}
	for {
		select {
		case event := <-w.fw.Events:
			if event.Has(fsnotify.Chmod) {
				continue
			}
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
				err := w.do(ctx, event.Name)
				w.log.Err(err).Msg("updating config")
				continue
			}
			// also allow normal files to be modified and reloaded.
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				err := w.do(ctx, event.Name)
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

func getFilesFromPath(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		// is path is a file, return it
		return []string{path}, nil
	}

	var files []string
	err = filepath.WalkDir(path, func(p string, i fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if i.IsDir() {
			// don't recurse into sub-folders
			return filepath.SkipDir
		}
		files = append(files, p)
		return nil
	})
	if err != nil {
		return files, err
	}

	return files, nil
}

func getData(path string) (map[string]string, error) {
	data := make(map[string]string)
	files, err := getFilesFromPath(path)
	if err != nil {
		return data, err
	}

	for _, fileName := range files {
		b, err := os.ReadFile(fileName)
		if err != nil {
			return data, err
		}
		data[filepath.Base(fileName)] = string(b)
	}

	return data, nil
}

func (w *Watcher) do(ctx context.Context, path string) error {
	cfg, ok := w.config[path]
	if !ok {
		var found bool
		// the path may point to a file in a watched folder
		for n := range w.config {
			if strings.HasPrefix(path, n) {
				cfg = w.config[n]
				path = n
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("config for %s not found", path)
		}
	}

	if cfg.ProcessName != "" {
		sig := syscall.SIGHUP
		if cfg.Signal != 0 {
			sig = cfg.Signal
		}
		err := utils.SignalProcess(ctx, cfg.ProcessName, sig)
		w.log.Err(err).Str("operation", "reload").Msgf("%s: %s", cfg.ProcessName, cfg.Signal)
		if err != nil {
			return err
		}
	}

	if cfg.Name == "" && cfg.URL == "" {
		// nothing left to be done
		return nil
	}

	data, err := getData(path)
	if err != nil {
		return err
	}

	// post the file contents to the configured URL
	if cfg.URL != "" {
		for _, payload := range data {
			// TODO: set the bodyType from the file type?
			resp, err := w.http.Post(cfg.URL, "", payload)
			w.log.Err(err).Str("operation", "post").Msgf("%s: %s", cfg.URL, resp.Status)
			if err != nil {
				return err
			}
		}
	}

	if cfg.Name == "" {
		// nothing left to be done
		return nil
	}

	if cfg.Key != "" {
		fname := filepath.Base(path)
		if d, ok := data[fname]; ok && fname != cfg.Key {
			data[cfg.Key] = d
			delete(data, fname)
		}
	}

	// Create or update the k8s resource
	op, err := utils.CreateOrUpdate(ctx, cfg.Name, cfg.Namespace, cfg.ResourceType, data, w.k8s)
	w.log.Err(err).Str("operation", string(op)).Msgf("%s: %s/%s", cfg.ResourceType, cfg.Namespace, cfg.Name)
	return err
}
