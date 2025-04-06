package common

import (
	"strconv"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Reconciler struct {
	RequiredLabel string
	DefaultPath   string
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
