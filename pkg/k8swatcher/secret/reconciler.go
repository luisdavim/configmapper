package secret

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/luisdavim/configmapper/pkg/k8swatcher/common"
)

// Reconciler reconciles a Secret object
type Reconciler struct {
	common.Reconciler
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, ps []predicate.Predicate) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		WithEventFilter(common.Predicates(ps)).
		Complete(r)
}

//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=secrets/finalizers,verbs=update

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.Log.WithName("secretController").WithValues("secret", req.NamespacedName)

	secret := &corev1.Secret{}
	if err := r.Get(ctx, req.NamespacedName, secret); err != nil {
		log.Error(err, "unable to fetch Secret")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	baseDir := r.DefaultPath
	if path := common.GetBaseDir(secret); path != "" {
		baseDir = path
	}

	if !secret.DeletionTimestamp.IsZero() {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(secret, common.FinalizerName) {
			if err := r.cleanup(secret, baseDir); err != nil {
				log.Error(err, "failed to cleanup")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if r.NeedsCleanUp(secret) {
		// the skip annotation was added or changed from false to true
		// or the required label was removed or set to false
		return ctrl.Result{}, r.cleanup(secret, baseDir)
	}

	if err := os.MkdirAll(baseDir, 0700); err != nil {
		return ctrl.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(secret, common.FinalizerName) {
		controllerutil.AddFinalizer(secret, common.FinalizerName)
		if err := r.Update(ctx, secret); err != nil {
			log.Error(err, "failed to add finalizer")
			return ctrl.Result{}, err
		}
		// no need to exit here the predicates will filter the finalizer update event
		// return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	for file, data := range secret.Data {
		log.WithValues("file", file, "path", baseDir).Info("writting file")
		if err := os.WriteFile(filepath.Join(baseDir, file), data, 0o644); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{RequeueAfter: time.Hour}, nil
}

func (r *Reconciler) cleanup(secret *corev1.Secret, baseDir string) error {
	var skip bool
	if secret.Annotations != nil {
		skip, _ = strconv.ParseBool(secret.Annotations[common.IgnoreDeleteAnnotation])
	}

	if !skip {
		for file := range secret.Data {
			_ = os.Remove(filepath.Join(baseDir, file))
		}
	}

	// we won't be tracking this resource anymore
	controllerutil.RemoveFinalizer(secret, common.FinalizerName)
	if err := r.Update(context.Background(), secret); err != nil {
		return fmt.Errorf("failed to remove finalizer: %w", err)
	}

	return nil
}
