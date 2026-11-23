package controller

import (
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdatadev/dolphinscheduler-operator/internal/security"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/util"
	authv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/authentication/v1alpha1"
	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	s3v1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/s3/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/config"
	opgoconstant "github.com/zncdatadev/operator-go/pkg/constant"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	opgos3 "github.com/zncdatadev/operator-go/pkg/s3"
	opgosecurity "github.com/zncdatadev/operator-go/pkg/security"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testClusterName               = "test-dolphinscheduler"
	testNamespace                 = "default"
	testDatabaseCredentialsSecret = "postgresql-credentials"
	testS3BucketName              = "workflows"
	testS3ConnectionName          = "object-store"
	testS3Host                    = "s3.example.com"
	testS3DefaultRegion           = "us-east-1"
	testS3AlternateRegion         = "eu-west-1"
)

// expectedShell pins the container shell invocation the start script runs under. Xtrace must
// stay disabled because the API role reads LDAP credentials in that script.
var expectedShell = []string{"/bin/bash", "-euo", "pipefail", "-c"}

func roleScript(command []string) string {
	GinkgoHelper()
	Expect(command).To(HaveLen(len(expectedShell) + 2))
	Expect(command[:len(expectedShell)]).To(Equal(expectedShell))
	Expect(command[len(command)-1]).To(Equal(roleCommandArgZero))
	return command[len(expectedShell)]
}

func newTestCluster() *dolphinv1alpha1.DolphinschedulerCluster {
	pullPolicy := corev1.PullIfNotPresent
	roleSpec := func() *dolphinv1alpha1.RoleSpec {
		return &dolphinv1alpha1.RoleSpec{
			RoleGroups: map[string]dolphinv1alpha1.RoleGroupSpec{
				defaultRoleGroupName: {Replicas: ptr.To[int32](1)},
			},
		}
	}
	return &dolphinv1alpha1.DolphinschedulerCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testClusterName,
			Namespace: testNamespace,
		},
		Spec: dolphinv1alpha1.DolphinschedulerClusterSpec{
			Image: &dolphinv1alpha1.ImageSpec{
				Repo:       dolphinv1alpha1.DefaultRepository,
				PullPolicy: &pullPolicy,
			},
			ClusterConfig: &dolphinv1alpha1.ClusterConfigSpec{
				ZookeeperConfigMapName: "test-znode",
				Database: &dolphinv1alpha1.DatabaseSpec{
					ConnectionString:  "jdbc:postgresql://postgresql:5432/dolphinscheduler",
					DatabaseType:      "postgresql",
					CredentialsSecret: testDatabaseCredentialsSecret,
				},
			},
			Master:  roleSpec(),
			Worker:  roleSpec(),
			Api:     roleSpec(),
			Alerter: roleSpec(),
		},
	}
}

// declareRole produces one role's declaration through the handler's RoleProvider seam, the way
// GenericReconciler does (once per pass, with the cr and a client in hand).
func declareRole(
	ctx context.Context,
	handler *DolphinSchedulerRoleGroupHandler,
	c ctrlclient.Client,
	cr *dolphinv1alpha1.DolphinschedulerCluster,
	roleName string,
) reconciler.RoleDeclaration {
	GinkgoHelper()
	seedRuntimeDependencies(ctx, c, cr)
	catalog, err := handler.DeclareRoles(ctx, c, cr)
	Expect(err).NotTo(HaveOccurred())
	decl, ok := catalog[roleName]
	Expect(ok).To(BeTrue(), "role %s missing from the catalog", roleName)
	return decl
}

func seedRuntimeDependencies(
	ctx context.Context,
	c ctrlclient.Client,
	cr *dolphinv1alpha1.DolphinschedulerCluster,
) {
	GinkgoHelper()
	objects := []ctrlclient.Object{
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name: envsConfigMapName(cr.Name), Namespace: cr.Namespace,
		}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name: cr.Spec.ClusterConfig.ZookeeperConfigMapName, Namespace: cr.Namespace,
		}},
	}
	if cr.Spec.ClusterConfig.Database != nil {
		objects = append(objects, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: databaseCredentialsSecretName(cr), Namespace: cr.Namespace},
			Data: map[string][]byte{
				util.ConnectionStringAuthUser: []byte("dolphinscheduler"),
				util.ConnectionStringAuthPass: []byte("database-password"),
			},
		})
	}
	for _, object := range objects {
		err := c.Create(ctx, object)
		Expect(err == nil || apierrors.IsAlreadyExists(err)).To(BeTrue(), "seed %T %s: %v", object, object.GetName(), err)
	}
}

type resourceVersionClearingClient struct {
	ctrlclient.Client
	kind string
	name string
}

func (c resourceVersionClearingClient) Get(
	ctx context.Context,
	key ctrlclient.ObjectKey,
	object ctrlclient.Object,
	options ...ctrlclient.GetOption,
) error {
	if err := c.Client.Get(ctx, key, object, options...); err != nil {
		return err
	}
	kind := ""
	switch object.(type) {
	case *corev1.ConfigMap:
		kind = runtimeDependencyKindConfigMap
	case *corev1.Secret:
		kind = runtimeDependencyKindSecret
	case *s3v1alpha1.S3Connection:
		kind = runtimeDependencyKindS3Connection
	}
	if kind == c.kind && key.Name == c.name {
		object.SetResourceVersion("")
	}
	return nil
}

// newBuildContext assembles a RoleGroupBuildContext the way GenericReconciler does: the
// declared config defaults folded beneath the CR's role and role group levels, the derived
// contribution merged beneath the role/group overrides, and the image resolved once.
func newBuildContext(
	cr *dolphinv1alpha1.DolphinschedulerCluster,
	roleName string,
	decl reconciler.RoleDeclaration,
) *reconciler.RoleGroupBuildContext {
	GinkgoHelper()
	groupName := defaultRoleGroupName
	spec := cr.GetSpec()
	roleSpec := spec.Roles[roleName]
	groupSpec := roleSpec.RoleGroups[groupName]

	foldedConfig, _, err := reconciler.FoldCommonConfig(
		decl.ConfigDefaults, roleSpec.GetConfig(), groupSpec.GetConfig())
	Expect(err).NotTo(HaveOccurred())
	mergedGroupSpec := groupSpec.DeepCopy()
	mergedGroupSpec.Config = foldedConfig

	contribution, err := ResolveRoleGroup(context.Background(), nil, cr, nil)
	Expect(err).NotTo(HaveOccurred())
	derived := &commonsv1alpha1.OverridesSpec{
		ConfigOverrides: contribution.ConfigOverrides,
		EnvOverrides:    contribution.EnvVars,
	}
	merged := config.NewConfigMerger().Merge(derived, roleSpec.GetOverrides(), groupSpec.GetOverrides())
	merged.Logging = foldedConfig.Logging

	imageDefaults := ProductImageDefaults()
	reference, err := spec.Image.ResolveImage(dolphinv1alpha1.DefaultProductName, imageDefaults)
	Expect(err).NotTo(HaveOccurred())

	return &reconciler.RoleGroupBuildContext{
		Declaration: decl,
		ResolvedImage: reconciler.ResolvedImage{
			Reference:      reference,
			PullPolicy:     spec.Image.ResolvedPullPolicy(imageDefaults),
			PullSecretName: spec.Image.ResolvedPullSecretName(imageDefaults),
			ProductVersion: spec.Image.ResolvedProductVersion(imageDefaults),
		},
		ProductName:        dolphinv1alpha1.DefaultProductName,
		ClusterName:        cr.GetName(),
		ClusterNamespace:   cr.GetNamespace(),
		ClusterLabels:      map[string]string{},
		ClusterSpec:        spec,
		RoleName:           roleName,
		RoleSpec:           &roleSpec,
		RoleGroupName:      groupName,
		RoleGroupSpec:      *mergedGroupSpec,
		MergedConfig:       merged,
		ResourceName:       reconciler.RoleGroupResourceName(cr.GetName(), roleName, groupName),
		ServiceAccountName: reconciler.ServiceAccountResourceName("DolphinschedulerCluster", cr.GetName()),
	}
}

func newFakeClient(objs ...ctrlclient.Object) ctrlclient.Client {
	scheme := runtime.NewScheme()
	Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
	Expect(dolphinv1alpha1.AddToScheme(scheme)).To(Succeed())
	Expect(authv1alpha1.AddToScheme(scheme)).To(Succeed())
	Expect(s3v1alpha1.AddToScheme(scheme)).To(Succeed())
	return fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

var _ = Describe("DolphinSchedulerRoleGroupHandler", func() {
	var (
		handler *DolphinSchedulerRoleGroupHandler
		cr      *dolphinv1alpha1.DolphinschedulerCluster
		ctx     context.Context
	)

	BeforeEach(func() {
		handler = NewDolphinSchedulerRoleGroupHandler(clientgoscheme.Scheme)
		cr = newTestCluster()
		ctx = context.Background()
	})

	Describe("role catalog", func() {
		It("declares exactly the four roles, each internally valid", func() {
			catalog, err := handler.DeclareRoles(ctx, newFakeClient(), cr)
			Expect(err).NotTo(HaveOccurred())
			Expect(catalog).To(HaveLen(4))
			for _, role := range []string{
				dolphinv1alpha1.RoleMaster,
				dolphinv1alpha1.RoleWorker,
				dolphinv1alpha1.RoleApi,
				dolphinv1alpha1.RoleAlert,
			} {
				decl, ok := catalog[role]
				Expect(ok).To(BeTrue(), "role %s", role)
				Expect(decl.Validate(role)).To(Succeed(), "role %s", role)
				Expect(decl.MainContainerName).To(Equal(mainContainerNames[role]))
				// The start script is the command's last element; args stay the user's.
				Expect(roleScript(decl.Command)).To(ContainSubstring(`bin/start.sh "$@" &`), "role %s", role)
				Expect(decl.LogVolumeSize).To(Equal(legacyLogVolumeSize))
				Expect(decl.ConfigDefaults).NotTo(BeNil())
				Expect(decl.ConfigDefaults.GracefulShutdownTimeout).To(HaveValue(Equal("120s")))
			}
		})

		It("declares the data volume on the worker only", func() {
			catalog, err := handler.DeclareRoles(ctx, newFakeClient(), cr)
			Expect(err).NotTo(HaveOccurred())
			worker := catalog[dolphinv1alpha1.RoleWorker]
			Expect(worker.DataVolume).To(Equal(&reconciler.DataVolume{
				Name:      "worker-data",
				MountPath: workerDataMountPath,
			}))
			for _, role := range []string{dolphinv1alpha1.RoleMaster, dolphinv1alpha1.RoleApi, dolphinv1alpha1.RoleAlert} {
				Expect(catalog[role].DataVolume).To(BeNil(), "role %s", role)
			}
		})

		It("builds the e2e-pinned resource names, all as StatefulSets", func() {
			for _, role := range []string{
				dolphinv1alpha1.RoleMaster,
				dolphinv1alpha1.RoleWorker,
				dolphinv1alpha1.RoleApi,
				dolphinv1alpha1.RoleAlert,
			} {
				c := newFakeClient()
				decl := declareRole(ctx, handler, c, cr, role)
				buildCtx := newBuildContext(cr, role, decl)
				res, err := handler.BuildResources(ctx, c, cr, buildCtx)
				Expect(err).NotTo(HaveOccurred(), "role %s", role)

				name := fmt.Sprintf("%s-%s-default", testClusterName, role)
				Expect(res.StatefulSet).NotTo(BeNil(), "role %s", role)
				Expect(res.StatefulSet.Name).To(Equal(name))
				Expect(res.ConfigMap.Name).To(Equal(name))
				Expect(res.Service.Name).To(Equal(name))
				Expect(res.HeadlessService.Name).To(Equal(name + "-headless"))
			}
		})

		It("declares a worker storage default that renders the legacy 2Gi claim", func() {
			c := newFakeClient()
			decl := declareRole(ctx, handler, c, cr, dolphinv1alpha1.RoleWorker)
			buildCtx := newBuildContext(cr, dolphinv1alpha1.RoleWorker, decl)
			res, err := handler.BuildResources(ctx, c, cr, buildCtx)
			Expect(err).NotTo(HaveOccurred())

			Expect(res.StatefulSet.Spec.VolumeClaimTemplates).To(HaveLen(1))
			vct := res.StatefulSet.Spec.VolumeClaimTemplates[0]
			Expect(vct.Name).To(Equal("worker-data"))
			Expect(vct.Spec.Resources.Requests.Storage().String()).To(Equal("2Gi"))

			container := res.StatefulSet.Spec.Template.Spec.Containers[0]
			Expect(container.VolumeMounts).To(ContainElement(corev1.VolumeMount{
				Name:      "worker-data",
				MountPath: workerDataMountPath,
			}))
		})
	})

	Describe("runtime dependency fingerprint", func() {
		It("rolls every role when the database credentials Secret version changes", func() {
			for _, role := range []string{
				dolphinv1alpha1.RoleMaster,
				dolphinv1alpha1.RoleWorker,
				dolphinv1alpha1.RoleApi,
				dolphinv1alpha1.RoleAlert,
			} {
				c := newFakeClient()
				decl := declareRole(ctx, handler, c, cr, role)
				res, err := handler.BuildResources(ctx, c, cr, newBuildContext(cr, role, decl))
				Expect(err).NotTo(HaveOccurred(), "role %s", role)
				before := res.StatefulSet.Spec.Template.Annotations[runtimeDependencyFingerprintAnnotation]
				Expect(before).To(HaveLen(64))
				Expect(before).NotTo(ContainSubstring("database-password"))

				secret := &corev1.Secret{}
				key := ctrlclient.ObjectKey{Namespace: cr.Namespace, Name: databaseCredentialsSecretName(cr)}
				Expect(c.Get(ctx, key, secret)).To(Succeed())
				secret.Data[util.ConnectionStringAuthPass] = []byte("rotated-database-password")
				Expect(c.Update(ctx, secret)).To(Succeed())

				decl = declareRole(ctx, handler, c, cr, role)
				res, err = handler.BuildResources(ctx, c, cr, newBuildContext(cr, role, decl))
				Expect(err).NotTo(HaveOccurred(), "role %s", role)
				after := res.StatefulSet.Spec.Template.Annotations[runtimeDependencyFingerprintAnnotation]
				Expect(after).NotTo(Equal(before), "role %s", role)
				Expect(after).NotTo(ContainSubstring("rotated-database-password"))
			}
		})

		It("rolls Pods when either consumed ConfigMap version changes", func() {
			c := newFakeClient()
			role := dolphinv1alpha1.RoleMaster
			decl := declareRole(ctx, handler, c, cr, role)
			res, err := handler.BuildResources(ctx, c, cr, newBuildContext(cr, role, decl))
			Expect(err).NotTo(HaveOccurred())
			previous := res.StatefulSet.Spec.Template.Annotations[runtimeDependencyFingerprintAnnotation]

			for _, name := range []string{envsConfigMapName(cr.Name), cr.Spec.ClusterConfig.ZookeeperConfigMapName} {
				configMap := &corev1.ConfigMap{}
				key := ctrlclient.ObjectKey{Namespace: cr.Namespace, Name: name}
				Expect(c.Get(ctx, key, configMap)).To(Succeed())
				configMap.Data = map[string]string{"revision": configMap.ResourceVersion}
				Expect(c.Update(ctx, configMap)).To(Succeed())

				decl = declareRole(ctx, handler, c, cr, role)
				res, err = handler.BuildResources(ctx, c, cr, newBuildContext(cr, role, decl))
				Expect(err).NotTo(HaveOccurred())
				fingerprint := res.StatefulSet.Spec.Template.Annotations[runtimeDependencyFingerprintAnnotation]
				Expect(fingerprint).NotTo(Equal(previous), "ConfigMap %s", name)
				previous = fingerprint
			}
		})

		It("rolls Pods when inline S3 configuration changes", func() {
			cr.Spec.ClusterConfig.S3 = &s3v1alpha1.S3BucketSpec{
				BucketName: testS3BucketName,
				Connection: &s3v1alpha1.S3BucketConnectionSpec{Inline: &s3v1alpha1.S3ConnectionSpec{
					Host: "minio", Port: 9000, Region: testS3DefaultRegion, PathStyle: true,
				}},
			}
			c := newFakeClient()
			role := dolphinv1alpha1.RoleMaster
			decl := declareRole(ctx, handler, c, cr, role)
			res, err := handler.BuildResources(ctx, c, cr, newBuildContext(cr, role, decl))
			Expect(err).NotTo(HaveOccurred())
			before := res.StatefulSet.Spec.Template.Annotations[runtimeDependencyFingerprintAnnotation]

			cr.Spec.ClusterConfig.S3.Connection.Inline.Region = testS3AlternateRegion
			decl = declareRole(ctx, handler, c, cr, role)
			res, err = handler.BuildResources(ctx, c, cr, newBuildContext(cr, role, decl))
			Expect(err).NotTo(HaveOccurred())
			after := res.StatefulSet.Spec.Template.Annotations[runtimeDependencyFingerprintAnnotation]
			Expect(after).NotTo(Equal(before))
			Expect(res.ConfigMap.Data[dolphinv1alpha1.DolphinCommonPropertiesName]).To(
				ContainSubstring("resource.aws.region=" + testS3AlternateRegion + "\n"))
		})

		It("rolls Pods when a referenced S3Connection version changes", func() {
			cr.Spec.ClusterConfig.S3 = &s3v1alpha1.S3BucketSpec{
				BucketName: testS3BucketName,
				Connection: &s3v1alpha1.S3BucketConnectionSpec{Reference: testS3ConnectionName},
			}
			connection := &s3v1alpha1.S3Connection{
				ObjectMeta: metav1.ObjectMeta{Name: testS3ConnectionName, Namespace: testNamespace},
				Spec: s3v1alpha1.S3ConnectionSpec{
					Host: testS3Host, Port: 443, Region: testS3DefaultRegion, PathStyle: true, Tls: &s3v1alpha1.Tls{},
				},
			}
			c := newFakeClient(connection)
			role := dolphinv1alpha1.RoleWorker
			decl := declareRole(ctx, handler, c, cr, role)
			res, err := handler.BuildResources(ctx, c, cr, newBuildContext(cr, role, decl))
			Expect(err).NotTo(HaveOccurred())
			before := res.StatefulSet.Spec.Template.Annotations[runtimeDependencyFingerprintAnnotation]

			stored := &s3v1alpha1.S3Connection{}
			Expect(c.Get(ctx, ctrlclient.ObjectKeyFromObject(connection), stored)).To(Succeed())
			stored.Spec.Region = testS3AlternateRegion
			Expect(c.Update(ctx, stored)).To(Succeed())
			decl = declareRole(ctx, handler, c, cr, role)
			res, err = handler.BuildResources(ctx, c, cr, newBuildContext(cr, role, decl))
			Expect(err).NotTo(HaveOccurred())
			after := res.StatefulSet.Spec.Template.Annotations[runtimeDependencyFingerprintAnnotation]
			Expect(after).NotTo(Equal(before))
			Expect(after).NotTo(ContainSubstring(stored.ResourceVersion))
		})

		It("includes OIDC credential Secrets only for the api role", func() {
			authClass := &authv1alpha1.AuthenticationClass{
				ObjectMeta: metav1.ObjectMeta{Name: string(security.Keycloak)},
				Spec: authv1alpha1.AuthenticationClassSpec{AuthenticationProvider: &authv1alpha1.AuthenticationProvider{
					OIDC: &authv1alpha1.OIDCProvider{
						Hostname: "keycloak.example.com", PrincipalClaim: "preferred_username", ProviderHint: string(security.Keycloak),
					},
				}},
			}
			oidcSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "oidc-credentials", Namespace: cr.Namespace},
				Data: map[string][]byte{
					security.OidcClientIdKey: []byte("client"),
					security.OidcSecretKey:   []byte("credential-value-must-not-be-rendered"),
				},
			}
			cr.Spec.ClusterConfig.Authentication = []dolphinv1alpha1.AuthenticationSpec{{
				AuthenticationClass: string(security.Keycloak),
				Oidc:                &dolphinv1alpha1.OidcCredentialSecretSpec{Secret: oidcSecret.Name},
			}}
			c := newFakeClient(authClass, oidcSecret)

			fingerprints := map[string]string{}
			for _, role := range []string{dolphinv1alpha1.RoleMaster, dolphinv1alpha1.RoleApi} {
				decl := declareRole(ctx, handler, c, cr, role)
				res, err := handler.BuildResources(ctx, c, cr, newBuildContext(cr, role, decl))
				Expect(err).NotTo(HaveOccurred())
				fingerprints[role] = res.StatefulSet.Spec.Template.Annotations[runtimeDependencyFingerprintAnnotation]
				Expect(fingerprints[role]).NotTo(ContainSubstring("credential-value-must-not-be-rendered"))
			}

			stored := &corev1.Secret{}
			Expect(c.Get(ctx, ctrlclient.ObjectKeyFromObject(oidcSecret), stored)).To(Succeed())
			stored.Data[security.OidcSecretKey] = []byte("rotated-oidc-secret")
			Expect(c.Update(ctx, stored)).To(Succeed())

			for _, role := range []string{dolphinv1alpha1.RoleMaster, dolphinv1alpha1.RoleApi} {
				decl := declareRole(ctx, handler, c, cr, role)
				res, err := handler.BuildResources(ctx, c, cr, newBuildContext(cr, role, decl))
				Expect(err).NotTo(HaveOccurred())
				updated := res.StatefulSet.Spec.Template.Annotations[runtimeDependencyFingerprintAnnotation]
				if role == dolphinv1alpha1.RoleApi {
					Expect(updated).NotTo(Equal(fingerprints[role]))
				} else {
					Expect(updated).To(Equal(fingerprints[role]))
				}
			}
		})

		It("fails clearly when a runtime dependency is missing", func() {
			c := newFakeClient()
			role := dolphinv1alpha1.RoleMaster
			decl := declareRole(ctx, handler, c, cr, role)
			buildCtx := newBuildContext(cr, role, decl)
			missing := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				Name: cr.Spec.ClusterConfig.ZookeeperConfigMapName, Namespace: cr.Namespace,
			}}
			Expect(c.Delete(ctx, missing)).To(Succeed())

			_, err := handler.BuildResources(ctx, c, cr, buildCtx)
			Expect(err).To(MatchError(ContainSubstring("failed to resolve runtime ConfigMap dependency default/test-znode")))
		})

		It("rejects a dependency without an API resourceVersion", func() {
			baseClient := newFakeClient()
			role := dolphinv1alpha1.RoleMaster
			decl := declareRole(ctx, handler, baseClient, cr, role)
			client := resourceVersionClearingClient{
				Client: baseClient, kind: runtimeDependencyKindConfigMap, name: envsConfigMapName(cr.Name),
			}

			_, err := handler.BuildResources(ctx, client, cr, newBuildContext(cr, role, decl))
			Expect(err).To(MatchError(ContainSubstring("runtime ConfigMap dependency default/test-dolphinscheduler-envs has no resourceVersion")))
		})
	})

	Describe("metrics service", func() {
		It("selects the Gen3 pods and targets the role's metrics port", func() {
			c := newFakeClient()
			decl := declareRole(ctx, handler, c, cr, dolphinv1alpha1.RoleMaster)
			buildCtx := newBuildContext(cr, dolphinv1alpha1.RoleMaster, decl)
			res, err := handler.BuildResources(ctx, c, cr, buildCtx)
			Expect(err).NotTo(HaveOccurred())

			svc := res.MetricsService
			Expect(svc).NotTo(BeNil())
			Expect(svc.Name).To(Equal(testClusterName + "-master-default-metrics"))
			Expect(svc.Spec.ClusterIP).To(Equal(corev1.ClusterIPNone))
			Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
			Expect(svc.Spec.Ports).To(ConsistOf(corev1.ServicePort{
				Name:       "metrics",
				Port:       5679,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt(5679),
			}))
			Expect(svc.Labels).To(HaveKeyWithValue("prometheus.io/scrape", defaultEnabled))
			Expect(svc.Annotations).To(Equal(map[string]string{
				"prometheus.io/scrape": defaultEnabled,
				"prometheus.io/path":   "/actuator/prometheus",
				"prometheus.io/port":   "5679",
				"prometheus.io/scheme": "http",
			}))
			Expect(svc.Spec.Selector).To(Equal(map[string]string{
				"dolphinscheduler.kubedoop.dev/cluster":    testClusterName,
				"dolphinscheduler.kubedoop.dev/role":       "master",
				"dolphinscheduler.kubedoop.dev/role-group": defaultRoleGroupName,
			}))
			for key, value := range svc.Spec.Selector {
				Expect(res.StatefulSet.Spec.Template.Labels).To(HaveKeyWithValue(key, value))
			}
		})

		It("uses the role-specific metrics port", func() {
			ports := map[string]int32{
				dolphinv1alpha1.RoleMaster: 5679,
				dolphinv1alpha1.RoleWorker: 1235,
				dolphinv1alpha1.RoleApi:    12345,
				dolphinv1alpha1.RoleAlert:  50053,
			}
			for role, port := range ports {
				c := newFakeClient()
				decl := declareRole(ctx, handler, c, cr, role)
				buildCtx := newBuildContext(cr, role, decl)
				res, err := handler.BuildResources(ctx, c, cr, buildCtx)
				Expect(err).NotTo(HaveOccurred(), "role %s", role)
				Expect(res.MetricsService.Spec.Ports[0].Port).To(Equal(port), "role %s", role)
				Expect(res.MetricsService.Annotations["prometheus.io/port"]).To(Equal(fmt.Sprintf("%d", port)), "role %s", role)
			}
		})
	})

	Describe("master container", func() {
		var container corev1.Container

		BeforeEach(func() {
			c := newFakeClient()
			decl := declareRole(ctx, handler, c, cr, dolphinv1alpha1.RoleMaster)
			buildCtx := newBuildContext(cr, dolphinv1alpha1.RoleMaster, decl)
			res, err := handler.BuildResources(ctx, c, cr, buildCtx)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.StatefulSet.Spec.Template.Spec.Containers).To(HaveLen(1))
			container = res.StatefulSet.Spec.Template.Spec.Containers[0]
		})

		It("keeps the legacy container name and carries the start script in the command", func() {
			Expect(container.Name).To(Equal("master-server"))
			script := roleScript(container.Command)
			Expect(script).To(ContainSubstring("prepare_signal_handlers"))
			Expect(script).To(ContainSubstring(`master-server/bin/start.sh "$@" &`))
			Expect(script).To(ContainSubstring("wait_for_termination $!"))
			// The Vector shutdown-marker lines are gone (native sidecar).
			Expect(script).NotTo(ContainSubstring("_vector/shutdown"))
			// The start block appears exactly once.
			Expect(strings.Count(script, `bin/start.sh "$@" &`)).To(Equal(1))
			// Args belong to the user's cliOverrides alone now.
			Expect(container.Args).To(BeEmpty())
		})

		It("forwards role-group cliOverrides to the product start script", func() {
			group := cr.Spec.Master.RoleGroups[defaultRoleGroupName]
			group.OverridesSpec = &commonsv1alpha1.OverridesSpec{
				CliOverrides: []string{"--spring.profiles.active=prod", "--server.port=6789"},
			}
			cr.Spec.Master.RoleGroups[defaultRoleGroupName] = group
			c := newFakeClient()
			decl := declareRole(ctx, handler, c, cr, dolphinv1alpha1.RoleMaster)
			buildCtx := newBuildContext(cr, dolphinv1alpha1.RoleMaster, decl)
			res, err := handler.BuildResources(ctx, c, cr, buildCtx)
			Expect(err).NotTo(HaveOccurred())

			got := res.StatefulSet.Spec.Template.Spec.Containers[0]
			Expect(got.Args).To(Equal(group.CliOverrides))
			Expect(roleScript(got.Command)).To(ContainSubstring(`master-server/bin/start.sh "$@" &`))
		})

		It("renders the exact legacy env list plus the product default env", func() {
			names := make([]string, 0, len(container.Env))
			for _, e := range container.Env {
				names = append(names, e.Name)
			}
			Expect(names).To(Equal([]string{
				envJavaOpts,
				"MASTER_DISPATCH_TASK_NUM",
				"MASTER_EXEC_TASK_NUM",
				"MASTER_EXEC_THREADS",
				"MASTER_FAILOVER_INTERVAL",
				"MASTER_HEARTBEAT_ERROR_THRESHOLD",
				"MASTER_HOST_SELECTOR",
				"MASTER_KILL_APPLICATION_WHEN_HANDLE_FAILOVER",
				"MASTER_MAX_HEARTBEAT_INTERVAL",
				"MASTER_SERVER_LOAD_PROTECTION_ENABLED",
				"MASTER_SERVER_LOAD_PROTECTION_MAX_DISK_USAGE_PERCENTAGE_THRESHOLDS",
				"MASTER_SERVER_LOAD_PROTECTION_MAX_JVM_CPU_USAGE_PERCENTAGE_THRESHOLDS",
				"MASTER_SERVER_LOAD_PROTECTION_MAX_SYSTEM_CPU_USAGE_PERCENTAGE_THRESHOLDS",
				"MASTER_SERVER_LOAD_PROTECTION_MAX_SYSTEM_MEMORY_USAGE_PERCENTAGE_THRESHOLDS",
				"MASTER_STATE_WHEEL_INTERVAL",
				"MASTER_TASK_COMMIT_INTERVAL",
				"MASTER_TASK_COMMIT_RETRYTIMES",
				zkConnectStringEnvName,
				"SPRING_DATASOURCE_USERNAME",
				"SPRING_DATASOURCE_PASSWORD",
				// From the derived contribution (also present via envFrom "<cluster>-envs").
				"SPRING_JACKSON_TIME_ZONE",
				"TZ",
			}))
			Expect(container.Env).To(ContainElements(databaseCredentialEnv(cr)))

			Expect(container.Env[0].Value).To(Equal("-Xms512m -Xmx512m -Xmn256m"))

			var zkEnv *corev1.EnvVar
			for i := range container.Env {
				if container.Env[i].Name == zkConnectStringEnvName {
					zkEnv = &container.Env[i]
				}
			}
			Expect(zkEnv).NotTo(BeNil())
			Expect(zkEnv.ValueFrom.ConfigMapKeyRef.Name).To(Equal("test-znode"))
			Expect(zkEnv.ValueFrom.ConfigMapKeyRef.Key).To(Equal("ZOOKEEPER"))

			Expect(container.EnvFrom).To(HaveLen(1))
			Expect(container.EnvFrom[0].ConfigMapRef.Name).To(Equal(testClusterName + "-envs"))
		})

		It("keeps the legacy actuator probes and adds a startup probe", func() {
			Expect(container.ReadinessProbe).NotTo(BeNil())
			Expect(container.ReadinessProbe.Exec.Command).To(Equal([]string{
				curlCommand, "http://localhost:5679/actuator/health/readiness",
			}))
			Expect(container.ReadinessProbe.InitialDelaySeconds).To(Equal(int32(30)))
			Expect(container.ReadinessProbe.PeriodSeconds).To(Equal(int32(30)))
			Expect(container.ReadinessProbe.TimeoutSeconds).To(Equal(int32(5)))
			Expect(container.ReadinessProbe.FailureThreshold).To(Equal(int32(3)))
			Expect(container.ReadinessProbe.SuccessThreshold).To(Equal(int32(1)))

			Expect(container.LivenessProbe).NotTo(BeNil())
			Expect(container.LivenessProbe.Exec.Command).To(Equal([]string{
				curlCommand, "http://localhost:5679/actuator/health/liveness",
			}))

			Expect(container.StartupProbe).NotTo(BeNil())
			Expect(container.StartupProbe.Exec.Command).To(Equal([]string{
				curlCommand, "http://localhost:5679/actuator/health/readiness",
			}))
			Expect(container.StartupProbe.PeriodSeconds).To(Equal(int32(10)))
			Expect(container.StartupProbe.FailureThreshold).To(Equal(int32(60)))
		})

		It("mounts the config files at the legacy subPath targets and keeps the framework mount", func() {
			Expect(container.VolumeMounts[0]).To(Equal(corev1.VolumeMount{
				Name:      configVolumeName,
				MountPath: "/kubedoop/dolphinscheduler/master-server/conf/common.properties",
				SubPath:   "common.properties",
			}))
			Expect(container.VolumeMounts[1]).To(Equal(corev1.VolumeMount{
				Name:      configVolumeName,
				MountPath: "/kubedoop/dolphinscheduler/master-server/conf/logback-spring.xml",
				SubPath:   "logback-spring.xml",
			}))
			// The framework's whole-directory config mount is kept (harmless read-only dir).
			Expect(container.VolumeMounts).To(ContainElement(corev1.VolumeMount{
				Name:      configVolumeName,
				MountPath: opgoconstant.KubedoopConfigDirMount,
				ReadOnly:  true,
			}))
		})

		It("renders the legacy container ports and resources", func() {
			Expect(container.Ports).To(Equal([]corev1.ContainerPort{
				{Name: "actual-port", ContainerPort: 5679, Protocol: corev1.ProtocolTCP},
				{Name: "port", ContainerPort: 5678, Protocol: corev1.ProtocolTCP},
			}))
			Expect(container.Resources.Requests.Cpu().String()).To(Equal("300m"))
			Expect(container.Resources.Limits.Cpu().String()).To(Equal("500m"))
			Expect(container.Resources.Limits.Memory().String()).To(Equal("800Mi"))
		})
	})

	Describe("alert role", func() {
		It("keeps the historical alerter-server container name over the alert-server binary", func() {
			c := newFakeClient()
			decl := declareRole(ctx, handler, c, cr, dolphinv1alpha1.RoleAlert)
			buildCtx := newBuildContext(cr, dolphinv1alpha1.RoleAlert, decl)
			res, err := handler.BuildResources(ctx, c, cr, buildCtx)
			Expect(err).NotTo(HaveOccurred())
			container := res.StatefulSet.Spec.Template.Spec.Containers[0]
			Expect(container.Name).To(Equal("alerter-server"))
			Expect(roleScript(container.Command)).To(ContainSubstring(`alert-server/bin/start.sh "$@" &`))
			Expect(container.VolumeMounts[0].MountPath).To(
				Equal("/kubedoop/dolphinscheduler/alert-server/conf/common.properties"))
		})
	})

	Describe("config map", func() {
		It("renders common.properties with the legacy sorted serializer and defaults", func() {
			c := newFakeClient()
			decl := declareRole(ctx, handler, c, cr, dolphinv1alpha1.RoleMaster)
			buildCtx := newBuildContext(cr, dolphinv1alpha1.RoleMaster, decl)
			res, err := handler.BuildResources(ctx, c, cr, buildCtx)
			Expect(err).NotTo(HaveOccurred())

			Expect(res.ConfigMap.Data).To(HaveKey("common.properties"))
			Expect(res.ConfigMap.Data).To(HaveKey("logback-spring.xml"))

			properties := res.ConfigMap.Data["common.properties"]
			Expect(properties).To(ContainSubstring("data.basedir.path=/tmp/dolphinscheduler\n"))
			Expect(properties).To(ContainSubstring("resource.storage.type=LOCAL\n"))
			// Sorted, raw "k=v" lines (the legacy serializer, not the escaping properties adapter).
			Expect(properties).To(ContainSubstring("datasource.encryption.salt=!@#$%^&*\n"))
			lines := strings.Split(strings.TrimSpace(properties), "\n")
			Expect(lines).To(HaveLen(34))
		})

		It("lets user configOverrides win over the product defaults", func() {
			cr.Spec.Master.OverridesSpec = &commonsv1alpha1.OverridesSpec{
				ConfigOverrides: map[string]map[string]string{
					"common.properties": {"data.basedir.path": "/custom"},
				},
			}
			c := newFakeClient()
			decl := declareRole(ctx, handler, c, cr, dolphinv1alpha1.RoleMaster)
			buildCtx := newBuildContext(cr, dolphinv1alpha1.RoleMaster, decl)
			res, err := handler.BuildResources(ctx, c, cr, buildCtx)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.ConfigMap.Data["common.properties"]).To(ContainSubstring("data.basedir.path=/custom\n"))
		})
	})

	Describe("S3 configuration", func() {
		It("delivers SecretClass credentials through CSI without writing them to the ConfigMap", func() {
			cr.Spec.ClusterConfig.S3 = &s3v1alpha1.S3BucketSpec{
				BucketName: testS3BucketName,
				Connection: &s3v1alpha1.S3BucketConnectionSpec{
					Inline: &s3v1alpha1.S3ConnectionSpec{
						Host:      "minio",
						Port:      9000,
						Region:    testS3DefaultRegion,
						PathStyle: true,
						Credentials: &commonsv1alpha1.Credentials{
							SecretClass: "s3-credentials",
						},
					},
				},
			}
			c := newFakeClient()
			decl := declareRole(ctx, handler, c, cr, dolphinv1alpha1.RoleMaster)
			buildCtx := newBuildContext(cr, dolphinv1alpha1.RoleMaster, decl)
			res, err := handler.BuildResources(ctx, c, cr, buildCtx)
			Expect(err).NotTo(HaveOccurred())

			properties := res.ConfigMap.Data[dolphinv1alpha1.DolphinCommonPropertiesName]
			Expect(properties).To(ContainSubstring("resource.storage.type=S3\n"))
			Expect(properties).To(ContainSubstring("resource.aws.region=" + testS3DefaultRegion + "\n"))
			Expect(properties).To(ContainSubstring("resource.aws.s3.bucket.name=" + testS3BucketName + "\n"))
			Expect(properties).To(ContainSubstring("resource.aws.s3.endpoint=http://minio:9000\n"))
			Expect(properties).NotTo(ContainSubstring("resource.aws.access.key.id="))
			Expect(properties).NotTo(ContainSubstring("resource.aws.secret.access.key="))

			container := res.StatefulSet.Spec.Template.Spec.Containers[0]
			script := roleScript(container.Command)
			Expect(script).To(ContainSubstring(
				`AWS_ACCESS_KEY_ID="$(cat "/kubedoop/secret/s3-credentials/ACCESS_KEY")"`))
			Expect(script).To(ContainSubstring(`export ` + s3AccessKeyEnvName + `="$AWS_ACCESS_KEY_ID"`))
			Expect(script).To(ContainSubstring(`export ` + s3SecretKeyEnvName + `="$AWS_SECRET_ACCESS_KEY"`))

			Expect(container.VolumeMounts).To(ContainElement(corev1.VolumeMount{
				Name:      opgos3.DefaultCredentialsVolumeName,
				ReadOnly:  true,
				MountPath: opgos3.CredentialsMountPath(opgos3.DefaultCredentialsVolumeName),
			}))
			var credentialsVolume *corev1.Volume
			for i := range res.StatefulSet.Spec.Template.Spec.Volumes {
				volume := &res.StatefulSet.Spec.Template.Spec.Volumes[i]
				if volume.Name == opgos3.DefaultCredentialsVolumeName {
					credentialsVolume = volume
				}
			}
			Expect(credentialsVolume).NotTo(BeNil())
			Expect(credentialsVolume.Ephemeral.VolumeClaimTemplate.Annotations).To(
				HaveKeyWithValue(opgosecurity.SecretClassAnnotation, "s3-credentials"))
		})

		It("resolves a referenced TLS S3Connection through the framework", func() {
			cr.Spec.ClusterConfig.S3 = &s3v1alpha1.S3BucketSpec{
				BucketName: testS3BucketName,
				Connection: &s3v1alpha1.S3BucketConnectionSpec{Reference: testS3ConnectionName},
			}
			connection := &s3v1alpha1.S3Connection{
				ObjectMeta: metav1.ObjectMeta{Name: testS3ConnectionName, Namespace: testNamespace},
				Spec: s3v1alpha1.S3ConnectionSpec{
					Host:      testS3Host,
					Port:      443,
					Region:    testS3AlternateRegion,
					PathStyle: true,
					Tls:       &s3v1alpha1.Tls{},
				},
			}
			c := newFakeClient(connection)
			decl := declareRole(ctx, handler, c, cr, dolphinv1alpha1.RoleApi)
			buildCtx := newBuildContext(cr, dolphinv1alpha1.RoleApi, decl)
			res, err := handler.BuildResources(ctx, c, cr, buildCtx)
			Expect(err).NotTo(HaveOccurred())

			properties := res.ConfigMap.Data[dolphinv1alpha1.DolphinCommonPropertiesName]
			Expect(properties).To(ContainSubstring("resource.aws.s3.endpoint=https://" + testS3Host + ":443\n"))
			Expect(properties).To(ContainSubstring("resource.aws.region=" + testS3AlternateRegion + "\n"))
		})

		It("rejects virtual-host addressing that DolphinScheduler cannot honor", func() {
			cr.Spec.ClusterConfig.S3 = &s3v1alpha1.S3BucketSpec{
				BucketName: testS3BucketName,
				Connection: &s3v1alpha1.S3BucketConnectionSpec{Inline: &s3v1alpha1.S3ConnectionSpec{
					Host: testS3Host,
				}},
			}

			_, err := handler.DeclareRoles(ctx, newFakeClient(), cr)
			Expect(err).To(MatchError(ContainSubstring("S3 pathStyle must be true")))
		})

		It("rejects an S3 custom CA until JVM trust material can be provisioned", func() {
			cr.Spec.ClusterConfig.S3 = &s3v1alpha1.S3BucketSpec{
				BucketName: testS3BucketName,
				Connection: &s3v1alpha1.S3BucketConnectionSpec{Inline: &s3v1alpha1.S3ConnectionSpec{
					Host:      testS3Host,
					PathStyle: true,
					Tls: &s3v1alpha1.Tls{Verification: &commonsv1alpha1.TLSVerificationSpec{
						Server: &commonsv1alpha1.ServerVerification{CACert: &commonsv1alpha1.CACert{
							SecretClass: "company-ca",
						}},
					}},
				}},
			}

			_, err := handler.DeclareRoles(ctx, newFakeClient(), cr)
			Expect(err).To(MatchError(ContainSubstring("S3 TLS custom CA SecretClass is not supported")))
		})
	})

	Describe("services", func() {
		It("keeps the legacy service port names and numbers", func() {
			c := newFakeClient()
			decl := declareRole(ctx, handler, c, cr, dolphinv1alpha1.RoleWorker)
			buildCtx := newBuildContext(cr, dolphinv1alpha1.RoleWorker, decl)
			res, err := handler.BuildResources(ctx, c, cr, buildCtx)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Service.Spec.Ports).To(Equal([]corev1.ServicePort{
				{Name: "actual-port", Port: 1235, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromString("actual-port")},
				{Name: "port", Port: 1234, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromString("port")},
			}))
		})
	})

	Describe("api role with LDAP authentication", func() {
		It("adds the LDAP env, the bind-credentials volume and a single start block", func() {
			authClass := &authv1alpha1.AuthenticationClass{
				ObjectMeta: metav1.ObjectMeta{Name: "ldap"},
				Spec: authv1alpha1.AuthenticationClassSpec{
					AuthenticationProvider: &authv1alpha1.AuthenticationProvider{
						LDAP: &authv1alpha1.LDAPProvider{
							Hostname: "openldap",
							Port:     1389,
							BindCredentials: &commonsv1alpha1.Credentials{
								SecretClass: "ldap-credentials",
							},
							SearchBase: "dc=example,dc=org",
						},
					},
				},
			}
			cr.Spec.ClusterConfig.Authentication = []dolphinv1alpha1.AuthenticationSpec{
				{AuthenticationClass: "ldap"},
			}

			// Auth resolution happens in DeclareRoles (env + script prologue) and again in
			// BuildResources (the CSI bind-credentials volume), so both get the same client.
			c := newFakeClient(authClass)
			decl := declareRole(ctx, handler, c, cr, dolphinv1alpha1.RoleApi)
			buildCtx := newBuildContext(cr, dolphinv1alpha1.RoleApi, decl)
			res, err := handler.BuildResources(ctx, c, cr, buildCtx)
			Expect(err).NotTo(HaveOccurred())

			container := res.StatefulSet.Spec.Template.Spec.Containers[0]

			names := make([]string, 0, len(container.Env))
			zkEnvCount := 0
			for _, e := range container.Env {
				names = append(names, e.Name)
				if e.Name == "REGISTRY_ZOOKEEPER_CONNECT_STRING" {
					zkEnvCount++
				}
			}
			Expect(names).To(ContainElement("SECURITY_AUTHENTICATION_TYPE"))
			Expect(names).To(ContainElement("SECURITY_AUTHENTICATION_LDAP_URLS"))
			// The ZooKeeper env appears exactly once (the legacy api rendering duplicated it).
			Expect(zkEnvCount).To(Equal(1))

			// The credentials-export prologue precedes exactly one start block (the legacy api
			// rendering duplicated the block).
			script := roleScript(container.Command)
			Expect(script).To(HavePrefix("SECURITY_AUTHENTICATION_LDAP_USERNAME"))
			Expect(script).NotTo(ContainSubstring("echo"))
			Expect(strings.Count(script, `bin/start.sh "$@" &`)).To(Equal(1))

			// The bind-credentials CSI volume is mounted read-only at the legacy path.
			var ldapMount *corev1.VolumeMount
			for i := range container.VolumeMounts {
				if container.VolumeMounts[i].Name == dolphinv1alpha1.LdapBindCredintialsVolumeName {
					ldapMount = &container.VolumeMounts[i]
				}
			}
			Expect(ldapMount).NotTo(BeNil())
			Expect(ldapMount.MountPath).To(Equal("/kubedoop/secret/ldap-bind-credentials"))

			var ldapVolume *corev1.Volume
			podSpec := res.StatefulSet.Spec.Template.Spec
			for i := range podSpec.Volumes {
				if podSpec.Volumes[i].Name == dolphinv1alpha1.LdapBindCredintialsVolumeName {
					ldapVolume = &podSpec.Volumes[i]
				}
			}
			Expect(ldapVolume).NotTo(BeNil())
			pvcTemplate := ldapVolume.Ephemeral.VolumeClaimTemplate
			Expect(pvcTemplate.Annotations).To(HaveKeyWithValue("secrets.kubedoop.dev/class", "ldap-credentials"))
			Expect(pvcTemplate.Spec.Resources.Requests.Storage().String()).To(Equal("1Mi"))
		})
	})
})
