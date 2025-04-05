package common

import "sigs.k8s.io/controller-runtime/pkg/client"

func GetBaseDir(o client.Object) string {
	annotations := o.GetAnnotations()
	if annotations == nil {
		return ""
	}

	return annotations[TargetDirAnnotation]
}
