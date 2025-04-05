package downloader

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/rs/zerolog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	konfig "sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/luisdavim/configmapper/pkg/config"
	"github.com/luisdavim/configmapper/pkg/utils"
)

const (
	DefaultKey      = "config"
	DefaultInterval = time.Minute
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
		if c.Name == "" {
			return nil, fmt.Errorf("no resource name for %s", u)
		}
		if c.Namespace == "" {
			c.Namespace = curNS
			d.config[u] = c
		}
		pu, err := url.Parse(u)
		if err != nil {
			return nil, err
		}
		if c.Key == "" {
			c.Key = filepath.Base(pu.Path)
			if c.Key == "." || c.Key == "/" {
				c.Key = DefaultKey
			}
		}
		if c.Interval.Duration == 0 {
			c.Interval.Duration = DefaultInterval
		}
	}

	return d, nil
}

func (d *Downloader) Start(ctx context.Context) {
	d.Lock()
	defer d.Unlock()
	for url := range d.config {
		if _, ok := d.stop[url]; ok {
			// already running
			continue
		}
		d.stop[url] = make(chan struct{})
		d.schedule(ctx, url)
	}
}

func (d *Downloader) Stop() {
	d.Lock()
	defer d.Unlock()
	for url := range d.config {
		if stopCh, ok := d.stop[url]; ok && stopCh != nil {
			close(stopCh)
		}
		delete(d.stop, url)
	}
}

func (d *Downloader) schedule(ctx context.Context, url string) {
	d.log.Info().Str("url", url).Msg("starting")
	go func() {
		err := d.download(url)
		d.log.Err(err).Str("url", url).Msgf("downloading")
		ticker := time.NewTicker(d.config[url].Interval.Duration)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				err := d.download(url)
				d.log.Err(err).Str("url", url).Msgf("downloading")
			case <-ctx.Done():
				d.log.Info().Str("url", url).Msg("context canceled, stopping")
				return
			case <-d.stop[url]:
				d.log.Info().Str("url", url).Msg("got quit signal, stopping")
				return
			}
		}
	}()
}

func (d *Downloader) download(url string) error {
	d.RLock()
	cfg, ok := d.config[url]
	d.RUnlock()
	if !ok {
		return fmt.Errorf("config for %s not found", url)
	}

	body, err := d.get(url)
	if err != nil {
		return err
	}

	data := map[string]string{
		cfg.Key: body,
	}

	op, err := utils.CreateOrUpdate(cfg.Name, cfg.Namespace, cfg.ResourceType, data, d.k8s)
	d.log.Err(err).Str("operation", string(op)).Msgf("%s: %s/%s", cfg.ResourceType, cfg.Namespace, cfg.Name)
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
	_ = res.Body.Close()

	return string(body), nil
}
