package filter

import (
	"strconv"

	"golang.org/x/exp/slices"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// SkipAnnotation will filter events that are marked to be skipped by the given annotation.
func SkipAnnotation(annotation string) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			annotations := e.Object.GetAnnotations()
			skip, _ := strconv.ParseBool(annotations[annotation])
			return !skip
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			newAnnotations := e.ObjectNew.GetAnnotations()
			if skip, _ := strconv.ParseBool(newAnnotations[annotation]); !skip {
				return true
			}
			oldAnnotations := e.ObjectOld.GetAnnotations()
			if skip, _ := strconv.ParseBool(oldAnnotations[annotation]); !skip {
				// cleanup needed
				return true
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return true
		},
	}
}

// ByLabel wiil exclude objects that don't have the required label
func ByLabel(label string) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			labels := e.Object.GetLabels()
			inc, _ := strconv.ParseBool(labels[label])
			return inc
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			newLabels := e.ObjectNew.GetLabels()
			if inc, _ := strconv.ParseBool(newLabels[label]); inc {
				return true
			}
			oldLabels := e.ObjectOld.GetLabels()
			if inc, _ := strconv.ParseBool(oldLabels[label]); inc {
				// cleanup needed
				return true
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return true
		},
	}
}

// ByNamespace will filter any events from Namespaces not in the given list.
func ByNamespace(namespaces []string) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(o client.Object) bool {
		return slices.Contains(namespaces, o.GetNamespace())
	})
}
