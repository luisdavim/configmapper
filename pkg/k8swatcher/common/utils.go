package common

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/luisdavim/configmapper/pkg/k8swatcher/filter"
)

func GetBaseDir(o client.Object) string {
	annotations := o.GetAnnotations()
	if annotations == nil {
		return ""
	}

	return annotations[TargetDirAnnotation]
}

// Predicates will filter events for objects that haven't changed
// or are annotated to be skipped
func Predicates(ps []predicate.Predicate) predicate.Predicate {
	ps = append(ps, filter.Default(), filter.SkipAnnotation(SkipAnnotation))

	return predicate.And(ps...)
}
