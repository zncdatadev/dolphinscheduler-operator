package resource

import (
	"context"
	"fmt"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/core"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewConfigMapBuilder(
	name string,
	namespace string,
	labels map[string]string,
	configGenerators []interface{},
) *ConfigMapBuilder {
	return &ConfigMapBuilder{
		Name:             name,
		Namespace:        namespace,
		Labels:           labels,
		ConfigGenerators: configGenerators,
	}
}

type ConfigMapType interface {
	core.ResourceBuilder
	core.ConfigurationOverride
}

type ConfigMapBuilder struct {
	Name             string
	Namespace        string
	Labels           map[string]string
	ConfigGenerators []interface{}
}

func (c *ConfigMapBuilder) Build() *corev1.ConfigMap {
	var data = make(map[string]string)
	if len(c.ConfigGenerators) == 0 {
		top := c.ConfigGenerators[0]
		switch top.(type) {
		case FileContentGenerator:
			var fileGenerators = make([]FileContentGenerator, 0)
			for _, generator := range c.ConfigGenerators {
				fileGenerators = append(fileGenerators, generator.(FileContentGenerator))
			}
			data = GenerateAllFile(fileGenerators)
		case EnvGenerator:
			var envGenerators = make([]EnvGenerator, 0)
			for _, generator := range c.ConfigGenerators {
				envGenerators = append(envGenerators, generator.(EnvGenerator))
			}
			data = GenerateAllEnv(envGenerators)
		default:
			panic(fmt.Sprintf("config generators not supported: %v", top))
		}
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name,
			Namespace: c.Namespace,
			Labels:    c.Labels,
		},
		Data: data,
	}
}

type ConfigType string

const (
	Xml        ConfigType = "xml"
	Properties ConfigType = "properties"
)

// OverrideConfigFileContent override config file content
// if we need to override config file content, we should use this function
// if field exist in current configMap, we need to override it
// if field not exist in current configMap, we need to append it
func OverrideConfigFileContent(current string, override map[string]string, configType ConfigType) string {
	switch configType {
	case Xml:
		return util.OverrideXmlContent(current, override)
	case Properties:
		overrideParis := make([]util.NameValuePair, 0)
		for k, v := range override {
			overrideParis = append(overrideParis, util.NameValuePair{
				Name:  k,
				Value: v,
			})
		}
		content, err := util.OverridePropertiesFileContent(current, overrideParis)
		if err != nil {
			return ""
		}
		return content
	default:
		panic(fmt.Sprintf("unknown config type: %s", configType))
	}
}

type ConfigGenerator[T string | map[string]string] interface {
	Generate() T
}

// FileContentGenerator generate config
// we can use this interface to generate config content
// and use GenerateAllFile function to generate configMap data
type FileContentGenerator interface {
	ConfigGenerator[string]
	FileName() string
}

type EnvGenerator interface {
	ConfigGenerator[map[string]string]
}

func GenerateAllFile(confGenerator []FileContentGenerator) map[string]string {
	data := make(map[string]string)
	for _, generator := range confGenerator {
		if generator.Generate() != "" {
			data[generator.FileName()] = generator.Generate()
		}
	}
	return data
}

func GenerateAllEnv(confGenerator []EnvGenerator) map[string]string {
	data := make(map[string]string)
	for _, generator := range confGenerator {
		if generator.Generate() != nil {
			for k, v := range generator.Generate() {
				data[k] = v
			}
		}
	}
	return data
}

type SecurityProtocol string

const (
	Plaintext SecurityProtocol = "PLAINTEXT"
	Ssl       SecurityProtocol = "SSL"
	SaslSsl   SecurityProtocol = "SASL_SSL"
	SaslPlain SecurityProtocol = "SASL_PLAINTEXT"
)

// GeneralConfigMapReconciler general config map reconciler generator
// it can be used to generate config map reconciler for simple config map
// parameters:
// 1. resourceBuilerFunc: a function to create a new resource
type GeneralConfigMapReconciler[T client.Object, G any] struct {
	core.GeneralResourceStyleReconciler[T, G]
	resourceBuilderFunc       func() (*corev1.ConfigMap, error)
	configurationOverrideFunc func() error
}

// NewGeneralConfigMap new a GeneralConfigMapReconciler
func NewGeneralConfigMap[T client.Object, G any](
	scheme *runtime.Scheme,
	instance T,
	client client.Client,
	groupName string,
	mergedLabels map[string]string,
	mergedCfg G,
	resourceBuilderFunc func() (*corev1.ConfigMap, error),
	configurationOverrideFunc func() error,

) *GeneralConfigMapReconciler[T, G] {
	return &GeneralConfigMapReconciler[T, G]{
		GeneralResourceStyleReconciler: *core.NewGeneraResourceStyleReconciler[T, G](
			scheme,
			instance,
			client,
			groupName,
			mergedLabels,
			mergedCfg),
		resourceBuilderFunc:       resourceBuilderFunc,
		configurationOverrideFunc: configurationOverrideFunc,
	}
}

// Build implements the ResourceBuilder interface
func (c *GeneralConfigMapReconciler[T, G]) Build(_ context.Context) (client.Object, error) {
	return c.resourceBuilderFunc()
}

// ConfigurationOverride implement ConfigurationOverride interface
func (c *GeneralConfigMapReconciler[T, G]) ConfigurationOverride(resource client.Object) {
	if c.configurationOverrideFunc != nil {
		err := c.configurationOverrideFunc()
		if err != nil {
			return
		}
	}
}
