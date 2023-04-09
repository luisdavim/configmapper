package downloader

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/rs/zerolog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	konfig "sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/luisdavim/configmapper/pkg/config"
	"github.com/luisdavim/configmapper/pkg/utils"
)

type Downloader struct {
	config config.URLMap
	log    zerolog.Logger
	stop   map[string](chan struct{})
	client *retryablehttp.Client
	k8s    client.Client
	sync.RWMutex
}

func New(cfg config.URLMap) (*Downloader, error) {
	kfg, err := konfig.GetConfig()
	if err != nil {
		return nil, err
	}
	c, err := client.New(kfg, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	d := &Downloader{
		config: cfg,
		client: retryablehttp.NewClient(),
		k8s:    c,
		stop:   make(map[string](chan struct{})),
		log:    zerolog.New(os.Stderr).With().Timestamp().Str("name", "downloader").Logger().Level(zerolog.InfoLevel),
	}

	curNS, _ := utils.GetInClusterNamespace()
	for u, c := range cfg {
		if _, err := url.Parse(u); err != nil {
			d.log.Err(err).Msg("invalid URL")
			continue
		}
		if c.Name == "" {
			d.log.Error().Msgf("no resource name for %s", u)
			continue
		}
		if c.Key == "" {
			c.Key = "config"
		}
		if c.Namespace == "" {
			c.Namespace = curNS
			d.config[u] = c
		}
	}

	return d, nil
}

func (d *Downloader) Start(ctx context.Context) {
	for url := range d.config {
		if _, ok := d.stop[url]; ok {
			// already running
			continue
		}
		d.stop[url] = make(chan struct{})
		d.schedule(ctx, url)
	}
}

func (r *Downloader) Stop() {
	r.Lock()
	defer r.Unlock()
	for name := range r.config {
		if stopCh, ok := r.stop[name]; ok && stopCh != nil {
			close(stopCh)
		}
		delete(r.stop, name)
	}
}

func (d *Downloader) schedule(ctx context.Context, url string) {
	d.log.Info().Str("name", url).Msg("starting config")
	go func() {
		d.download(url)
		ticker := time.NewTicker(d.config[url].Interval.Duration)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				d.download(url)
			case <-ctx.Done():
				d.log.Info().Str("name", url).Msg("stopping config")
				return
			case <-d.stop[url]:
				d.log.Info().Str("name", url).Msg("got quit signal, stopping config")
				return
			}
		}
	}()
}

func (w *Downloader) download(path string) error {
	cfg, ok := w.config[path]
	if !ok {
		return fmt.Errorf("config for %s not found", path)
	}

	body, err := w.get(path)
	if err != nil {
		return err
	}

	data := map[string]string{
		cfg.Key: body,
	}

	op, err := utils.CreateOrUpdate(cfg.Name, cfg.Namespace, cfg.ResourceType, data, w.k8s)
	w.log.Err(err).Str("operation", string(op)).Msgf("%s: %s/%s", cfg.ResourceType, cfg.Namespace, cfg.Name)
	return err
}

func (d *Downloader) get(url string) (string, error) {
	res, err := d.client.Get(url)
	if err != nil {
		return "", err
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	res.Body.Close()

	return string(body), nil
}
