package controller

import (
	"context"
	"fmt"
	"maps"
	"path"
	"slices"
	"strconv"
	"strings"

	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdatadev/dolphinscheduler-operator/internal/security"
	"github.com/zncdatadev/dolphinscheduler-operator/internal/util/version"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/constant"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/util"
	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	opgoconstant "github.com/zncdatadev/operator-go/pkg/constant"
	"github.com/zncdatadev/operator-go/pkg/productlogging"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	opgos3 "github.com/zncdatadev/operator-go/pkg/s3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// mainContainerNames maps role names to the primary container name. Note the alert role's
// container is "alerter-server" (historical), while its server binary and config directory are
// "alert-server".
var mainContainerNames = map[string]string{
	dolphinv1alpha1.RoleMaster: "master-server",
	dolphinv1alpha1.RoleWorker: "worker-server",
	dolphinv1alpha1.RoleApi:    "api-server",
	dolphinv1alpha1.RoleAlert:  "alerter-server",
}

// legacyLogVolumeSize is the shared log emptyDir SizeLimit the pre-migration operator rendered
// (3 x the 10Mi max log file size).
const legacyLogVolumeSize = "30Mi"

const (
	// zkConnectStringEnvName is the ZooKeeper discovery env var every server reads.
	zkConnectStringEnvName = "REGISTRY_ZOOKEEPER_CONNECT_STRING"

	// configVolumeName is the framework-owned role group ConfigMap volume.
	configVolumeName = reconciler.ConfigVolumeName

	// workerDataVolumeName is the worker's data claim template name (legacy "worker-data").
	workerDataVolumeName = "worker-data"

	// workerDataMountPath is where the worker's data PVC is mounted: the data.basedir.path of
	// common.properties, i.e. the directory the worker writes task working data to. The legacy
	// operator rendered the claim but never mounted it; the framework refuses an unmounted claim.
	workerDataMountPath = "/tmp/dolphinscheduler"

	// curlCommand is the probe binary of the legacy actuator exec probes.
	curlCommand = "curl"

	// DolphinScheduler 3.2.2's PropertyUtils maps these environment names to the
	// resource.aws.* properties. They keep credential values out of ConfigMaps.
	s3AccessKeyEnvName = "RESOURCE_AWS_ACCESS_KEY_ID"
	s3SecretKeyEnvName = "RESOURCE_AWS_SECRET_ACCESS_KEY"
)

// DolphinSchedulerRoleGroupHandler builds the resources of a DolphinScheduler role group on top
// of reconciler.BaseRoleGroupHandler: the base handler owns labels, Services, the StatefulSet
// and the ConfigMap skeleton, while this handler declares the roles (reconciler.RoleProvider)
// and contributes the product specifics through post-build edits.
type DolphinSchedulerRoleGroupHandler struct {
	reconciler.BaseRoleGroupHandler[*dolphinv1alpha1.DolphinschedulerCluster]
}

var _ reconciler.RoleGroupHandler[*dolphinv1alpha1.DolphinschedulerCluster] = &DolphinSchedulerRoleGroupHandler{}
var _ reconciler.RoleProvider[*dolphinv1alpha1.DolphinschedulerCluster] = &DolphinSchedulerRoleGroupHandler{}

// ProductImageDefaults supplies whatever spec.image leaves empty, evaluated every reconcile so
// an operator upgrade moves clusters onto the co-released product image. It is the single image
// literal behind both GenericReconcilerConfig.ImageResolution.Defaults and the DB-init Job.
func ProductImageDefaults() commonsv1alpha1.ImageSpec {
	return commonsv1alpha1.ImageSpec{
		Repo:            dolphinv1alpha1.DefaultRepository,
		ProductVersion:  dolphinv1alpha1.DefaultProductVersion,
		KubedoopVersion: version.BuildVersion,
	}
}

// resolveProductImage resolves the container image and pull policy for the cluster from
// spec.image folded over the product defaults. The role path resolves through the framework
// (ImageResolution); this survives only for the DB-init Job, which is outside the role path.
func resolveProductImage(cr *dolphinv1alpha1.DolphinschedulerCluster) (string, corev1.PullPolicy, error) {
	spec := cr.GetSpec().Image
	image, err := spec.ResolveImage(dolphinv1alpha1.DefaultProductName, ProductImageDefaults())
	if err != nil {
		return "", "", err
	}
	return image, spec.ResolvedPullPolicy(ProductImageDefaults()), nil
}

// NewDolphinSchedulerRoleGroupHandler creates the handler. All per-role knowledge lives in
// DeclareRoles now; the handler itself carries only the reconcile-invariant identity settings.
func NewDolphinSchedulerRoleGroupHandler(scheme *runtime.Scheme) *DolphinSchedulerRoleGroupHandler {
	h := &DolphinSchedulerRoleGroupHandler{}
	h.Scheme = scheme
	h.LabelDomain = LabelDomain
	return h
}

// rolePorts returns the container ports of a role, byte-identical (names, numbers, order) to
// the legacy rendering (the order is the legacy sorted-by-name order).
func rolePorts(roleName string) []corev1.ContainerPort {
	switch roleName {
	case dolphinv1alpha1.RoleMaster:
		return []corev1.ContainerPort{
			{Name: dolphinv1alpha1.MasterActualPortName, ContainerPort: dolphinv1alpha1.MasterActualPort, Protocol: corev1.ProtocolTCP},
			{Name: dolphinv1alpha1.MasterPortName, ContainerPort: dolphinv1alpha1.MasterPort, Protocol: corev1.ProtocolTCP},
		}
	case dolphinv1alpha1.RoleWorker:
		return []corev1.ContainerPort{
			{Name: dolphinv1alpha1.WorkerActualPortName, ContainerPort: dolphinv1alpha1.WorkerActualPort, Protocol: corev1.ProtocolTCP},
			{Name: dolphinv1alpha1.WorkerPortName, ContainerPort: dolphinv1alpha1.WorkerPort, Protocol: corev1.ProtocolTCP},
		}
	case dolphinv1alpha1.RoleApi:
		return []corev1.ContainerPort{
			{Name: dolphinv1alpha1.ApiPortName, ContainerPort: dolphinv1alpha1.ApiPort, Protocol: corev1.ProtocolTCP},
			{Name: dolphinv1alpha1.ApiPythonPortName, ContainerPort: dolphinv1alpha1.ApiPythonPort, Protocol: corev1.ProtocolTCP},
		}
	case dolphinv1alpha1.RoleAlert:
		return []corev1.ContainerPort{
			{Name: dolphinv1alpha1.AlerterActualPortName, ContainerPort: dolphinv1alpha1.AlerterActualPort, Protocol: corev1.ProtocolTCP},
			{Name: dolphinv1alpha1.AlerterPortName, ContainerPort: dolphinv1alpha1.AlerterPort, Protocol: corev1.ProtocolTCP},
		}
	default:
		return nil
	}
}

// servicePortsFor mirrors the container ports as Service ports with a named targetPort, the
// legacy shape.
func servicePortsFor(ports []corev1.ContainerPort) []corev1.ServicePort {
	svcPorts := make([]corev1.ServicePort, 0, len(ports))
	for _, p := range ports {
		svcPorts = append(svcPorts, corev1.ServicePort{
			Name:       p.Name,
			Port:       p.ContainerPort,
			Protocol:   p.Protocol,
			TargetPort: intstr.FromString(p.Name),
		})
	}
	return svcPorts
}

// DeclareRoles implements reconciler.RoleProvider: everything a DolphinScheduler role is made
// of — container name, ports, command + start script, env, probes, log producers, config
// defaults and the worker data volume — computed once per reconcile from THIS cr.
func (h *DolphinSchedulerRoleGroupHandler) DeclareRoles(
	ctx context.Context,
	c ctrlclient.Client,
	cr *dolphinv1alpha1.DolphinschedulerCluster,
) (reconciler.RoleCatalog, error) {
	clusterConfig := cr.Spec.ClusterConfig
	if clusterConfig == nil {
		return nil, fmt.Errorf("spec.clusterConfig is required")
	}

	s3Info, err := resolveS3Bucket(ctx, c, cr)
	if err != nil {
		return nil, err
	}
	s3ExportScript := s3CredentialsExportScript(s3Info)

	// Authentication (api role only): resolves the AuthenticationClass into container env and,
	// for LDAP, a credentials-export prologue for the start script. The CSI bind-credentials
	// volume is resolved again in BuildResources (per role group), where VolumeProviders live.
	var auth *security.AuthenticationResult
	if clusterConfig.Authentication != nil {
		var err error
		auth, err = security.Authentication(ctx, c, cr.Namespace, clusterConfig.Authentication)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve authentication: %w", err)
		}
	}

	catalog := reconciler.RoleCatalog{}
	for role, container := range mainContainerNames {
		metricsPort, err := metricsPortForRole(role)
		if err != nil {
			return nil, err
		}

		resources, err := roleResourceDefaults(role)
		if err != nil {
			return nil, err
		}
		affinity, err := defaultRoleAffinity(cr.Name, role)
		if err != nil {
			return nil, err
		}

		// Command carries the start script followed by bash's fixed $0 placeholder: the
		// declaration has no Args slot, so args stay purely the user's cliOverrides and are
		// forwarded by the script through "$@". The LDAP
		// credentials-export prologue precedes the single start block (api only).
		script := mainContainerScript(role)
		if s3ExportScript != "" {
			script = s3ExportScript + "\n" + script
		}
		if role == dolphinv1alpha1.RoleApi && auth != nil && auth.LdapExportCommand != "" {
			script = auth.LdapExportCommand + "\n" + script
		}

		// Product env: sorted role defaults, then the ZooKeeper discovery env, then (api only)
		// the auth env. Declared env is emitted beneath the merged overrides, so a user's
		// envOverrides of the same name still wins at runtime.
		env := roleEnvDefaults(role)
		env = append(env, corev1.EnvVar{
			Name: zkConnectStringEnvName,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: clusterConfig.ZookeeperConfigMapName},
					Key:                  constant.ZookeeperDiscoveryKey,
				},
			},
		})
		env = append(env, databaseCredentialEnv(cr)...)
		if role == dolphinv1alpha1.RoleApi && auth != nil {
			env = append(env, auth.EnvVars...)
		}

		ports := rolePorts(role)
		decl := reconciler.RoleDeclaration{
			MainContainerName: container,
			ContainerPorts:    ports,
			ServicePorts:      servicePortsFor(ports),
			Command:           append(slices.Clone(containerCommand), script, roleCommandArgZero),
			Env:               env,
			ReadinessProbe:    actuatorProbe(metricsPort, "readiness"),
			LivenessProbe:     actuatorProbe(metricsPort, "liveness"),
			StartupProbe:      startupProbe(metricsPort),
			LogProducers: []productlogging.ContainerLogging{{
				Container: container,
				Framework: productlogging.LoggingFrameworkLogback,
				FileName:  dolphinv1alpha1.LogbackPropertiesFileName,
				Pattern:   dolphinv1alpha1.ConsoleConversionPattern,
			}},
			// Keep the legacy shared log emptyDir size (the framework default is larger).
			LogVolumeSize: legacyLogVolumeSize,
			ConfigDefaults: &commonsv1alpha1.RoleGroupConfigSpec{
				Resources:               resources,
				Affinity:                affinity,
				GracefulShutdownTimeout: ptr.To(defaultGracefulShutdown),
			},
		}

		// Only the worker has a data PVC: the claim is built from the effective
		// config.resources.storage (2Gi default) and mounted at data.basedir.path.
		if role == dolphinv1alpha1.RoleWorker {
			decl.DataVolume = &reconciler.DataVolume{
				Name:      workerDataVolumeName,
				MountPath: workerDataMountPath,
			}
		}

		catalog[role] = decl
	}

	return catalog, nil
}

// BuildResources builds all resources of one DolphinScheduler role group.
func (h *DolphinSchedulerRoleGroupHandler) BuildResources(
	ctx context.Context,
	k8sClient ctrlclient.Client,
	cr *dolphinv1alpha1.DolphinschedulerCluster,
	buildCtx *reconciler.RoleGroupBuildContext,
) (*reconciler.RoleGroupResources, error) {
	roleName := buildCtx.RoleName
	containerName, known := mainContainerNames[roleName]
	if !known {
		return nil, fmt.Errorf("unsupported role: %s", roleName)
	}

	clusterConfig := cr.Spec.ClusterConfig
	if clusterConfig == nil {
		return nil, fmt.Errorf("spec.clusterConfig is required")
	}

	s3Info, err := resolveS3Bucket(ctx, k8sClient, cr)
	if err != nil {
		return nil, err
	}
	if s3Info != nil {
		if provisioner := s3Info.CredentialsProvisioner(opgos3.DefaultCredentialsVolumeName); provisioner != nil {
			buildCtx.VolumeProviders = append(buildCtx.VolumeProviders, provisioner)
		}
	}

	// Authentication (api role only): re-resolved here (client reads are cached) for the CSI
	// bind-credentials volume, which is per role group; env and the script prologue are already
	// in the declaration from DeclareRoles.
	if roleName == dolphinv1alpha1.RoleApi && clusterConfig.Authentication != nil {
		auth, err := security.Authentication(ctx, k8sClient, cr.Namespace, clusterConfig.Authentication)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve authentication: %w", err)
		}
		if auth.Provisioner != nil {
			buildCtx.VolumeProviders = append(buildCtx.VolumeProviders, auth.Provisioner)
		}
	}

	res, err := h.BaseRoleGroupHandler.BuildResources(ctx, k8sClient, cr, buildCtx)
	if err != nil {
		return nil, err
	}

	// Post-build edits of the primary container (matched by name, never index): the envFrom on
	// "<cluster>-envs" and the two subPath config mounts. Neither has a declaration channel:
	// envFrom is not a declaration field, and a subPath mount targets a file path inside the
	// product's conf directory. The framework's whole-directory config mount at
	// /kubedoop/mount/config is kept (harmless read-only directory).
	containers := res.StatefulSet.Spec.Template.Spec.Containers
	for i := range containers {
		if containers[i].Name != containerName {
			continue
		}
		c := &containers[i]

		c.EnvFrom = append(c.EnvFrom, corev1.EnvFromSource{
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: envsConfigMapName(buildCtx.ClusterName)},
			},
		})

		// DolphinScheduler reads its config files at fixed paths inside the server's conf
		// directory, so mount the same "config" volume (the role group ConfigMap) twice more
		// with a subPath, ahead of the framework's mounts (legacy mount order).
		mounts := make([]corev1.VolumeMount, 0, len(c.VolumeMounts)+2)
		mounts = append(mounts,
			corev1.VolumeMount{
				Name:      configVolumeName,
				MountPath: roleConfigPath(roleName, dolphinv1alpha1.DolphinCommonPropertiesName),
				SubPath:   dolphinv1alpha1.DolphinCommonPropertiesName,
			},
			corev1.VolumeMount{
				Name:      configVolumeName,
				MountPath: roleConfigPath(roleName, dolphinv1alpha1.LogbackPropertiesFileName),
				SubPath:   dolphinv1alpha1.LogbackPropertiesFileName,
			},
		)
		mounts = append(mounts, c.VolumeMounts...)
		c.VolumeMounts = mounts
	}

	// common.properties: re-render the merged key set (product defaults < role < group
	// overrides, from MergedConfig) with the legacy sorted "k=v" serializer, applying the
	// S3-derived keys last — byte-identical content to the legacy ConfigMap.
	commonProperties, err := h.renderCommonProperties(ctx, k8sClient, cr, buildCtx)
	if err != nil {
		return nil, err
	}
	res.ConfigMap.Data[dolphinv1alpha1.DolphinCommonPropertiesName] = commonProperties

	// Compute the rollout hash only after all product-specific ConfigMap rendering is complete.
	// This makes inline configuration changes (including S3 endpoint, region, and bucket) update
	// the Pod template even though no referenced object's resourceVersion changes.
	dependencyFingerprint, err := runtimeDependencyFingerprint(ctx, k8sClient, cr, roleName, res.ConfigMap.Data)
	if err != nil {
		return nil, err
	}
	if res.StatefulSet.Spec.Template.Annotations == nil {
		res.StatefulSet.Spec.Template.Annotations = map[string]string{}
	}
	res.StatefulSet.Spec.Template.Annotations[runtimeDependencyFingerprintAnnotation] = dependencyFingerprint

	// Metrics Service: keep the legacy name, headless ClusterIP, and Prometheus metadata, while
	// selecting the Gen3 pod labels and targeting the role's actual metrics port.
	metricsService, err := h.buildMetricsService(buildCtx)
	if err != nil {
		return nil, err
	}
	res.MetricsService = metricsService

	return res, nil
}

// roleConfigPath returns the config file path inside the container:
// /kubedoop/dolphinscheduler/<role>-server/conf/<file>.
func roleConfigPath(roleName, fileName string) string {
	return path.Join(opgoconstant.KubedoopRoot, dolphinv1alpha1.DefaultProductName, roleServerName(roleName), "conf", fileName)
}

// envsConfigMapName is the cluster-wide env ConfigMap every container envFroms.
func envsConfigMapName(clusterName string) string {
	return clusterName + "-envs"
}

// actuatorProbe is the legacy Spring actuator exec probe (curl, 30s delay, 30s period).
func actuatorProbe(port int32, endpoint string) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{
					curlCommand,
					fmt.Sprintf("http://localhost:%d/actuator/health/%s", port, endpoint),
				},
			},
		},
		InitialDelaySeconds: 30,
		SuccessThreshold:    1,
		FailureThreshold:    3,
		PeriodSeconds:       30,
		TimeoutSeconds:      5,
	}
}

// startupProbe is new relative to the legacy rendering (zookeeper-operator precedent): it gives
// a slow first start up to 10 minutes before the readiness/liveness budgets apply, so a cold
// database or registry cannot crash-loop the servers.
func startupProbe(port int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{
					curlCommand,
					fmt.Sprintf("http://localhost:%d/actuator/health/readiness", port),
				},
			},
		},
		PeriodSeconds:    10,
		FailureThreshold: 60,
		SuccessThreshold: 1,
		TimeoutSeconds:   5,
	}
}

// renderCommonProperties serializes the merged common.properties (with the S3 overlay applied
// last) using the legacy sorted "key=value" serializer.
func (h *DolphinSchedulerRoleGroupHandler) renderCommonProperties(
	ctx context.Context,
	k8sClient ctrlclient.Client,
	cr *dolphinv1alpha1.DolphinschedulerCluster,
	buildCtx *reconciler.RoleGroupBuildContext,
) (string, error) {
	props := map[string]string{}
	if buildCtx.MergedConfig != nil {
		maps.Copy(props, buildCtx.MergedConfig.ConfigFiles[dolphinv1alpha1.DolphinCommonPropertiesName])
	}

	if cr.Spec.ClusterConfig.S3 != nil {
		s3Info, err := resolveS3Bucket(ctx, k8sClient, cr)
		if err != nil {
			return "", err
		}
		props["resource.storage.type"] = "S3"
		props["resource.aws.region"] = s3Info.Region
		props["resource.aws.s3.bucket.name"] = s3Info.BucketName
		props["resource.aws.s3.endpoint"] = s3Info.Endpoint.String()
	}

	return util.ToProperties(props), nil
}

func resolveS3Bucket(
	ctx context.Context,
	k8sClient ctrlclient.Client,
	cr *dolphinv1alpha1.DolphinschedulerCluster,
) (*opgos3.BucketInfo, error) {
	if cr.Spec.ClusterConfig == nil || cr.Spec.ClusterConfig.S3 == nil {
		return nil, nil
	}
	info, err := opgos3.ResolveBucket(ctx, k8sClient, cr.Namespace, cr.Spec.ClusterConfig.S3, "")
	if err != nil {
		return nil, fmt.Errorf("failed to resolve S3 bucket: %w", err)
	}
	if !info.PathStyle {
		return nil, fmt.Errorf("S3 pathStyle must be true because DolphinScheduler 3.2.2 cannot honor virtual-host addressing for a configured endpoint")
	}
	if info.TLS != nil && info.TLS.Verification != nil {
		verification := info.TLS.Verification
		if verification.None != nil {
			return nil, fmt.Errorf("S3 TLS verification mode none is not supported by DolphinScheduler 3.2.2")
		}
		if verification.Server != nil && verification.Server.CACert != nil &&
			verification.Server.CACert.SecretClass != "" {
			return nil, fmt.Errorf("S3 TLS custom CA SecretClass is not supported until JVM trust material can be provisioned")
		}
	}
	return info, nil
}

// s3CredentialsExportScript bridges the framework's SecretClass CSI credential files to the
// environment keys DolphinScheduler 3.2.2's PropertyUtils uses for resource.aws.*. The values
// exist only inside the container process; neither the controller nor a ConfigMap sees them.
func s3CredentialsExportScript(info *opgos3.BucketInfo) string {
	if info == nil || info.Credentials == nil {
		return ""
	}
	return s3CredentialsExportScriptForMount(
		opgos3.CredentialsMountPath(opgos3.DefaultCredentialsVolumeName))
}

func s3CredentialsExportScriptForMount(mountPath string) string {
	accessKeyFile := path.Join(mountPath, opgos3.AccessKeyFile)
	secretKeyFile := path.Join(mountPath, opgos3.SecretKeyFile)
	// TODO(operator-go): replace this local fragment once CredentialsExportScript propagates
	// credential-file read failures and rejects empty values. Its current "export VAR=$(cat ...)"
	// shape masks cat's status, so set -e cannot stop an unsafe ambient-credential fallback.
	return strings.Join([]string{
		fmt.Sprintf(`AWS_ACCESS_KEY_ID="$(cat "%s")"`, accessKeyFile),
		`: "${AWS_ACCESS_KEY_ID:?S3 access key file is empty}"`,
		fmt.Sprintf(`AWS_SECRET_ACCESS_KEY="$(cat "%s")"`, secretKeyFile),
		`: "${AWS_SECRET_ACCESS_KEY:?S3 secret key file is empty}"`,
		`export AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY`,
		`export ` + s3AccessKeyEnvName + `="$AWS_ACCESS_KEY_ID"`,
		`export ` + s3SecretKeyEnvName + `="$AWS_SECRET_ACCESS_KEY"`,
	}, "\n")
}

// buildMetricsService assembles the metrics Service slot. Its selector must come from the base
// handler: those exact labels are stamped on the Gen3 pod template.
func (h *DolphinSchedulerRoleGroupHandler) buildMetricsService(buildCtx *reconciler.RoleGroupBuildContext) (*corev1.Service, error) {
	metricsPort, err := metricsPortForRole(buildCtx.RoleName)
	if err != nil {
		return nil, err
	}

	labels := legacyRoleGroupLabels(buildCtx.ClusterName, buildCtx.RoleName, buildCtx.RoleGroupName)
	labels["prometheus.io/scrape"] = defaultEnabled

	annotations := map[string]string{
		"prometheus.io/scrape": defaultEnabled,
		"prometheus.io/path":   "/actuator/prometheus",
		"prometheus.io/port":   strconv.Itoa(int(metricsPort)),
		"prometheus.io/scheme": "http",
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        buildCtx.ResourceName + "-metrics",
			Namespace:   buildCtx.ClusterNamespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeClusterIP,
			ClusterIP: corev1.ClusterIPNone,
			Ports: []corev1.ServicePort{
				{
					Name:       dolphinv1alpha1.MetricsPortName,
					Port:       metricsPort,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt32(metricsPort),
				},
			},
			Selector: h.SelectorLabels(buildCtx),
		},
	}, nil
}
