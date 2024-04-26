package common

import (
	"fmt"
	dolphinv1alpha1 "github.com/zncdata-labs/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/core"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/resource"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/util"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	"strconv"
	"strings"
)

func OverrideEnvVars(origin *[]corev1.EnvVar, override map[string]string) {
	var originVars = make(map[string]int)
	for i, env := range *origin {
		originVars[env.Name] = i
	}

	for k, v := range override {
		// if env Name is in override, then override it
		if idx, ok := originVars[k]; ok {
			(*origin)[idx].Value = v
		} else {
			// if override's key is new, then append it
			*origin = append(*origin, corev1.EnvVar{
				Name:  k,
				Value: v,
			})
		}
	}
}

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

func CreateLog4jBuilder(containerLogging *dolphinv1alpha1.LoggingConfigSpec, consoleAppenderName,
	fileAppenderName string, fileLogLocation string) *resource.Log4jLoggingDataBuilder {
	log4jBuilder := &resource.Log4jLoggingDataBuilder{}
	if loggers := containerLogging.Loggers; loggers != nil {
		var builderLoggers []resource.LogBuilderLoggers
		for logger, level := range loggers {
			builderLoggers = append(builderLoggers, resource.LogBuilderLoggers{
				Logger: logger,
				Level:  level.Level,
			})
		}
		log4jBuilder.Loggers = builderLoggers
	}
	if console := containerLogging.Console; console != nil {
		log4jBuilder.Console = &resource.LogBuilderAppender{
			AppenderName: consoleAppenderName,
			Level:        console.Level,
		}
	}
	if file := containerLogging.File; file != nil {
		log4jBuilder.File = &resource.LogBuilderAppender{
			AppenderName:       fileAppenderName,
			Level:              file.Level,
			DefaultLogLocation: fileLogLocation,
		}
	}

	return log4jBuilder
}

func K8sEnvRef(envName string) string {
	return fmt.Sprintf("$(%s)", envName)
}

func LinuxEnvRef(envName string) string {
	return fmt.Sprintf("$%s", envName)
}

func PdbCfg(pdbSpec *dolphinv1alpha1.PodDisruptionBudgetSpec) *core.PdbConfig {
	return &core.PdbConfig{
		MaxUnavailable: pdbSpec.MaxUnavailable,
		MinAvailable:   pdbSpec.MinAvailable,
	}
}
func ExtractDataBaseReference(dbSpec *dolphinv1alpha1.DatabaseSpec) (*resource.DatabaseConfiguration, *resource.DatabaseParams) {
	inlineDb := dbSpec.Inline
	db := resource.DatabaseConfiguration{
		DbReference: &dbSpec.Reference,
		DbInline: resource.NewDatabaseParams(
			inlineDb.Driver,
			inlineDb.Username,
			inlineDb.Password,
			inlineDb.Host,
			strconv.Itoa(int(inlineDb.Port)),
			inlineDb.DatabaseName),
	}
	params, err := db.GetDatabaseParams()
	if err != nil {
		panic(err)
	}
	return &db, params
}

func MakeDataBaseEnvs(dbSpec *dolphinv1alpha1.DatabaseSpec) []corev1.EnvVar {
	db, params := ExtractDataBaseReference(dbSpec)
	uri, err := db.GetURI()
	if err != nil {
		panic(err)
	}
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

var _ core.RoleGroupConfigGetter = &GenericRoleGroupGetter{}

type GenericRoleGroupGetter struct {
	core.RoleConfigGetter
	mergedCfg *dolphinv1alpha1.RoleGroupSpec
}

func (r *GenericRoleGroupGetter) Replicas() int32 {
	return r.mergedCfg.Replicas
}

func NewRoleGroupTransformer(
	roleConfigGetter core.RoleConfigGetter,
	groupSpec *dolphinv1alpha1.RoleGroupSpec) *GenericRoleGroupGetter {
	return &GenericRoleGroupGetter{RoleConfigGetter: roleConfigGetter, mergedCfg: groupSpec}
}

func (r *GenericRoleGroupGetter) MergedConfig() any {
	return r.mergedCfg
}

func (r *GenericRoleGroupGetter) GroupPdbSpec() *core.PdbConfig {
	pdbSpec := r.mergedCfg.Config.PodDisruptionBudget
	if pdbSpec == nil {
		return nil
	}
	return &core.PdbConfig{
		MinAvailable:   pdbSpec.MinAvailable,
		MaxUnavailable: pdbSpec.MaxUnavailable,
	}
}

func (r *GenericRoleGroupGetter) NodeSelector() map[string]string {
	return r.mergedCfg.Config.NodeSelector
}

var _ core.InstanceAttributes = &DolphinSchedulerClusterInstance{}

type DolphinSchedulerClusterInstance struct {
	Instance *dolphinv1alpha1.DolphinschedulerCluster
}

func (k *DolphinSchedulerClusterInstance) GetRoleConfig(role core.Role) *core.RoleConfiguration {
	switch role {
	case core.Master:
		masterSpec := k.Instance.Spec.Master
		roleGetter := &DolphinSchedulerRoleGetter{masterSpec.Config}
		groups := maps.Keys(masterSpec.RoleGroups)
		return k.transformRoleSpec(roleGetter, groups, masterSpec.PodDisruptionBudget)
	case core.Worker:
		workerSpec := k.Instance.Spec.Worker
		roleGetter := &DolphinSchedulerRoleGetter{workerSpec.Config}
		groups := maps.Keys(workerSpec.RoleGroups)
		return k.transformRoleSpec(roleGetter, groups, workerSpec.PodDisruptionBudget)
	case core.Alerter:
		alerterSpec := k.Instance.Spec.Alerter
		roleGetter := &DolphinSchedulerRoleGetter{alerterSpec.Config}
		groups := maps.Keys(alerterSpec.RoleGroups)
		return k.transformRoleSpec(roleGetter, groups, alerterSpec.PodDisruptionBudget)
	case core.Api:
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
	return core.NewRoleConfiguration(roleConfigGetter.Config(), groups,
		&core.PdbConfig{
			MinAvailable:   rolePdbSpec.MinAvailable,
			MaxUnavailable: rolePdbSpec.MinAvailable,
		})
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
