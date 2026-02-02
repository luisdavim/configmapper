package utils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	ps "github.com/shirou/gopsutil/v4/process"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var ErrNotInK8s = errors.New("not running in-cluster, please specify the namespace")

const inClusterNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

// GetInClusterNamespace returns the pod's namespace
func GetInClusterNamespace() (string, error) {
	if namespace := os.Getenv("POD_NAMESPACE"); namespace != "" {
		return namespace, nil
	}

	// Check whether the namespace file exists.
	// If not, we are not running in cluster so can't guess the namespace.
	if _, err := os.Stat(inClusterNamespacePath); err != nil {
		if os.IsNotExist(err) {
			return "default", ErrNotInK8s
		}
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

func ReadersEqual(r1, r2 io.Reader, chunkSize int) (bool, error) {
	if chunkSize == 0 {
		chunkSize = 4 * 1024
	}

	b1 := make([]byte, chunkSize)
	b2 := make([]byte, chunkSize)

	for {
		n1, err1 := io.ReadFull(r1, b1)
		n2, err2 := io.ReadFull(r2, b2)

		// https://pkg.go.dev/io#Reader
		// > Callers should always process the n > 0 bytes returned
		// > before considering the error err. Doing so correctly
		// > handles I/O errors that happen after reading some bytes
		// > and also both of the allowed EOF behaviors.

		if n1 != n2 {
			return false, nil
		}

		if !bytes.Equal(b1[:n1], b2[:n2]) {
			return false, nil
		}

		// reached the end of both readers
		if (err1 == io.EOF && err2 == io.EOF) || (err1 == io.ErrUnexpectedEOF && err2 == io.ErrUnexpectedEOF) {
			return true, nil
		}

		// reached the end of one of the readers
		if (err1 != err2) && (err1 == nil || err2 == nil) && (err1 == io.EOF || err2 == io.EOF || err1 == io.ErrUnexpectedEOF || err2 == io.ErrUnexpectedEOF) {
			return false, nil
		}

		if err1 != nil || err2 != nil {
			return false, errors.Join(err1, err2)
		}
	}
}
