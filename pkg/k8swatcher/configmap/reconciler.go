package configmap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/luisdavim/configmapper/pkg/k8swatcher/common"
	"github.com/luisdavim/configmapper/pkg/k8swatcher/filter"
)

// Reconciler reconciles a ConfigMap object
type Reconciler struct {
	RequiredLabel string
	DefaultPath   string
	client.Client
	Scheme *runtime.Scheme
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, ps []predicate.Predicate) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		WithEventFilter(predicates(ps)).
		Complete(r)
}

// predicates will filter events for configMaps that haven't changed
// or are annotated to be skipped
func predicates(ps []predicate.Predicate) predicate.Predicate {
	ps = append(ps, filter.Default(), filter.SkipAnnotation(common.SkipAnnotation))

	return predicate.And(ps...)
}

//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps/finalizers,verbs=update

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.Log.WithName("configMapController").WithValues("configMap", req.NamespacedName)

	configMap := &corev1.ConfigMap{}
	if err := r.Get(ctx, req.NamespacedName, configMap); err != nil {
		log.Error(err, "unable to fetch ConfigMap")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !configMap.DeletionTimestamp.IsZero() {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(configMap, common.FinalizerName) {
			if err := r.cleanup(configMap); err != nil {
				log.Error(err, "failed to cleanup")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if r.needsCleanUp(configMap) {
		// the skip annotation was added or changed from false to true
		// or the required label was removed or set to false
		err := r.cleanup(configMap)
		return ctrl.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(configMap, common.FinalizerName) {
		controllerutil.AddFinalizer(configMap, common.FinalizerName)
		if err := r.Update(ctx, configMap); err != nil {
			log.Error(err, "failed to add finalizer")
			return ctrl.Result{}, err
		}
		// no need to exit here the predicates will filter the finalizer update event
		// return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	baseDir := r.DefaultPath
	if path := common.GetBaseDir(configMap); path != "" {
		baseDir = path
	}
	for file, data := range configMap.Data {
		log.WithValues("file", file, "path", baseDir).Info("writting file")
		if err := os.WriteFile(filepath.Join(baseDir, file), []byte(data), 0o644); err != nil {
			return ctrl.Result{}, err
		}
	}
	for file, data := range configMap.BinaryData {
		log.WithValues("file", file, "path", baseDir).Info("writting file")
		if err := os.WriteFile(filepath.Join(baseDir, file), data, 0o644); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{RequeueAfter: time.Hour}, nil
}

func (r *Reconciler) needsCleanUp(configMap *corev1.ConfigMap) bool {
	if v, ok := configMap.Annotations[common.SkipAnnotation]; ok {
		// skip annotation was added or changed from false to true
		if skip, _ := strconv.ParseBool(v); skip {
			return true
		}
	}

	// required label was removed or set to false
	v, ok := configMap.Labels[r.RequiredLabel]
	if !ok {
		return true
	}
	if inc, _ := strconv.ParseBool(v); !inc {
		return true
	}
	return false
}

func (r *Reconciler) cleanup(configMap *corev1.ConfigMap) error {
	baseDir := r.DefaultPath
	if path, ok := configMap.Annotations[common.TargetDirAnnotation]; ok {
		baseDir = path
	}
	skip, _ := strconv.ParseBool(configMap.Annotations[common.IgnoreDeleteAnnotation])
	if !skip {
		for file := range configMap.Data {
			_ = os.Remove(filepath.Join(baseDir, file))
		}
	}

	// we won't be tracking this resource anymore
	controllerutil.RemoveFinalizer(configMap, common.FinalizerName)
	if err := r.Update(context.Background(), configMap); err != nil {
		return fmt.Errorf("failed to remove finalizer: %w", err)
	}

	return nil
}
