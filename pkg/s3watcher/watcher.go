package s3watcher

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/luisdavim/configmapper/pkg/config"
	"github.com/luisdavim/configmapper/pkg/utils"
	"github.com/rs/zerolog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	konfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	DefaultInterval = time.Minute
)

type worker struct {
	log        zerolog.Logger
	client     *s3.Client
	k8s        client.Client
	stop       chan struct{}
	bucketName string
	files      config.ResourceMap
}

type S3Watcher struct {
	config  config.S3Map
	log     zerolog.Logger
	workers map[string]worker
	k8s     client.Client
	sync.RWMutex
}

func getConfigKey(cfg config.S3Mapping) string {
	return filepath.Join(cfg.S3Endpoint, cfg.BucketName)
}

func newS3Client(ctx context.Context, endpoint string) *s3.Client {
	// LoadDefaultConfig automatically reads AWS_ACCESS_KEY_ID,
	// AWS_SECRET_ACCESS_KEY, and AWS_REGION from the environment.
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)

			// Custom S3 providers almost always need PathStyle.
			// o.UsePathStyle = true
		}
	})

	return client
}

func New(cfg config.S3Map) (*S3Watcher, error) {
	kfg, err := konfig.GetConfig()
	if err != nil {
		return nil, err
	}
	c, err := client.New(kfg, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	w := &S3Watcher{
		config: cfg,
		log:    zerolog.New(os.Stderr).With().Timestamp().Str("name", "filewatcher").Logger().Level(zerolog.InfoLevel),
		k8s:    c,
	}

	curNS, _ := utils.GetInClusterNamespace()
	defaultEndpoint := os.Getenv("S3_ENDPOINT")
	for file, c := range cfg {
		if c.Name == "" {
			return nil, fmt.Errorf("no resource name for %s", file)
		}
		if c.Namespace == "" {
			c.Namespace = curNS
			w.config[file] = c
		}
		if c.S3Endpoint == "" {
			c.S3Endpoint = defaultEndpoint
			w.config[file] = c
		}

		if c.Interval.Duration == 0 {
			c.Interval.Duration = DefaultInterval
		}
	}

	return w, nil
}

func (w *S3Watcher) Start(ctx context.Context) {
	w.Lock()
	defer w.Unlock()

	if w.workers == nil {
		w.workers = make(map[string]worker)
	}

	for f, c := range w.config {
		key := getConfigKey(c)
		wrk, ok := w.workers[key]
		if !ok {
			wrk = worker{
				bucketName: c.BucketName,
				client:     newS3Client(ctx, c.S3Endpoint),
				k8s:        w.k8s,
				stop:       make(chan struct{}),
			}
		}
		if wrk.files == nil {
			wrk.files = make(config.ResourceMap)
		}
		wrk.files[f] = c.ResourceMapping
		w.workers[key] = wrk
		wrk.schedule(ctx, c.Interval.Duration)
	}
}

func (w *S3Watcher) Stop() {
	w.Lock()
	defer w.Unlock()
	for _, c := range w.config {
		key := getConfigKey(c)
		if wkr, ok := w.workers[key]; ok && wkr.stop != nil {
			close(wkr.stop)
		}
		delete(w.workers, key)
	}
}

func (w *worker) schedule(ctx context.Context, interval time.Duration) {
	go func() {
		err := w.download(ctx)
		w.log.Err(err).Str("bucket", w.bucketName).Msg("downloading")
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				err := w.download(ctx)
				w.log.Err(err).Str("bucket", w.bucketName).Msg("downloading")
			case <-ctx.Done():
				w.log.Info().Str("bucket", w.bucketName).Msg("context canceled, stopping")
				return
			case <-w.stop:
				w.log.Info().Str("bucket", w.bucketName).Msg("got quit signal, stopping")
				return
			}
		}
	}()
}

func (w *worker) download(ctx context.Context) error {
	for file, cfg := range w.files {
		output, err := w.client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
			Bucket: aws.String(w.bucketName),
			Prefix: &file,
		})
		w.log.Err(err).Str("bucket", w.bucketName).Str("path", file).Msg("listing objects")
		if err != nil {
			continue
		}

		data := map[string]string{}

		for _, obj := range output.Contents {
			res, err := w.client.GetObject(ctx, &s3.GetObjectInput{
				Bucket: aws.String(w.bucketName),
				Key:    obj.Key,
			})
			w.log.Err(err).Str("bucket", w.bucketName).Str("path", *obj.Key).Msg("getting object")
			if err != nil {
				continue
			}
			defer res.Body.Close()
			buf := new(strings.Builder)
			_, err = io.Copy(buf, res.Body)
			w.log.Err(err).Str("bucket", w.bucketName).Str("path", *obj.Key).Msg("reading object")
			if err != nil {
				continue
			}
			data[filepath.Base(*obj.Key)] = buf.String()
			buf.Reset()
		}

		op, err := utils.CreateOrUpdate(ctx, cfg.Name, cfg.Namespace, cfg.ResourceType, data, w.k8s)
		w.log.Err(err).Str("bucket", w.bucketName).Str("path", file).Str("operation", string(op)).Msgf("%s: %s/%s", cfg.ResourceType, cfg.Namespace, cfg.Name)
		if err != nil {
			continue
		}
	}

	// TODO: collect and aggregate errors
	return nil
}
