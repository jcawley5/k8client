package k8

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Create takes the representation of a namespace and creates it.  Returns the server's representation of the namespace, and an error, if there is any.
func CreateNamespace(ctx context.Context, namespace *v1.Namespace, opts metav1.CreateOptions) (result *v1.Namespace, err error) {
	// result = &v1.Namespace{}
	// err = c.client.Post().
	// 	Resource("namespaces").
	// 	VersionedParams(&opts, scheme.ParameterCodec).
	// 	Body(namespace).
	// 	Do(ctx).
	// 	Into(result)
	// return
}
