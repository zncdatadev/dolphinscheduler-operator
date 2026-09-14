package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/util"
	authv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/authentication/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func newOwnedLegacyStatefulSet(
	cr *dolphinv1alpha1.DolphinschedulerCluster,
	scheme *runtime.Scheme,
	roleName string,
	groupName string,
) *appsv1.StatefulSet {
	GinkgoHelper()
	name := testClusterName + "-" + roleName + "-" + groupName
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
		Spec: appsv1.StatefulSetSpec{
			// Gen2b used the workload name itself; Gen3 uses "<name>-headless".
			ServiceName: name,
		},
	}
	Expect(controllerutil.SetControllerReference(cr, statefulSet, scheme)).To(Succeed())
	return statefulSet
}

func newOwnedLegacyDeployment(
	cr *dolphinv1alpha1.DolphinschedulerCluster,
	scheme *runtime.Scheme,
	roleName string,
) *appsv1.Deployment {
	GinkgoHelper()
	deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Name:      testClusterName + "-" + roleName + "-" + defaultRoleGroupName,
		Namespace: testNamespace,
	}}
	Expect(controllerutil.SetControllerReference(cr, deployment, scheme)).To(Succeed())
	return deployment
}

func desiredDBAnnotations(cr *dolphinv1alpha1.DolphinschedulerCluster) map[string]string {
	GinkgoHelper()
	image, target, err := desiredDBInitIdentity(cr)
	Expect(err).NotTo(HaveOccurred())
	credentialsVersion, err := databaseCredentialsVersionHash(
		context.Background(), newFakeClient(testDBCredentialsSecret(cr)), cr)
	Expect(err).NotTo(HaveOccurred())
	return map[string]string{
		dbInitImageAnnotation:       image,
		dbInitTargetAnnotation:      target,
		dbInitCredentialsAnnotation: credentialsVersion,
	}
}

func testDBCredentialsSecret(cr *dolphinv1alpha1.DolphinschedulerCluster) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: databaseCredentialsSecretName(cr), Namespace: cr.Namespace, ResourceVersion: "1",
		},
		Data: map[string][]byte{
			util.ConnectionStringAuthUser: []byte("dolphinscheduler"),
			util.ConnectionStringAuthPass: []byte("database-password"),
		},
	}
}

func newDBInitFakeClient(
	cr *dolphinv1alpha1.DolphinschedulerCluster,
	objects ...ctrlclient.Object,
) ctrlclient.Client {
	GinkgoHelper()
	return newFakeClient(append(objects, testDBCredentialsSecret(cr))...)
}

// The Gen2b-to-Gen3 upgrade guard stops before applying role groups: master/worker StatefulSets
// carry incompatible immutable fields, while api/alert Deployments could run alongside their new
// same-named StatefulSets. Workload deletion remains an explicit, PVC-preserving operator action.
var _ = Describe("ClusterExtension upgrade guard", func() {
	var (
		ctx       context.Context
		scheme    *runtime.Scheme
		extension *ClusterExtension
		cr        *dolphinv1alpha1.DolphinschedulerCluster
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(dolphinv1alpha1.AddToScheme(scheme)).To(Succeed())
		Expect(authv1alpha1.AddToScheme(scheme)).To(Succeed())
		extension = NewClusterExtension(scheme)
		cr = newTestCluster()
		cr.UID = types.UID("cluster-uid")
	})

	DescribeTable("detects each legacy workload kind independently",
		func(roleName string, statefulSet bool) {
			name := testClusterName + "-" + roleName + "-" + defaultRoleGroupName
			var err error
			if statefulSet {
				legacy := newOwnedLegacyStatefulSet(cr, scheme, roleName, defaultRoleGroupName)
				err = extension.ensureNoLegacyWorkloads(ctx, newFakeClient(legacy), cr)
				Expect(err).To(MatchError(ContainSubstring("delete statefulset " + name)))
			} else {
				legacy := newOwnedLegacyDeployment(cr, scheme, roleName)
				err = extension.ensureNoLegacyWorkloads(ctx, newFakeClient(legacy), cr)
				Expect(err).To(MatchError(ContainSubstring("delete deployment " + name)))
			}
			Expect(err.Error()).To(ContainSubstring(name))
		},
		Entry("master StatefulSet", dolphinv1alpha1.RoleMaster, true),
		Entry("worker StatefulSet", dolphinv1alpha1.RoleWorker, true),
		Entry("api Deployment", dolphinv1alpha1.RoleApi, false),
		Entry("alert Deployment", dolphinv1alpha1.RoleAlert, false),
	)

	It("reports every leftover in deterministic, directly runnable commands", func() {
		master := newOwnedLegacyStatefulSet(cr, scheme, dolphinv1alpha1.RoleMaster, defaultRoleGroupName)
		worker := newOwnedLegacyStatefulSet(cr, scheme, dolphinv1alpha1.RoleWorker, defaultRoleGroupName)
		api := newOwnedLegacyDeployment(cr, scheme, dolphinv1alpha1.RoleApi)
		alert := newOwnedLegacyDeployment(cr, scheme, dolphinv1alpha1.RoleAlert)

		err := extension.ensureNoLegacyWorkloads(ctx, newFakeClient(worker, api, master, alert), cr)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(
			"StatefulSet(s) " + testClusterName + "-master-default, " + testClusterName + "-worker-default"))
		Expect(err.Error()).To(ContainSubstring(
			"Deployment(s) " + testClusterName + "-alert-default, " + testClusterName + "-api-default"))
		Expect(err.Error()).To(ContainSubstring(
			"`kubectl -n " + testNamespace + " delete statefulset " +
				testClusterName + "-master-default " + testClusterName + "-worker-default " +
				"--cascade=foreground --wait=true`"))
		Expect(err.Error()).To(ContainSubstring(
			"`kubectl -n " + testNamespace + " delete deployment " +
				testClusterName + "-alert-default " + testClusterName + "-api-default " +
				"--cascade=foreground --wait=true`"))
		Expect(err.Error()).To(ContainSubstring("will not delete resources automatically"))
		Expect(err.Error()).To(ContainSubstring("do not delete PersistentVolumeClaims"))
		Expect(err.Error()).To(ContainSubstring("worker data PVCs are retained"))
	})

	It("detects an owned legacy role group even when it is no longer declared", func() {
		legacy := newOwnedLegacyStatefulSet(cr, scheme, dolphinv1alpha1.RoleMaster, "retired")

		err := extension.ensureNoLegacyWorkloads(ctx, newFakeClient(legacy), cr)
		Expect(err).To(MatchError(ContainSubstring(testClusterName + "-master-retired")))
	})

	It("reports incompatible client Services and fixed-name legacy RBAC", func() {
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testClusterName + "-master-" + defaultRoleGroupName,
				Namespace: testNamespace,
			},
			Spec: corev1.ServiceSpec{ClusterIP: corev1.ClusterIPNone},
		}
		serviceAccount := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
			Name: dolphinv1alpha1.DefaultProductName, Namespace: testNamespace,
		}}
		role := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{
			Name: dolphinv1alpha1.DefaultProductName, Namespace: testNamespace,
		}}
		roleBinding := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{
			Name: dolphinv1alpha1.DefaultProductName, Namespace: testNamespace,
		}}
		for _, object := range []metav1.Object{service, serviceAccount, role, roleBinding} {
			Expect(controllerutil.SetControllerReference(cr, object, scheme)).To(Succeed())
		}

		err := extension.ensureNoLegacyWorkloads(
			ctx, newFakeClient(service, serviceAccount, role, roleBinding), cr)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("delete service " + service.Name))
		Expect(err.Error()).To(ContainSubstring("delete serviceaccount dolphinscheduler"))
		Expect(err.Error()).To(ContainSubstring("delete role dolphinscheduler"))
		Expect(err.Error()).To(ContainSubstring("delete rolebinding dolphinscheduler"))
		Expect(err.Error()).To(ContainSubstring("listed legacy resource is gone"))
	})

	It("ignores Gen3 headless and metrics Services", func() {
		services := []*corev1.Service{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: testClusterName + "-master-default-headless", Namespace: testNamespace,
				},
				Spec: corev1.ServiceSpec{ClusterIP: corev1.ClusterIPNone},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: testClusterName + "-master-default-metrics", Namespace: testNamespace,
				},
				Spec: corev1.ServiceSpec{ClusterIP: corev1.ClusterIPNone},
			},
		}
		for _, service := range services {
			Expect(controllerutil.SetControllerReference(cr, service, scheme)).To(Succeed())
		}

		Expect(extension.ensureNoLegacyWorkloads(ctx, newFakeClient(services[0], services[1]), cr)).To(Succeed())
	})

	It("ignores legacy-shaped workloads controlled by another CR UID", func() {
		foreignOwner := cr.DeepCopy()
		foreignOwner.UID = types.UID("foreign-cluster-uid")
		statefulSet := newOwnedLegacyStatefulSet(
			foreignOwner, scheme, dolphinv1alpha1.RoleMaster, defaultRoleGroupName)
		deployment := newOwnedLegacyDeployment(foreignOwner, scheme, dolphinv1alpha1.RoleApi)

		Expect(extension.ensureNoLegacyWorkloads(ctx, newFakeClient(statefulSet, deployment), cr)).To(Succeed())
	})

	It("ignores a controller-owned Gen3 StatefulSet", func() {
		name := testClusterName + "-" + dolphinv1alpha1.RoleMaster + "-" + defaultRoleGroupName
		statefulSet := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
			Spec:       appsv1.StatefulSetSpec{ServiceName: name + "-headless"},
		}
		Expect(controllerutil.SetControllerReference(cr, statefulSet, scheme)).To(Succeed())

		Expect(extension.ensureNoLegacyWorkloads(ctx, newFakeClient(statefulSet), cr)).To(Succeed())
	})

	It("passes when no legacy workload exists", func() {
		Expect(extension.ensureNoLegacyWorkloads(ctx, newFakeClient(), cr)).To(Succeed())
	})
})

var _ = Describe("ClusterExtension DB init job", func() {
	It("stores legacy inline credentials in an owned Secret instead of the env ConfigMap", func() {
		cr := newTestCluster()
		cr.UID = types.UID("cluster-uid")
		cr.Spec.ClusterConfig.Database.CredentialsSecret = ""
		cr.Spec.ClusterConfig.Database.ConnectionString =
			"jdbc:postgresql://postgresql:5432/dolphinscheduler?user=db-user&password=p%40ss%3Dword"
		scheme := runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(dolphinv1alpha1.AddToScheme(scheme)).To(Succeed())
		extension := NewClusterExtension(scheme)
		c := newFakeClient()

		Expect(extension.ensureDatabaseCredentialsSecret(context.Background(), c, cr)).To(Succeed())
		secret := &corev1.Secret{}
		Expect(c.Get(context.Background(), types.NamespacedName{
			Namespace: testNamespace,
			Name:      testClusterName + databaseCredentialsSecretSuffix,
		}, secret)).To(Succeed())
		Expect(secret.Data).To(Equal(map[string][]byte{
			util.ConnectionStringAuthUser: []byte("db-user"),
			util.ConnectionStringAuthPass: []byte("p@ss=word"),
		}))
		Expect(metav1.IsControlledBy(secret, cr)).To(BeTrue())

		Expect(extension.ensureEnvsConfigMap(context.Background(), c, cr)).To(Succeed())
		configMap := &corev1.ConfigMap{}
		Expect(c.Get(context.Background(), types.NamespacedName{
			Namespace: testNamespace,
			Name:      envsConfigMapName(testClusterName),
		}, configMap)).To(Succeed())
		Expect(configMap.Data).NotTo(HaveKey("SPRING_DATASOURCE_USERNAME"))
		Expect(configMap.Data).NotTo(HaveKey("SPRING_DATASOURCE_PASSWORD"))
	})

	It("does not overwrite a foreign inline-credentials Secret", func() {
		cr := newTestCluster()
		cr.UID = types.UID("cluster-uid")
		cr.Spec.ClusterConfig.Database.CredentialsSecret = ""
		cr.Spec.ClusterConfig.Database.ConnectionString =
			"jdbc:postgresql://postgresql:5432/dolphinscheduler?user=db-user&password=db-password"
		foreign := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Name:      databaseCredentialsSecretName(cr),
			Namespace: testNamespace,
		}}
		extension := NewClusterExtension(clientgoscheme.Scheme)

		err := extension.ensureDatabaseCredentialsSecret(context.Background(), newFakeClient(foreign), cr)
		Expect(err).To(MatchError(ContainSubstring("refusing to overwrite database credentials Secret")))
	})

	It("keeps an IPv6 database host bracketed in the datasource URL", func() {
		cr := newTestCluster()
		cr.UID = types.UID("cluster-uid")
		cr.Spec.ClusterConfig.Database.ConnectionString =
			"jdbc:postgresql://[2001:db8::1]:5433/dolphinscheduler"
		scheme := runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(dolphinv1alpha1.AddToScheme(scheme)).To(Succeed())
		extension := NewClusterExtension(scheme)
		c := newFakeClient(testDBCredentialsSecret(cr))

		Expect(extension.ensureEnvsConfigMap(context.Background(), c, cr)).To(Succeed())
		configMap := &corev1.ConfigMap{}
		Expect(c.Get(context.Background(), types.NamespacedName{
			Namespace: cr.Namespace,
			Name:      envsConfigMapName(cr.Name),
		}, configMap)).To(Succeed())
		Expect(configMap.Data).To(HaveKeyWithValue(
			"SPRING_DATASOURCE_URL", "jdbc:postgresql://[2001:db8::1]:5433/dolphinscheduler"))
	})

	It("waits for the database port from the JDBC connection string", func() {
		cr := newTestCluster()
		cr.Spec.ClusterConfig.Database.ConnectionString =
			"jdbc:postgresql://postgresql.example.com:15432/dolphinscheduler"
		extension := NewClusterExtension(clientgoscheme.Scheme)

		job, err := extension.buildDBInitJob(cr)
		Expect(err).NotTo(HaveOccurred())
		Expect(job.Spec.Template.Spec.InitContainers).To(HaveLen(1))
		Expect(job.Spec.Template.Spec.InitContainers[0].Command).To(Equal([]string{
			"sh",
			"-ec",
			`for i in $(seq 1 180); do nc -z -w3 "$1" "$2" && exit 0 || sleep 5; done; exit 1`,
			"wait-for-database",
			"postgresql.example.com",
			"15432",
		}))
		Expect(job.Spec.Template.Spec.Containers[0].Env).To(Equal(databaseCredentialEnv(cr)))
		Expect(job.Annotations).To(HaveKeyWithValue(dbInitImageAnnotation, job.Spec.Template.Spec.Containers[0].Image))
		Expect(job.Annotations).To(HaveKeyWithValue(dbInitTargetAnnotation, Not(BeEmpty())))
		Expect(job.Spec.Template.Spec.AutomountServiceAccountToken).To(HaveValue(BeFalse()))
	})

	It("inherits the product image pull Secret", func() {
		cr := newTestCluster()
		cr.Spec.Image.PullSecretName = "private-registry"
		extension := NewClusterExtension(clientgoscheme.Scheme)

		job, err := extension.buildDBInitJob(cr)
		Expect(err).NotTo(HaveOccurred())
		Expect(job.Spec.Template.Spec.ImagePullSecrets).To(Equal([]corev1.LocalObjectReference{{
			Name: "private-registry",
		}}))
	})

	It("creates a DB init Job with complete identity annotations", func() {
		cr := newTestCluster()
		cr.UID = types.UID("cluster-uid")
		scheme := runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(dolphinv1alpha1.AddToScheme(scheme)).To(Succeed())
		c := newDBInitFakeClient(cr)
		extension := NewClusterExtension(scheme)

		err := extension.ensureDBInitJob(context.Background(), c, cr)
		Expect(err).To(MatchError(ContainSubstring(dbInitWaitMessage)))
		job := &batchv1.Job{}
		Expect(c.Get(context.Background(), types.NamespacedName{
			Namespace: cr.Namespace, Name: cr.Name,
		}, job)).To(Succeed())
		Expect(job.Annotations).To(HaveKeyWithValue(dbInitImageAnnotation, Not(BeEmpty())))
		Expect(job.Annotations).To(HaveKeyWithValue(dbInitTargetAnnotation, Not(BeEmpty())))
		Expect(job.Annotations).To(HaveKeyWithValue(dbInitCredentialsAnnotation, Not(BeEmpty())))
	})

	It("rejects a same-named Job not controlled by the cluster", func() {
		cr := newTestCluster()
		cr.UID = types.UID("cluster-uid")
		job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: cr.Name, Namespace: cr.Namespace}}
		job.Status.Succeeded = 1
		extension := NewClusterExtension(clientgoscheme.Scheme)

		err := extension.ensureDBInitJob(context.Background(), newFakeClient(job), cr)
		Expect(err).To(MatchError(ContainSubstring("not controlled by DolphinschedulerCluster")))
	})

	It("recreates a completed DB init Job when the product image changes", func() {
		cr := newTestCluster()
		cr.UID = types.UID("cluster-uid")
		scheme := runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(dolphinv1alpha1.AddToScheme(scheme)).To(Succeed())
		annotations := desiredDBAnnotations(cr)
		annotations[dbInitImageAnnotation] = "old.example/dolphinscheduler:old"
		job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
			Name: cr.Name, Namespace: cr.Namespace,
			Annotations: annotations,
		}}
		Expect(controllerutil.SetControllerReference(cr, job, scheme)).To(Succeed())
		job.Status.Succeeded = 1
		c := newDBInitFakeClient(cr, job)
		extension := NewClusterExtension(scheme)

		err := extension.ensureDBInitJob(context.Background(), c, cr)
		Expect(err).To(MatchError(ContainSubstring(dbInitRecreateMessage)))
		remaining := &batchv1.Job{}
		Expect(c.Get(context.Background(), types.NamespacedName{
			Namespace: cr.Namespace,
			Name:      cr.Name,
		}, remaining)).NotTo(Succeed())
	})

	It("recreates a successful legacy DB init Job whose database target cannot be proven", func() {
		cr := newTestCluster()
		cr.UID = types.UID("cluster-uid")
		scheme := runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(dolphinv1alpha1.AddToScheme(scheme)).To(Succeed())
		desiredImage, _, err := resolveProductImage(cr)
		Expect(err).NotTo(HaveOccurred())
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name: cr.Name, Namespace: cr.Namespace, UID: types.UID("legacy-job-uid"),
			},
			Spec: batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: dbInitJobContainerName, Image: desiredImage}},
			}}},
		}
		Expect(controllerutil.SetControllerReference(cr, job, scheme)).To(Succeed())
		job.Status.Succeeded = 1
		c := newDBInitFakeClient(cr, job)
		extension := NewClusterExtension(scheme)

		err = extension.ensureDBInitJob(context.Background(), c, cr)
		Expect(err).To(MatchError(ContainSubstring(dbInitRecreateMessage)))
		remaining := &batchv1.Job{}
		Expect(c.Get(context.Background(), types.NamespacedName{
			Namespace: cr.Namespace,
			Name:      cr.Name,
		}, remaining)).NotTo(Succeed())
	})

	It("recreates a successful DB init Job when only the database target changes", func() {
		cr := newTestCluster()
		cr.UID = types.UID("cluster-uid")
		scheme := runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(dolphinv1alpha1.AddToScheme(scheme)).To(Succeed())
		job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
			Name: cr.Name, Namespace: cr.Namespace,
			Annotations: desiredDBAnnotations(cr),
		}}
		Expect(controllerutil.SetControllerReference(cr, job, scheme)).To(Succeed())
		job.Status.Succeeded = 1
		cr.Spec.ClusterConfig.Database.ConnectionString =
			"jdbc:postgresql://postgresql:5432/another-database"
		c := newDBInitFakeClient(cr, job)
		extension := NewClusterExtension(scheme)

		err := extension.ensureDBInitJob(context.Background(), c, cr)
		Expect(err).To(MatchError(ContainSubstring(dbInitRecreateMessage)))
		remaining := &batchv1.Job{}
		Expect(c.Get(context.Background(), types.NamespacedName{
			Namespace: cr.Namespace,
			Name:      cr.Name,
		}, remaining)).NotTo(Succeed())
	})

	It("recreates a failed DB init Job after its desired identity changes", func() {
		cr := newTestCluster()
		cr.UID = types.UID("cluster-uid")
		scheme := runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(dolphinv1alpha1.AddToScheme(scheme)).To(Succeed())
		annotations := desiredDBAnnotations(cr)
		annotations[dbInitImageAnnotation] = "old.example/dolphinscheduler:old"
		job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
			Name: cr.Name, Namespace: cr.Namespace, Annotations: annotations,
		}}
		Expect(controllerutil.SetControllerReference(cr, job, scheme)).To(Succeed())
		job.Status.Conditions = []batchv1.JobCondition{{
			Type: batchv1.JobFailed, Status: corev1.ConditionTrue,
		}}
		c := newDBInitFakeClient(cr, job)
		extension := NewClusterExtension(scheme)

		err := extension.ensureDBInitJob(context.Background(), c, cr)
		Expect(err).To(MatchError(ContainSubstring(dbInitRecreateMessage)))
	})

	It("recreates an active DB init Job after its credentials Secret rotates", func() {
		cr := newTestCluster()
		cr.UID = types.UID("cluster-uid")
		scheme := runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(dolphinv1alpha1.AddToScheme(scheme)).To(Succeed())
		job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
			Name: cr.Name, Namespace: cr.Namespace, Annotations: desiredDBAnnotations(cr),
		}}
		Expect(controllerutil.SetControllerReference(cr, job, scheme)).To(Succeed())
		c := newDBInitFakeClient(cr, job)
		secret := &corev1.Secret{}
		secretKey := types.NamespacedName{
			Namespace: cr.Namespace, Name: databaseCredentialsSecretName(cr),
		}
		Expect(c.Get(context.Background(), secretKey, secret)).To(Succeed())
		secret.Data[util.ConnectionStringAuthPass] = []byte("rotated-password")
		Expect(c.Update(context.Background(), secret)).To(Succeed())
		extension := NewClusterExtension(scheme)

		err := extension.ensureDBInitJob(context.Background(), c, cr)
		Expect(err).To(MatchError(ContainSubstring(dbInitRecreateMessage)))
	})

	It("reports a terminal DB init failure instead of waiting forever", func() {
		cr := newTestCluster()
		cr.UID = types.UID("cluster-uid")
		job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
			Name: cr.Name, Namespace: cr.Namespace, Annotations: desiredDBAnnotations(cr),
		}}
		scheme := runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(dolphinv1alpha1.AddToScheme(scheme)).To(Succeed())
		Expect(controllerutil.SetControllerReference(cr, job, scheme)).To(Succeed())
		job.Status.Conditions = []batchv1.JobCondition{{
			Type:    batchv1.JobFailed,
			Status:  corev1.ConditionTrue,
			Reason:  "BackoffLimitExceeded",
			Message: "container failed",
		}}
		extension := NewClusterExtension(scheme)

		err := extension.ensureDBInitJob(context.Background(), newDBInitFakeClient(cr, job), cr)
		Expect(err).To(MatchError(ContainSubstring("BackoffLimitExceeded: container failed")))
	})

	It("rejects a malformed JDBC connection string", func() {
		cr := newTestCluster()
		cr.Spec.ClusterConfig.Database.ConnectionString = "not-a-jdbc-url"
		extension := NewClusterExtension(clientgoscheme.Scheme)

		_, err := extension.buildDBInitJob(cr)
		Expect(err).To(MatchError(ContainSubstring("failed to resolve database endpoint")))
	})
})
