package secret

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

	"github.com/luisdavim/configmapper/pkg/k8swatcher/consts"
	"github.com/luisdavim/configmapper/pkg/k8swatcher/filter"
)

// Reconciler reconciles a Secret object
type Reconciler struct {
	RequiredLabel string
	DefaultPath   string
	client.Client
	Scheme *runtime.Scheme
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, ps []predicate.Predicate) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		WithEventFilter(predicates(ps)).
		Complete(r)
}

// predicates will filter events for secrets that haven't changed
// or are annotated to be skipped
func predicates(ps []predicate.Predicate) predicate.Predicate {
	ps = append(ps, predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{}), filter.SkipAnnotation(consts.SkipAnnotation))

	return predicate.And(ps...)
}

//+kubebuilder:rbac:groups=networking.k8s.io,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=secrets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=networking.k8s.io,resources=secrets/finalizers,verbs=update

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.Log.WithName("controller").WithValues("secret", req.NamespacedName)

	secret := &corev1.Secret{}
	if err := r.Get(ctx, req.NamespacedName, secret); err != nil {
		log.Error(err, "unable to fetch Secret")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !secret.DeletionTimestamp.IsZero() {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(secret, consts.FinalizerName) {
			if err := r.cleanup(secret); err != nil {
				log.Error(err, "failed to cleanup checks for secret")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if r.needsCleanUp(secret) {
		// the skip annotation was added or changed from false to true
		// or the required label was removed or set to false
		err := r.cleanup(secret)
		return ctrl.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(secret, consts.FinalizerName) {
		controllerutil.AddFinalizer(secret, consts.FinalizerName)
		if err := r.Update(ctx, secret); err != nil {
			log.Error(err, "failed to add finalizer")
			return ctrl.Result{}, err
		}
		// no need to exit here the predicates will filter the finalizer update event
		// return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	baseDir := r.DefaultPath
	if path, ok := secret.Annotations[consts.TargetDirAnnotation]; ok {
		baseDir = path
	}
	for file, data := range secret.Data {
		log.WithValues("file", file, "path", baseDir).Info("writting file")
		if err := os.WriteFile(filepath.Join(baseDir, file), data, 0o644); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{RequeueAfter: time.Hour}, nil
}

func (r *Reconciler) needsCleanUp(secret *corev1.Secret) bool {
	if v, ok := secret.Annotations[consts.SkipAnnotation]; ok {
		// skip annotation was added or changed from false to true
		if skip, _ := strconv.ParseBool(v); skip {
			return true
		}
	}

	// required label was removed or set to false
	v, ok := secret.Labels[r.RequiredLabel]
	if !ok {
		return true
	}
	if inc, _ := strconv.ParseBool(v); !inc {
		return true
	}
	return false
}

func (r *Reconciler) cleanup(secret *corev1.Secret) error {
	baseDir := r.DefaultPath
	if path, ok := secret.Annotations[consts.TargetDirAnnotation]; ok {
		baseDir = path
	}
	skip, _ := strconv.ParseBool(secret.Annotations[consts.IgnoreDeleteAnnotation])
	if !skip {
		for file := range secret.Data {
			os.Remove(filepath.Join(baseDir, file))
		}
	}

	// we won't be tracking this resource anymore
	controllerutil.RemoveFinalizer(secret, consts.FinalizerName)
	if err := r.Update(context.Background(), secret); err != nil {
		return fmt.Errorf("failed to remove finalizer: %w", err)
	}

	return nil
}
