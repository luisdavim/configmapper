package consts

const (
	AnnotationPrefix       = "configmapper"
	FinalizerName          = AnnotationPrefix + "/finalizer"
	SkipAnnotation         = AnnotationPrefix + "/skip"
	TargetDirAnnotation    = AnnotationPrefix + "/target-directory"
	IgnoreDeleteAnnotation = AnnotationPrefix + "/ignore-delete"
)
