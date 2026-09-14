package controller

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	s3v1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/s3/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	runtimeDependencyFingerprintAnnotation = LabelDomain + "/runtime-dependency-hash"
	runtimeDependencyKindConfigMap         = "ConfigMap"
	runtimeDependencyKindSecret            = "Secret"
	runtimeDependencyKindS3Connection      = "S3Connection"
)

type runtimeDependencyReference struct {
	kind string
	name string
}

// runtimeDependencyFingerprint returns a stable digest of both the role group's rendered
// configuration and the resourceVersions of external objects consumed by its Pods. Referenced
// values (and in particular Secret data) are deliberately never included. A missing object or
// resourceVersion stops reconciliation: Kubernetes API objects always have a resourceVersion,
// and starting a Pod with an unresolved dependency would only turn the same configuration error
// into a delayed startup failure.
func runtimeDependencyFingerprint(
	ctx context.Context,
	c ctrlclient.Client,
	cr *dolphinv1alpha1.DolphinschedulerCluster,
	roleName string,
	roleGroupConfig map[string]string,
) (string, error) {
	if cr.Spec.ClusterConfig == nil {
		return "", fmt.Errorf("spec.clusterConfig is required")
	}

	references := map[string]runtimeDependencyReference{}
	add := func(kind, name string) error {
		name = strings.TrimSpace(name)
		if name == "" {
			return fmt.Errorf("runtime %s dependency name is required", kind)
		}
		references[kind+"/"+name] = runtimeDependencyReference{kind: kind, name: name}
		return nil
	}

	if cr.Spec.ClusterConfig.Database != nil {
		if err := add(runtimeDependencyKindSecret, databaseCredentialsSecretName(cr)); err != nil {
			return "", err
		}
	}
	if err := add(runtimeDependencyKindConfigMap, envsConfigMapName(cr.Name)); err != nil {
		return "", err
	}
	if err := add(runtimeDependencyKindConfigMap, cr.Spec.ClusterConfig.ZookeeperConfigMapName); err != nil {
		return "", err
	}
	if bucket := cr.Spec.ClusterConfig.S3; bucket != nil && bucket.Connection != nil &&
		strings.TrimSpace(bucket.Connection.Reference) != "" {
		if err := add(runtimeDependencyKindS3Connection, bucket.Connection.Reference); err != nil {
			return "", err
		}
	}
	if roleName == dolphinv1alpha1.RoleApi {
		for _, authentication := range cr.Spec.ClusterConfig.Authentication {
			if authentication.Oidc != nil {
				if err := add(runtimeDependencyKindSecret, authentication.Oidc.Secret); err != nil {
					return "", fmt.Errorf("OIDC credential %w", err)
				}
			}
		}
	}

	versions := make([]string, 0, len(references))
	for _, key := range slices.Sorted(maps.Keys(references)) {
		reference := references[key]
		var object ctrlclient.Object
		switch reference.kind {
		case runtimeDependencyKindConfigMap:
			object = &corev1.ConfigMap{}
		case runtimeDependencyKindSecret:
			object = &corev1.Secret{}
		case runtimeDependencyKindS3Connection:
			object = &s3v1alpha1.S3Connection{}
		default:
			return "", fmt.Errorf("unsupported runtime dependency kind %q", reference.kind)
		}

		objectKey := types.NamespacedName{Namespace: cr.Namespace, Name: reference.name}
		if err := c.Get(ctx, objectKey, object); err != nil {
			return "", fmt.Errorf("failed to resolve runtime %s dependency %s: %w", reference.kind, objectKey, err)
		}
		resourceVersion := object.GetResourceVersion()
		if resourceVersion == "" {
			return "", fmt.Errorf("runtime %s dependency %s has no resourceVersion", reference.kind, objectKey)
		}
		versions = append(versions, reference.kind+"/"+objectKey.String()+"="+resourceVersion)
	}

	payload, err := json.Marshal(struct {
		Config   map[string]string `json:"config"`
		Versions []string          `json:"versions"`
	}{Config: roleGroupConfig, Versions: versions})
	if err != nil {
		return "", fmt.Errorf("failed to encode runtime dependency fingerprint: %w", err)
	}
	digest := sha256.Sum256(payload)
	return fmt.Sprintf("%x", digest), nil
}
