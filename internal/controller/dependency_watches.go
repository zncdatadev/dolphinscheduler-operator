/*
Copyright 2024 zncdatadev.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	authv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/authentication/v1alpha1"
	s3v1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/s3/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ReferenceWatches wakes clusters when a non-owned referenced object changes. Owns watches only
// follow controller references; these ConfigMaps, Secrets, S3Connections, and cluster-scoped
// AuthenticationClasses are deliberately user-managed and therefore need reverse mapping.
func ReferenceWatches(c ctrlclient.Client) []func(*builder.Builder) *builder.Builder {
	return []func(*builder.Builder) *builder.Builder{
		func(b *builder.Builder) *builder.Builder {
			return b.Watches(&corev1.ConfigMap{}, handler.EnqueueRequestsFromMapFunc(
				func(ctx context.Context, object ctrlclient.Object) []reconcile.Request {
					return mapConfigMap(ctx, c, object)
				}))
		},
		func(b *builder.Builder) *builder.Builder {
			return b.Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(
				func(ctx context.Context, object ctrlclient.Object) []reconcile.Request {
					return mapSecret(ctx, c, object)
				}))
		},
		func(b *builder.Builder) *builder.Builder {
			return b.Watches(&s3v1alpha1.S3Connection{}, handler.EnqueueRequestsFromMapFunc(
				func(ctx context.Context, object ctrlclient.Object) []reconcile.Request {
					return mapS3Connection(ctx, c, object)
				}))
		},
		func(b *builder.Builder) *builder.Builder {
			return b.Watches(&authv1alpha1.AuthenticationClass{}, handler.EnqueueRequestsFromMapFunc(
				func(ctx context.Context, object ctrlclient.Object) []reconcile.Request {
					return mapAuthenticationClass(ctx, c, object)
				}))
		},
	}
}

func mapConfigMap(ctx context.Context, c ctrlclient.Client, object ctrlclient.Object) []reconcile.Request {
	return mapReferencedObject(ctx, c, object, func(cr *dolphinv1alpha1.DolphinschedulerCluster) bool {
		if cr.Spec.ClusterConfig == nil {
			return false
		}
		return cr.Spec.ClusterConfig.ZookeeperConfigMapName == object.GetName() ||
			cr.Spec.ClusterConfig.VectorAggregatorConfigMapName == object.GetName()
	})
}

func mapSecret(ctx context.Context, c ctrlclient.Client, object ctrlclient.Object) []reconcile.Request {
	return mapReferencedObject(ctx, c, object, func(cr *dolphinv1alpha1.DolphinschedulerCluster) bool {
		if cr.Spec.ClusterConfig == nil {
			return false
		}
		if database := cr.Spec.ClusterConfig.Database; database != nil &&
			database.CredentialsSecret == object.GetName() {
			return true
		}
		for _, authentication := range cr.Spec.ClusterConfig.Authentication {
			if authentication.Oidc != nil && authentication.Oidc.Secret == object.GetName() {
				return true
			}
		}
		return false
	})
}

func mapS3Connection(ctx context.Context, c ctrlclient.Client, object ctrlclient.Object) []reconcile.Request {
	return mapReferencedObject(ctx, c, object, func(cr *dolphinv1alpha1.DolphinschedulerCluster) bool {
		if cr.Spec.ClusterConfig == nil || cr.Spec.ClusterConfig.S3 == nil ||
			cr.Spec.ClusterConfig.S3.Connection == nil {
			return false
		}
		return cr.Spec.ClusterConfig.S3.Connection.Reference == object.GetName()
	})
}

func mapReferencedObject(
	ctx context.Context,
	c ctrlclient.Client,
	object ctrlclient.Object,
	matches func(*dolphinv1alpha1.DolphinschedulerCluster) bool,
) []reconcile.Request {
	clusters := &dolphinv1alpha1.DolphinschedulerClusterList{}
	if err := c.List(ctx, clusters, ctrlclient.InNamespace(object.GetNamespace())); err != nil {
		log.FromContext(ctx).Error(err, "failed to map referenced object to DolphinScheduler clusters",
			"kind", object.GetObjectKind().GroupVersionKind().Kind,
			"namespace", object.GetNamespace(), "name", object.GetName())
		return nil
	}

	requests := make([]reconcile.Request, 0, len(clusters.Items))
	for i := range clusters.Items {
		cluster := &clusters.Items[i]
		if matches(cluster) {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: cluster.Namespace,
				Name:      cluster.Name,
			}})
		}
	}
	return requests
}

func mapAuthenticationClass(
	ctx context.Context,
	c ctrlclient.Client,
	object ctrlclient.Object,
) []reconcile.Request {
	clusters := &dolphinv1alpha1.DolphinschedulerClusterList{}
	if err := c.List(ctx, clusters); err != nil {
		log.FromContext(ctx).Error(err, "failed to map AuthenticationClass to DolphinScheduler clusters",
			"name", object.GetName())
		return nil
	}

	requests := make([]reconcile.Request, 0, len(clusters.Items))
	for i := range clusters.Items {
		cluster := &clusters.Items[i]
		if cluster.Spec.ClusterConfig == nil {
			continue
		}
		for _, authentication := range cluster.Spec.ClusterConfig.Authentication {
			if authentication.AuthenticationClass == object.GetName() {
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
					Namespace: cluster.Namespace,
					Name:      cluster.Name,
				}})
				break
			}
		}
	}
	return requests
}
