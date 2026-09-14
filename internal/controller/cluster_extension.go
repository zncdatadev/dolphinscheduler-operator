package controller

import (
	"context"
	"crypto/sha256"
	"fmt"
	"maps"
	"net"
	"slices"
	"strings"
	"time"

	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/util"
	opcommon "github.com/zncdatadev/operator-go/pkg/common"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// +kubebuilder:rbac:groups=dolphinscheduler.kubedoop.dev,resources=dolphinschedulerclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dolphinscheduler.kubedoop.dev,resources=dolphinschedulerclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dolphinscheduler.kubedoop.dev,resources=dolphinschedulerclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=secrets.kubedoop.dev,resources=secretclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups=s3.kubedoop.dev,resources=s3connections,verbs=get;list;watch
// +kubebuilder:rbac:groups=authentication.kubedoop.dev,resources=authenticationclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete

const (
	// dbInitJobContainerName / dbInitWaitContainerName are the DB-init Job container names.
	dbInitJobContainerName  = "dolphinscheduler-db-init-job"
	dbInitWaitContainerName = "wait-for-database"

	// dbInitWaitImage is the init container image probing the database port.
	dbInitWaitImage = "busybox:1.30.1"

	// dbInitWaitReason / dbInitWaitMessage are the Waiting condition of the DB-init gate. The
	// message is deliberately CONSTANT across passes: the status write is DeepEqual-guarded and
	// this path returns no error, so a varying message would rewrite the CR forever. The
	// namespace/name go into a log line instead.
	dbInitWaitReason            = "WaitingForDBInit"
	dbInitWaitMessage           = "the database schema initialization Job has not completed"
	dbInitRecreateReason        = "RecreatingDBInit"
	dbInitRecreateMessage       = "the database schema initialization Job used a different product image, database target, or active credential version and is being recreated"
	dbInitImageAnnotation       = "dolphinscheduler.kubedoop.dev/db-init-image"
	dbInitTargetAnnotation      = "dolphinscheduler.kubedoop.dev/db-init-target-hash"
	dbInitCredentialsAnnotation = "dolphinscheduler.kubedoop.dev/db-init-credentials-version-hash"

	// dbInitRequeueAfter is how long to wait between DB-init Job checks.
	dbInitRequeueAfter = 15 * time.Second

	// databaseCredentialsSecretSuffix is used only for backwards-compatible JDBC URLs that
	// still carry inline credentials. New installations should point credentialsSecret at a
	// user-managed Secret instead.
	databaseCredentialsSecretSuffix = "-database-credentials"
)

// ClusterExtension provisions the cluster-scoped resources the role groups depend on: the
// "<cluster>-envs" ConfigMap and the create-once DB schema init Job. PreReconcile returns a
// typed wait until the Job has succeeded, so the roles wait for the schema — the legacy
// ordering. It also guards the breaking Gen2b-to-Gen3 workload migration: legacy controller-owned
// master/worker StatefulSets, api/alert Deployments, incompatible headless client Services, and
// fixed-name RBAC fail the cluster with an explicit runbook error before any role-group resource
// is applied.
type ClusterExtension struct {
	scheme *runtime.Scheme
}

var _ opcommon.ClusterExtension[*dolphinv1alpha1.DolphinschedulerCluster] = &ClusterExtension{}

// NewClusterExtension creates the cluster extension.
func NewClusterExtension(scheme *runtime.Scheme) *ClusterExtension {
	return &ClusterExtension{scheme: scheme}
}

// Name implements opcommon.Extension.
func (e *ClusterExtension) Name() string { return "dolphinscheduler-cluster" }

// PreReconcile implements opcommon.ClusterExtension.
func (e *ClusterExtension) PreReconcile(ctx context.Context, c ctrlclient.Client, cr *dolphinv1alpha1.DolphinschedulerCluster) error {
	if err := e.ensureNoLegacyWorkloads(ctx, c, cr); err != nil {
		return err
	}
	if err := e.ensureDatabaseCredentialsSecret(ctx, c, cr); err != nil {
		return err
	}
	if err := e.ensureEnvsConfigMap(ctx, c, cr); err != nil {
		return err
	}
	return e.ensureDBInitJob(ctx, c, cr)
}

// PostReconcile implements opcommon.ClusterExtension.
func (e *ClusterExtension) PostReconcile(_ context.Context, _ ctrlclient.Client, _ *dolphinv1alpha1.DolphinschedulerCluster) error {
	return nil
}

// OnReconcileError implements opcommon.ClusterExtension.
func (e *ClusterExtension) OnReconcileError(_ context.Context, _ ctrlclient.Client, _ *dolphinv1alpha1.DolphinschedulerCluster, _ error) error {
	return nil
}

func databaseCredentialsSecretName(cr *dolphinv1alpha1.DolphinschedulerCluster) string {
	if cr.Spec.ClusterConfig != nil && cr.Spec.ClusterConfig.Database != nil &&
		cr.Spec.ClusterConfig.Database.CredentialsSecret != "" {
		return cr.Spec.ClusterConfig.Database.CredentialsSecret
	}
	return cr.Name + databaseCredentialsSecretSuffix
}

func databaseCredentialEnv(cr *dolphinv1alpha1.DolphinschedulerCluster) []corev1.EnvVar {
	if cr.Spec.ClusterConfig == nil || cr.Spec.ClusterConfig.Database == nil {
		return nil
	}
	secretName := databaseCredentialsSecretName(cr)
	return []corev1.EnvVar{
		{
			Name: "SPRING_DATASOURCE_USERNAME",
			ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  util.ConnectionStringAuthUser,
			}},
		},
		{
			Name: "SPRING_DATASOURCE_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  util.ConnectionStringAuthPass,
			}},
		},
	}
}

// ensureDatabaseCredentialsSecret preserves the legacy inline-JDBC API without copying its
// credentials into a ConfigMap or Pod template. If credentialsSecret is set, that user-managed
// Secret is used directly and validated by ensureEnvsConfigMap.
func (e *ClusterExtension) ensureDatabaseCredentialsSecret(
	ctx context.Context,
	c ctrlclient.Client,
	cr *dolphinv1alpha1.DolphinschedulerCluster,
) error {
	if cr.Spec.ClusterConfig == nil || cr.Spec.ClusterConfig.Database == nil ||
		cr.Spec.ClusterConfig.Database.CredentialsSecret != "" {
		return nil
	}

	dbSpec := cr.Spec.ClusterConfig.Database
	dbConfig, err := util.NewDataBaseExtractor(c, &dbSpec.ConnectionString).ExtractDatabaseInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to extract inline database credentials: %w", err)
	}

	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name:      databaseCredentialsSecretName(cr),
		Namespace: cr.Namespace,
	}}
	if _, err := controllerutil.CreateOrUpdate(ctx, c, secret, func() error {
		if secret.ResourceVersion != "" && !metav1.IsControlledBy(secret, cr) {
			return fmt.Errorf(
				"refusing to overwrite database credentials Secret %s/%s because it is not controlled by DolphinschedulerCluster %s",
				cr.Namespace, secret.Name, cr.Name)
		}
		secret.Labels = legacyClusterLabels(cr.Name)
		secret.Type = corev1.SecretTypeOpaque
		secret.Data = map[string][]byte{
			util.ConnectionStringAuthUser: []byte(dbConfig.Username),
			util.ConnectionStringAuthPass: []byte(dbConfig.Password),
		}
		return controllerutil.SetControllerReference(cr, secret, e.scheme)
	}); err != nil {
		return fmt.Errorf("failed to ensure database credentials Secret %s/%s: %w", cr.Namespace, secret.Name, err)
	}
	return nil
}

// ensureNoLegacyWorkloads is the breaking Gen2b-to-Gen3 upgrade guard. The old operator rendered
// master/worker as StatefulSets whose immutable serviceName was the workload name; Gen3 uses
// "<name>-headless" and a different immutable selector. Letting the framework preserve those old
// values would leave an upgraded cluster materially different from a fresh one. The old api/alert
// Deployments are worse: their same-named Gen3 StatefulSets can coexist and double Service
// endpoints. Old headless client Services have immutable ClusterIP shape, and fixed-name RBAC
// would remain over-privileged debris. Detect every affected resource controller-owned by this
// CR, including stale groups no longer declared in the spec, and stop before any role group is
// applied. Deletion stays an explicit operator action because workload deletion causes downtime.
func (e *ClusterExtension) ensureNoLegacyWorkloads(ctx context.Context, c ctrlclient.Client, cr *dolphinv1alpha1.DolphinschedulerCluster) error {
	legacy, err := findLegacyResources(ctx, c, cr)
	if err != nil {
		return err
	}
	if legacy.empty() {
		return nil
	}

	legacy.sort()
	leftovers, commands := legacy.migrationInstructions(cr.Namespace)

	return fmt.Errorf(
		"legacy Gen2b resources block the Gen3 migration: %s. "+
			"Gen3 changes immutable StatefulSet selectors/serviceName and renders api/alert as StatefulSets; "+
			"the operator will not delete resources automatically because this breaking upgrade causes downtime. "+
			"After scheduling the outage, run %s. "+
			"These commands intentionally do not delete "+
			"PersistentVolumeClaims, so worker data PVCs are retained. Reconciliation resumes after every "+
			"listed legacy resource is gone",
		strings.Join(leftovers, "; "), strings.Join(commands, " and then "))
}

type legacyResources struct {
	statefulSets    []string
	deployments     []string
	services        []string
	serviceAccounts []string
	roles           []string
	roleBindings    []string
}

func findLegacyResources(
	ctx context.Context,
	c ctrlclient.Client,
	cr *dolphinv1alpha1.DolphinschedulerCluster,
) (*legacyResources, error) {
	legacy := &legacyResources{}
	statefulSets := &appsv1.StatefulSetList{}
	if err := c.List(ctx, statefulSets, ctrlclient.InNamespace(cr.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to list StatefulSets while checking for legacy workloads in namespace %s: %w", cr.Namespace, err)
	}

	for i := range statefulSets.Items {
		statefulSet := &statefulSets.Items[i]
		if metav1.IsControlledBy(statefulSet, cr) && statefulSet.Spec.ServiceName == statefulSet.Name {
			legacy.statefulSets = append(legacy.statefulSets, statefulSet.Name)
		}
	}

	deployments := &appsv1.DeploymentList{}
	if err := c.List(ctx, deployments, ctrlclient.InNamespace(cr.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to list Deployments while checking for legacy workloads in namespace %s: %w", cr.Namespace, err)
	}

	for i := range deployments.Items {
		deployment := &deployments.Items[i]
		// Gen3 renders every role as a StatefulSet, so every Deployment controlled by this CR is
		// a legacy workload. A same-named Deployment owned by any other object is not ours.
		if metav1.IsControlledBy(deployment, cr) {
			legacy.deployments = append(legacy.deployments, deployment.Name)
		}
	}

	services := &corev1.ServiceList{}
	if err := c.List(ctx, services, ctrlclient.InNamespace(cr.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to list Services while checking for legacy resources in namespace %s: %w", cr.Namespace, err)
	}
	for i := range services.Items {
		service := &services.Items[i]
		// Gen2b used the client Service itself as master/worker's StatefulSet governing
		// headless Service. Gen3 needs that name for a normal client Service and creates a
		// separate "-headless" Service, so ClusterIP=None cannot be updated in place.
		if metav1.IsControlledBy(service, cr) && service.Spec.ClusterIP == corev1.ClusterIPNone &&
			!strings.HasSuffix(service.Name, "-headless") && !strings.HasSuffix(service.Name, "-metrics") {
			legacy.services = append(legacy.services, service.Name)
		}
	}

	serviceAccounts := &corev1.ServiceAccountList{}
	if err := c.List(ctx, serviceAccounts, ctrlclient.InNamespace(cr.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to list ServiceAccounts while checking for legacy resources in namespace %s: %w", cr.Namespace, err)
	}
	for i := range serviceAccounts.Items {
		serviceAccount := &serviceAccounts.Items[i]
		if serviceAccount.Name == dolphinv1alpha1.DefaultProductName && metav1.IsControlledBy(serviceAccount, cr) {
			legacy.serviceAccounts = append(legacy.serviceAccounts, serviceAccount.Name)
		}
	}

	roles := &rbacv1.RoleList{}
	if err := c.List(ctx, roles, ctrlclient.InNamespace(cr.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to list Roles while checking for legacy resources in namespace %s: %w", cr.Namespace, err)
	}
	for i := range roles.Items {
		role := &roles.Items[i]
		if role.Name == dolphinv1alpha1.DefaultProductName && metav1.IsControlledBy(role, cr) {
			legacy.roles = append(legacy.roles, role.Name)
		}
	}

	roleBindings := &rbacv1.RoleBindingList{}
	if err := c.List(ctx, roleBindings, ctrlclient.InNamespace(cr.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to list RoleBindings while checking for legacy resources in namespace %s: %w", cr.Namespace, err)
	}
	for i := range roleBindings.Items {
		roleBinding := &roleBindings.Items[i]
		if roleBinding.Name == dolphinv1alpha1.DefaultProductName && metav1.IsControlledBy(roleBinding, cr) {
			legacy.roleBindings = append(legacy.roleBindings, roleBinding.Name)
		}
	}

	return legacy, nil
}

func (r legacyResources) empty() bool {
	return len(r.statefulSets) == 0 && len(r.deployments) == 0 && len(r.services) == 0 &&
		len(r.serviceAccounts) == 0 && len(r.roles) == 0 && len(r.roleBindings) == 0
}

func (r *legacyResources) sort() {
	slices.Sort(r.statefulSets)
	slices.Sort(r.deployments)
	slices.Sort(r.services)
	slices.Sort(r.serviceAccounts)
	slices.Sort(r.roles)
	slices.Sort(r.roleBindings)
}

func (r legacyResources) migrationInstructions(namespace string) ([]string, []string) {
	var leftovers []string
	var commands []string
	appendLegacyDeletion(&leftovers, &commands, "StatefulSet", "statefulset", r.statefulSets, namespace,
		"--cascade=foreground --wait=true")
	appendLegacyDeletion(&leftovers, &commands, "Deployment", "deployment", r.deployments, namespace,
		"--cascade=foreground --wait=true")
	appendLegacyDeletion(&leftovers, &commands, "Service", "service", r.services, namespace, "--wait=true")
	appendLegacyDeletion(&leftovers, &commands, "ServiceAccount", "serviceaccount", r.serviceAccounts, namespace,
		"--wait=true")
	appendLegacyDeletion(&leftovers, &commands, "Role", "role", r.roles, namespace, "--wait=true")
	appendLegacyDeletion(&leftovers, &commands, "RoleBinding", "rolebinding", r.roleBindings, namespace, "--wait=true")
	return leftovers, commands
}

func appendLegacyDeletion(
	leftovers, commands *[]string,
	kind, resource string,
	names []string,
	namespace, flags string,
) {
	if len(names) == 0 {
		return
	}
	*leftovers = append(*leftovers, kind+"(s) "+strings.Join(names, ", "))
	*commands = append(*commands, fmt.Sprintf("`kubectl -n %s delete %s %s %s`",
		namespace, resource, strings.Join(names, " "), flags))
}

// ensureEnvsConfigMap applies the "<cluster>-envs" ConfigMap every container (and the DB-init
// Job) envFroms: the master role's envOverrides merged with the product defaults (defaults win
// — the legacy precedence, kept byte for byte) plus the database-derived Spring datasource env.
func (e *ClusterExtension) ensureEnvsConfigMap(ctx context.Context, c ctrlclient.Client, cr *dolphinv1alpha1.DolphinschedulerCluster) error {
	data := map[string]string{}

	// The legacy operator built this ConfigMap from the MASTER role's env overrides only
	// (role-level, then role-group-level). Groups are folded in sorted name order so the
	// result is deterministic where the legacy map iteration was not.
	lastGroup := defaultRoleGroupName
	if master := cr.Spec.Master; master != nil {
		if master.OverridesSpec != nil {
			maps.Copy(data, master.EnvOverrides)
		}
		for _, groupName := range slices.Sorted(maps.Keys(master.RoleGroups)) {
			if o := master.RoleGroups[groupName].OverridesSpec; o != nil {
				maps.Copy(data, o.EnvOverrides)
			}
			lastGroup = groupName
		}
	}

	// Product defaults deliberately WIN over user env overrides here (legacy behavior).
	maps.Copy(data, defaultEnvOverrides())

	if clusterConfig := cr.Spec.ClusterConfig; clusterConfig != nil && clusterConfig.Database != nil {
		dbSpec := clusterConfig.Database
		extractor := util.NewDataBaseExtractor(c, &dbSpec.ConnectionString)
		if dbSpec.CredentialsSecret != "" {
			extractor = extractor.CredentialsInSecret(dbSpec.CredentialsSecret, cr.Namespace)
		}
		dbConfig, err := extractor.ExtractDatabaseInfo(ctx)
		if err != nil {
			return fmt.Errorf("failed to extract database info: %w", err)
		}
		maps.Copy(data, map[string]string{
			"DATABASE":                            dbConfig.DbType,
			"SPRING_DATASOURCE_URL":               fmt.Sprintf("jdbc:%s://%s/%s", dbConfig.DbType, net.JoinHostPort(dbConfig.Host, dbConfig.Port), dbConfig.DbName),
			"SPRING_DATASOURCE_DRIVER-CLASS-NAME": dbConfig.Driver,
		})
	}

	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name:      envsConfigMapName(cr.Name),
		Namespace: cr.Namespace,
	}}
	if _, err := controllerutil.CreateOrUpdate(ctx, c, cm, func() error {
		cm.Labels = legacyRoleGroupLabels(cr.Name, dolphinv1alpha1.RoleMaster, lastGroup)
		cm.Data = data
		return controllerutil.SetControllerReference(cr, cm, e.scheme)
	}); err != nil {
		return fmt.Errorf("failed to ensure env configmap %s/%s: %w", cr.Namespace, envsConfigMapName(cr.Name), err)
	}

	return nil
}

// ensureDBInitJob creates the DB schema init Job (name = the cluster name) and returns a typed
// wait until it has succeeded, so the role groups are not reconciled before the schema exists
// (Waiting condition, no Degraded, no Warning event). A Job's spec is immutable: an image,
// database-target, or active credential-version identity change deletes it for foreground
// recreation, and a concurrent create is tolerated.
func (e *ClusterExtension) ensureDBInitJob(ctx context.Context, c ctrlclient.Client, cr *dolphinv1alpha1.DolphinschedulerCluster) error {
	clusterConfig := cr.Spec.ClusterConfig
	if clusterConfig == nil || clusterConfig.Database == nil {
		log.FromContext(ctx).Info("no database configured; skipping DB init job")
		return nil
	}

	existing := &batchv1.Job{}
	err := c.Get(ctx, types.NamespacedName{Namespace: cr.Namespace, Name: cr.Name}, existing)
	switch {
	case apierrors.IsNotFound(err):
		job, buildErr := e.buildDBInitJob(cr)
		if buildErr != nil {
			return buildErr
		}
		credentialsVersion, versionErr := databaseCredentialsVersionHash(ctx, c, cr)
		if versionErr != nil {
			return versionErr
		}
		job.Annotations[dbInitCredentialsAnnotation] = credentialsVersion
		if err := controllerutil.SetControllerReference(cr, job, e.scheme); err != nil {
			return err
		}
		if err := c.Create(ctx, job); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create DB init job %s/%s: %w", cr.Namespace, cr.Name, err)
		}
		log.FromContext(ctx).Info("waiting for DB init job to complete", "namespace", cr.Namespace, "job", cr.Name)
		return opcommon.NewRequeueAfterError(dbInitRequeueAfter, dbInitWaitReason, dbInitWaitMessage)
	case err != nil:
		return fmt.Errorf("failed to get DB init job %s/%s: %w", cr.Namespace, cr.Name, err)
	}
	if !metav1.IsControlledBy(existing, cr) {
		return fmt.Errorf(
			"refusing to use DB init Job %s/%s because it is not controlled by DolphinschedulerCluster %s",
			cr.Namespace, cr.Name, cr.Name)
	}

	desiredImage, desiredTarget, err := desiredDBInitIdentity(cr)
	if err != nil {
		return err
	}
	credentialsVersion, err := databaseCredentialsVersionHash(ctx, c, cr)
	if err != nil {
		return err
	}
	staticIdentityMatches := dbInitIdentityMatches(existing, desiredImage, desiredTarget)
	credentialsMatch := existing.Annotations[dbInitCredentialsAnnotation] == credentialsVersion
	if !staticIdentityMatches || (existing.Status.Succeeded == 0 && !credentialsMatch) {
		return replaceDBInitJob(ctx, c, cr, existing)
	}
	if existing.Status.Succeeded > 0 {
		return nil
	}
	for _, condition := range existing.Status.Conditions {
		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			return fmt.Errorf("DB init Job %s/%s failed: %s: %s", cr.Namespace, cr.Name, condition.Reason, condition.Message)
		}
	}
	log.FromContext(ctx).Info("waiting for DB init job to complete", "namespace", cr.Namespace, "job", cr.Name)
	return opcommon.NewRequeueAfterError(dbInitRequeueAfter, dbInitWaitReason, dbInitWaitMessage)
}

func desiredDBInitIdentity(cr *dolphinv1alpha1.DolphinschedulerCluster) (string, string, error) {
	desiredImage, _, err := resolveProductImage(cr)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve desired DB init image: %w", err)
	}
	canonicalTarget, err := util.CanonicalDatabaseTarget(cr.Spec.ClusterConfig.Database.ConnectionString)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve desired DB init target: %w", err)
	}
	targetHash := sha256.Sum256([]byte(canonicalTarget))
	return desiredImage, fmt.Sprintf("%x", targetHash), nil
}

func dbInitIdentityMatches(job *batchv1.Job, desiredImage, desiredTarget string) bool {
	return job.Annotations[dbInitImageAnnotation] == desiredImage &&
		job.Annotations[dbInitTargetAnnotation] == desiredTarget
}

func databaseCredentialsVersionHash(
	ctx context.Context,
	c ctrlclient.Client,
	cr *dolphinv1alpha1.DolphinschedulerCluster,
) (string, error) {
	secretName := databaseCredentialsSecretName(cr)
	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: cr.Namespace, Name: secretName}
	if err := c.Get(ctx, key, secret); err != nil {
		return "", fmt.Errorf("failed to resolve DB init credentials Secret %s: %w", key, err)
	}
	if secret.ResourceVersion == "" {
		return "", fmt.Errorf("DB init credentials Secret %s has no resourceVersion", key)
	}
	digest := sha256.Sum256([]byte(key.String() + "\x00" + secret.ResourceVersion))
	return fmt.Sprintf("%x", digest), nil
}

func replaceDBInitJob(
	ctx context.Context,
	c ctrlclient.Client,
	cr *dolphinv1alpha1.DolphinschedulerCluster,
	existing *batchv1.Job,
) error {
	if err := c.Delete(ctx, existing, ctrlclient.PropagationPolicy(metav1.DeletePropagationForeground)); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to replace stale DB init Job %s/%s: %w", cr.Namespace, cr.Name, err)
	}
	return opcommon.NewRequeueAfterError(dbInitRequeueAfter, dbInitRecreateReason, dbInitRecreateMessage)
}

// buildDBInitJob renders the DB-init Job: a busybox init container waiting for the database
// endpoint and the product image running tools/bin/upgrade-schema.sh with the
// "<cluster>-envs" environment.
func (e *ClusterExtension) buildDBInitJob(cr *dolphinv1alpha1.DolphinschedulerCluster) (*batchv1.Job, error) {
	image, pullPolicy, err := resolveProductImage(cr)
	if err != nil {
		return nil, err
	}

	dbHost, dbPort, err := util.GetDatabaseHost(cr.Spec.ClusterConfig.Database.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve database endpoint: %w", err)
	}
	_, targetHash, err := desiredDBInitIdentity(cr)
	if err != nil {
		return nil, err
	}
	labels := legacyClusterLabels(cr.Name)
	pullSecretName := cr.GetSpec().Image.ResolvedPullSecretName(ProductImageDefaults())
	var imagePullSecrets []corev1.LocalObjectReference
	if pullSecretName != "" {
		imagePullSecrets = []corev1.LocalObjectReference{{Name: pullSecretName}}
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				dbInitImageAnnotation:  image,
				dbInitTargetAnnotation: targetHash,
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy:                corev1.RestartPolicyNever,
					ImagePullSecrets:             imagePullSecrets,
					AutomountServiceAccountToken: ptr.To(false),
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:    ptr.To[int64](1001),
						RunAsGroup:   ptr.To[int64](1001),
						RunAsNonRoot: ptr.To(false),
					},
					InitContainers: []corev1.Container{
						{
							Name:  dbInitWaitContainerName,
							Image: dbInitWaitImage,
							Command: []string{
								"sh",
								"-ec",
								`for i in $(seq 1 180); do nc -z -w3 "$1" "$2" && exit 0 || sleep 5; done; exit 1`,
								"wait-for-database",
								dbHost,
								dbPort,
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            dbInitJobContainerName,
							Image:           image,
							ImagePullPolicy: pullPolicy,
							Command:         slices.Clone(containerCommand),
							Args:            []string{"chmod +x tools/bin/upgrade-schema.sh && tools/bin/upgrade-schema.sh"},
							Env:             databaseCredentialEnv(cr),
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{Name: envsConfigMapName(cr.Name)},
									},
								},
							},
						},
					},
				},
			},
		},
	}, nil
}
