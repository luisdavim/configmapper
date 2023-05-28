// configMapwatcher is a kubernetes controller that watches ConfigMap and Secret resources
package k8swatcher

import (
	"context"
	"fmt"
	"strings"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/luisdavim/configmapper/pkg/config"
	"github.com/luisdavim/configmapper/pkg/k8swatcher/configmap"
	"github.com/luisdavim/configmapper/pkg/k8swatcher/filter"
	"github.com/luisdavim/configmapper/pkg/k8swatcher/secret"
	"github.com/luisdavim/configmapper/pkg/utils"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	//+kubebuilder:scaffold:scheme
}

func Start(ctx context.Context, cfg config.Watcher) error {
	if !cfg.ConfigMaps && !cfg.Secrets {
		// nothing to do here...
		return nil
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zap.Options{
		Development: false,
	})))

	ctrlOpts := ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     ":8080",
		Port:                   9443,
		HealthProbeBindAddress: ":8081",
		LeaderElection:         false,
		LeaderElectionID:       "configmapper",
	}

	// limit the tool to the local namespace by default
	if cfg.Namespaces == "" {
		cfg.Namespaces, _ = utils.GetInClusterNamespace()
	}

	nss := strings.Split(cfg.Namespaces, ",")

	if len(nss) >= 1 {
		ctrlOpts.Cache.Namespaces = nss
	}

	var filters []predicate.Predicate
	if cfg.Namespaces != "" && len(nss) > 0 {
		filters = append(filters, filter.ByNamespace(nss))
	}

	if cfg.RequiredLabel != "" {
		filters = append(filters, filter.ByLabel(cfg.RequiredLabel))
	}

	if cfg.LabelSelector != "" {
		selector, err := labels.Parse(cfg.LabelSelector)
		if err != nil {
			return fmt.Errorf("invalid label selector: %w", err)
		}
		filters = append(filters, predicate.NewPredicateFuncs(func(o client.Object) bool {
			return selector.Matches(labels.Set(o.GetLabels()))
		}))
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrlOpts)
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		return fmt.Errorf("unable to create manager: %w", err)
	}

	// watch configMaps
	if cfg.ConfigMaps {
		if err := (&configmap.Reconciler{
			RequiredLabel: cfg.RequiredLabel,
			DefaultPath:   cfg.DefaultPath,
			Client:        mgr.GetClient(),
			Scheme:        mgr.GetScheme(),
		}).SetupWithManager(mgr, filters); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "ConfigMaps")
			return fmt.Errorf("unable to create controller: %w", err)
		}
	}

	// watch secrets
	if cfg.Secrets {
		if err := (&secret.Reconciler{
			RequiredLabel: cfg.RequiredLabel,
			DefaultPath:   cfg.DefaultPath,
			Client:        mgr.GetClient(),
			Scheme:        mgr.GetScheme(),
		}).SetupWithManager(mgr, filters); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Secrets")
			return fmt.Errorf("unable to create controller: %w", err)
		}
	}

	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		return fmt.Errorf("unable to set up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		return fmt.Errorf("unable to set up ready check: %w", err)
	}

	setupLog.Info("starting controller manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		return fmt.Errorf("problem running manager: %w", err)
	}
	return nil
}
