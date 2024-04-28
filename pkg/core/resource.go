package core

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var resourceLog = ctrl.Log.WithName("resourceFetcher")

type ResourceClient struct {
	Ctx       context.Context
	Client    client.Client
	Namespace string
}

// NewResourceClient new resource client
func NewResourceClient(ctx context.Context, client client.Client, namespace string) *ResourceClient {
	if namespace == "" {
		namespace = metav1.NamespaceDefault
	}
	return &ResourceClient{
		Ctx:       ctx,
		Client:    client,
		Namespace: namespace,
	}
}

func (r *ResourceClient) Get(obj client.Object) error {
	name := obj.GetName()
	kind := obj.GetObjectKind()
	if err := r.Client.Get(r.Ctx, client.ObjectKey{Namespace: r.Namespace, Name: name}, obj); err != nil {
		opt := []any{"ns", r.Namespace, "name", name, "kind", kind}
		if apierrors.IsNotFound(err) {
			resourceLog.Error(err, "Fetch resource NotFound", opt...)
		} else {
			resourceLog.Error(err, "Fetch resource occur some unknown err", opt...)
		}
		return err
	}
	return nil
}

func (r *ResourceClient) List(obj client.ObjectList) error {
	return r.Client.List(r.Ctx, obj, client.InNamespace(r.Namespace))
}

type InstanceAttributes interface {
	ClusterConfigGetter
	GetClusterConfig() any

	GetNamespace() string

	GetInstanceName() string
}
