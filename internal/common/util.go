package common

import (
	"context"
	"fmt"
	dolphinv1alpha1 "github.com/zncdata-labs/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/core"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/resource"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/util"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
)

const Master core.Role = "master"
const Worker core.Role = "worker"
const Alerter core.Role = "alerter"
const Api core.Role = "api"

func ConfigConfigMapName(instanceName string, groupName string) string {
	return util.NewResourceNameGenerator(instanceName, "", groupName).GenerateResourceName("config")
}

func EnvsConfigMapName(instanceName string, groupName string) string {
	return util.NewResourceNameGenerator(instanceName, "", groupName).GenerateResourceName("envs")
}

func CreateNetworkUrl(podName string, svcName, namespace, clusterDomain string, port int32) string {
	return podName + "." + CreateDnsDomain(svcName, namespace, clusterDomain, port)
}

func CreateDnsDomain(svcName, namespace, clusterDomain string, port int32) string {
	return fmt.Sprintf("%s:%d", CreateDomainHost(svcName, namespace, clusterDomain), port)
}

func CreateDomainHost(svcName, namespace, clusterDomain string) string {
	return fmt.Sprintf("%s.%s.svc.%s", svcName, namespace, clusterDomain)
}

// CreatePodNamesByReplicas create pod names by replicas
func CreatePodNamesByReplicas(replicas int32, statefulResourceName string) []string {
	podNames := make([]string, replicas)
	for i := int32(0); i < replicas; i++ {
		podName := fmt.Sprintf("%s-%d", statefulResourceName, i)
		podNames[i] = podName
	}
	return podNames
}

func CreateServiceAccountName(instanceName string) string {
	return util.NewResourceNameGeneratorOneRole(instanceName, "").GenerateResourceName("sa")
}

func CreateKvContentByReplicas(replicas int32, keyTemplate string, valueTemplate string) [][2]string {
	var res [][2]string
	for i := int32(0); i < replicas; i++ {
		key := fmt.Sprintf(keyTemplate, i)
		var value string
		if strings.Contains(valueTemplate, "%d") {
			value = fmt.Sprintf(valueTemplate, i)
		} else {
			value = valueTemplate
		}
		res = append(res, [2]string{key, value})
	}
	return res
}

func K8sEnvRef(envName string) string {
	return fmt.Sprintf("$(%s)", envName)
}

func LinuxEnvRef(envName string) string {
	return fmt.Sprintf("$%s", envName)
}

func TransformApiLogger(logging *dolphinv1alpha1.LoggingConfigSpec) *resource.TextTemplateLoggingDataBuilder {
	var customLoggers []resource.LoggerLevel
	for logger, lvl := range logging.Loggers {
		customLoggers = append(customLoggers, resource.LoggerLevel{
			Logger: logger,
			Level:  lvl.Level,
		})
	}
	return &resource.TextTemplateLoggingDataBuilder{
		Loggers: customLoggers,
		Console: &resource.LoggingAppender{
			Level: logging.Console.Level,
		},
		File: &resource.LoggingAppender{Level: logging.File.Level},
	}
}

func PdbCfg(pdbSpec *dolphinv1alpha1.PodDisruptionBudgetSpec) *core.PdbConfig {
	if pdbSpec == nil {
		return nil
	}
	return &core.PdbConfig{
		MaxUnavailable: pdbSpec.MaxUnavailable,
		MinAvailable:   pdbSpec.MinAvailable,
	}
}
func ExtractDataBaseReference(dbSpec *dolphinv1alpha1.DatabaseSpec, ctx context.Context, client client.Client,
	namespace string) (*resource.DatabaseConfiguration, *resource.DatabaseParams) {
	db := resource.DatabaseConfiguration{
		DbReference:    &dbSpec.Reference,
		ResourceClient: core.NewResourceClient(ctx, client, namespace),
	}
	if inlineDb := dbSpec.Inline; inlineDb != nil {
		db.DbInline = resource.NewDatabaseParams(
			inlineDb.Driver,
			inlineDb.Username,
			inlineDb.Password,
			inlineDb.Host,
			strconv.Itoa(int(inlineDb.Port)),
			inlineDb.DatabaseName)
	}
	params, err := db.GetDatabaseParams()
	if err != nil {
		panic(err)
	}
	return &db, params
}

func MakeDataBaseEnvs(params *resource.DatabaseParams) []corev1.EnvVar {
	uri := resource.ToUri(params)
	return []corev1.EnvVar{
		{
			Name:  "DATABASE",
			Value: string(params.DbType),
		},
		{
			Name:  "SPRING_DATASOURCE_URL",
			Value: uri,
		},
		{
			Name:  "SPRING_DATASOURCE_USERNAME",
			Value: params.Username,
		},
		{
			Name:  "SPRING_DATASOURCE_PASSWORD",
			Value: params.Password,
		},
		{
			Name:  "SPRING_DATASOURCE_DRIVER-CLASS-NAME",
			Value: params.Driver,
		},
	}
}

var _ core.InstanceAttributes = &DolphinSchedulerClusterInstance{}

type DolphinSchedulerClusterInstance struct {
	Instance *dolphinv1alpha1.DolphinschedulerCluster
}

func (k *DolphinSchedulerClusterInstance) GetRoleConfig(role core.Role) *core.RoleConfiguration {
	switch role {
	case Master:
		masterSpec := k.Instance.Spec.Master
		roleGetter := &DolphinSchedulerRoleGetter{masterSpec.Config}
		groups := maps.Keys(masterSpec.RoleGroups)
		return k.transformRoleSpec(roleGetter, groups, masterSpec.PodDisruptionBudget)
	case Worker:
		workerSpec := k.Instance.Spec.Worker
		roleGetter := &DolphinSchedulerRoleGetter{workerSpec.Config}
		groups := maps.Keys(workerSpec.RoleGroups)
		return k.transformRoleSpec(roleGetter, groups, workerSpec.PodDisruptionBudget)
	case Alerter:
		alerterSpec := k.Instance.Spec.Alerter
		roleGetter := &DolphinSchedulerRoleGetter{alerterSpec.Config}
		groups := maps.Keys(alerterSpec.RoleGroups)
		return k.transformRoleSpec(roleGetter, groups, alerterSpec.PodDisruptionBudget)
	case Api:
		apiSpec := k.Instance.Spec.Api
		roleGetter := &DolphinSchedulerRoleGetter{apiSpec.Config}
		groups := maps.Keys(apiSpec.RoleGroups)
		return k.transformRoleSpec(roleGetter, groups, apiSpec.PodDisruptionBudget)
	default:
		panic(fmt.Sprintf("unknown role: %s", role))
	}
}

// masterSpec to RoleConfiguration
func (k *DolphinSchedulerClusterInstance) transformRoleSpec(
	roleConfigGetter core.RoleConfigGetter,
	groups []string,
	rolePdbSpec *dolphinv1alpha1.PodDisruptionBudgetSpec,
) *core.RoleConfiguration {
	var pdbCfg *core.PdbConfig
	if rolePdbSpec != nil {
		pdbCfg = &core.PdbConfig{
			MinAvailable:   rolePdbSpec.MinAvailable,
			MaxUnavailable: rolePdbSpec.MinAvailable,
		}
	}
	return core.NewRoleConfiguration(roleConfigGetter.Config(), groups, pdbCfg)
}

func (k *DolphinSchedulerClusterInstance) GetClusterConfig() any {
	return k.Instance.Spec.ClusterConfigSpec
}

func (k *DolphinSchedulerClusterInstance) GetNamespace() string {
	return k.Instance.GetNamespace()
}

func (k *DolphinSchedulerClusterInstance) GetInstanceName() string {
	return k.Instance.GetName()
}

var _ core.RoleConfigGetter = &DolphinSchedulerRoleGetter{}

type DolphinSchedulerRoleGetter struct {
	roleSpec *dolphinv1alpha1.ConfigSpec
}

func (d DolphinSchedulerRoleGetter) Config() any {
	return d.roleSpec
}
