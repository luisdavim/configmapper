package utils

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"

	ps "github.com/shirou/gopsutil/v4/process"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const inClusterNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

// GetInClusterNamespace returns the pod's namespace
func GetInClusterNamespace() (string, error) {
	if namespace := os.Getenv("POD_NAMESPACE"); namespace != "" {
		return namespace, nil
	}

	// Check whether the namespace file exists.
	// If not, we are not running in cluster so can't guess the namespace.
	if _, err := os.Stat(inClusterNamespacePath); os.IsNotExist(err) {
		return "default", fmt.Errorf("not running in-cluster, please specify the namespace")
	} else if err != nil {
		return "", fmt.Errorf("error checking namespace file: %w", err)
	}

	// Load the namespace file and return its content
	namespace, err := os.ReadFile(inClusterNamespacePath)
	if err != nil {
		return "", fmt.Errorf("error reading namespace file: %w", err)
	}
	return strings.TrimSpace(string(namespace)), nil
}

func CreateOrUpdate(ctx context.Context, name, namespace, kind string, data map[string]string, c client.Client) (ctrlutil.OperationResult, error) {
	if strings.EqualFold(kind, "secret") {
		obj := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
		op, err := ctrlutil.CreateOrUpdate(ctx, c, obj, func() error {
			obj.StringData = data
			return nil
		})

		return op, err
	}

	if !strings.EqualFold(kind, "configmap") {
		return ctrlutil.OperationResultNone, fmt.Errorf("kind must be Secret or ConfigMap")
	}

	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	op, err := ctrlutil.CreateOrUpdate(ctx, c, obj, func() error {
		obj.Data = data
		return nil
	})
	return op, err
}

func findProcess(ctx context.Context, process string) (*ps.Process, error) {
	processes, err := ps.ProcessesWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list processes: %v", err)
	}

	for _, p := range processes {
		pname, err := p.Name()
		if err != nil {
			return nil, err
		}
		if pname == process {
			return p, nil
		}
	}

	return nil, fmt.Errorf("no process matching %s found", process)
}

func SignalProcess(ctx context.Context, process string, signal syscall.Signal) error {
	p, err := findProcess(ctx, process)
	if err != nil {
		return err
	}

	return p.SendSignalWithContext(ctx, signal)
}
