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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	s3v1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/s3/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("referenced dependency watches", func() {
	requestFor := func(cr *dolphinv1alpha1.DolphinschedulerCluster) reconcile.Request {
		return reconcile.Request{NamespacedName: types.NamespacedName{
			Namespace: cr.Namespace,
			Name:      cr.Name,
		}}
	}

	It("maps a ZooKeeper discovery ConfigMap only within its namespace", func() {
		matching := newTestCluster()
		otherNamespace := newTestCluster()
		otherNamespace.Name = "other-namespace-cluster"
		otherNamespace.Namespace = "other"
		c := newFakeClient(matching, otherNamespace)
		configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name: matching.Spec.ClusterConfig.ZookeeperConfigMapName, Namespace: matching.Namespace,
		}}

		Expect(mapConfigMap(context.Background(), c, configMap)).To(ConsistOf(requestFor(matching)))
	})

	It("maps a Vector aggregator discovery ConfigMap", func() {
		matching := newTestCluster()
		matching.Spec.ClusterConfig.VectorAggregatorConfigMapName = "vector-aggregator"
		unrelated := newTestCluster()
		unrelated.Name = "unrelated-cluster"
		c := newFakeClient(matching, unrelated)
		configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name: matching.Spec.ClusterConfig.VectorAggregatorConfigMapName, Namespace: matching.Namespace,
		}}

		Expect(mapConfigMap(context.Background(), c, configMap)).To(ConsistOf(requestFor(matching)))
	})

	It("maps database and OIDC credential Secrets", func() {
		databaseCluster := newTestCluster()
		oidcCluster := newTestCluster()
		oidcCluster.Name = "oidc-cluster"
		oidcCluster.Spec.ClusterConfig.Database.CredentialsSecret = "another-secret"
		oidcCluster.Spec.ClusterConfig.Authentication = []dolphinv1alpha1.AuthenticationSpec{{
			AuthenticationClass: "oidc",
			Oidc:                &dolphinv1alpha1.OidcCredentialSecretSpec{Secret: testDatabaseCredentialsSecret},
		}}
		c := newFakeClient(databaseCluster, oidcCluster)
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Name: testDatabaseCredentialsSecret, Namespace: testNamespace,
		}}

		Expect(mapSecret(context.Background(), c, secret)).To(
			ConsistOf(requestFor(databaseCluster), requestFor(oidcCluster)))
	})

	It("maps a referenced S3Connection", func() {
		matching := newTestCluster()
		matching.Spec.ClusterConfig.S3 = &s3v1alpha1.S3BucketSpec{
			BucketName: testS3BucketName,
			Connection: &s3v1alpha1.S3BucketConnectionSpec{Reference: testS3ConnectionName},
		}
		c := newFakeClient(matching)
		connection := &s3v1alpha1.S3Connection{ObjectMeta: metav1.ObjectMeta{
			Name: testS3ConnectionName, Namespace: testNamespace,
		}}

		Expect(mapS3Connection(context.Background(), c, connection)).To(ConsistOf(requestFor(matching)))
	})

	It("maps a cluster-scoped AuthenticationClass across namespaces", func() {
		first := newTestCluster()
		first.Spec.ClusterConfig.Authentication = []dolphinv1alpha1.AuthenticationSpec{{
			AuthenticationClass: "company-ldap",
		}}
		second := first.DeepCopy()
		second.Name = "second-cluster"
		second.Namespace = "other"
		c := newFakeClient(first, second)
		authenticationClass := &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{
			Name: "company-ldap",
		}}

		Expect(mapAuthenticationClass(context.Background(), c, authenticationClass)).To(
			ConsistOf(requestFor(first), requestFor(second)))
	})
})
