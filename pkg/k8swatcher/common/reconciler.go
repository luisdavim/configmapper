package common

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/luisdavim/configmapper/pkg/utils"
)

type Reconciler struct {
	RequeueInterval time.Duration
	RequiredLabel   string
	DefaultPath     string
	ProcessName     string
	Signal          syscall.Signal
	client.Client
	Scheme *runtime.Scheme
}

func (r *Reconciler) NeedsCleanUp(obj client.Object) bool {
	annotations := obj.GetAnnotations()
	if annotations != nil {
		if v, ok := annotations[SkipAnnotation]; ok {
			// skip annotation was added or changed from false to true
			if skip, _ := strconv.ParseBool(v); skip {
				return true
			}
		}
	}

	if r.RequiredLabel == "" {
		return false
	}

	// required label was removed or set to false
	labels := obj.GetLabels()

	if labels == nil {
		return true
	}

	v, ok := labels[r.RequiredLabel]
	if !ok {
		return true
	}
	if inc, _ := strconv.ParseBool(v); !inc {
		return true
	}
	return false
}

func (r *Reconciler) HandleFileUpdate(ctx context.Context, file, baseDir string, data []byte, force bool) error {
	log := ctrl.LoggerFrom(ctx)

	fp := filepath.Join(baseDir, file)

	if !force {
		// avoid overwritting the file if the contents already match
		f, err := os.Open(fp)
		if err == nil {
			if equal, _ := utils.ReadersEqual(bytes.NewReader(data), f, 0); equal {
				return nil
			}
		}
		if f != nil {
			_ = f.Close()
		}
	}

	log.WithValues("file", file, "path", baseDir).Info("writting file")
	if err := os.WriteFile(fp, data, 0o644); err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) SignalProcess(ctx context.Context) error {
	log := ctrl.LoggerFrom(ctx)

	if r.ProcessName == "" {
		return nil
	}

	err := utils.SignalProcess(ctx, r.ProcessName, r.Signal)
	if err != nil {
		log.Error(err, "failed to signal process", "ProcessName", r.ProcessName, "Signal", r.Signal.String())
		return err
	}

	log.Info("signaled process", "ProcessName", r.ProcessName, "Signal", r.Signal.String())

	return nil
}
