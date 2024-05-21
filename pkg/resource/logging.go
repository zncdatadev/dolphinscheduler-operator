package resource

import (
	"bytes"
	"context"
	"text/template"

	"github.com/zncdatadev/dolphinscheduler-operator/pkg/core"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewLoggingReconciler new logging reconcile
func NewLoggingReconciler[T client.Object](
	scheme *runtime.Scheme,
	instance T,
	client client.Client,
	groupName string,
	mergedLabels map[string]string,
	mergedCfg any,
	logDataBuilder RoleLoggingDataBuilder,
	role core.Role,
	configmapName string,
) *LoggingRecociler[T, any] {
	return &LoggingRecociler[T, any]{
		GeneralResourceStyleReconciler: *core.NewGeneraResourceStyleReconciler(
			scheme,
			instance,
			client,
			groupName,
			mergedLabels,
			mergedCfg,
		),
		RoleLoggingDataBuilder: logDataBuilder,
		role:                   role,
		ConfigmapName:          configmapName,
	}
}

type RoleLoggingDataBuilder interface {
	MakeContainerLogData() map[string]string
}

type OverrideExistLogging interface {
	OverrideExist(exist *corev1.ConfigMap)
}

type GenericRoleLoggingDataBuilder struct {
	Role        core.Role
	LogTemplate string
	LogFileName string
	LoggingContentGenerator
}

func NewGenericRoleLoggingDataBuilder(role core.Role, logTemplate string, logFileName string,
	loggingContentGenerator LoggingContentGenerator) *GenericRoleLoggingDataBuilder {
	return &GenericRoleLoggingDataBuilder{
		Role:                    role,
		LogTemplate:             logTemplate,
		LogFileName:             logFileName,
		LoggingContentGenerator: loggingContentGenerator,
	}
}

func (b *GenericRoleLoggingDataBuilder) MakeContainerLogData() map[string]string {
	data := b.LoggingContentGenerator.OverrideAndGet(b.LogTemplate)
	return map[string]string{
		b.LogFileName: data,
	}
}

type LoggingRecociler[T client.Object, G any] struct {
	core.GeneralResourceStyleReconciler[T, G]
	RoleLoggingDataBuilder RoleLoggingDataBuilder
	role                   core.Role
	ConfigmapName          string
}

// Build  config map
func (l *LoggingRecociler[T, G]) Build(_ context.Context) (client.Object, error) {
	cmData := l.RoleLoggingDataBuilder.MakeContainerLogData()
	if len(cmData) == 0 {
		return nil, nil
	}
	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      l.ConfigmapName,
			Namespace: l.Instance.GetNamespace(),
			Labels:    l.Labels,
		},
		Data: cmData,
	}
	return obj, nil
}

// OverrideExistLoggingRecociler override exist logging config reconciler
// if log properties exist in some configmap, we need to override it
type OverrideExistLoggingRecociler[T client.Object, G any] struct {
	core.GeneralResourceStyleReconciler[T, G]
	RoleLoggingDataBuilder RoleLoggingDataBuilder
}

// NewOverrideExistLoggingRecociler new OverrideExistLoggingReconcile
func NewOverrideExistLoggingRecociler[T client.Object](
	scheme *runtime.Scheme,
	instance T,
	client client.Client,
	groupName string,
	mergedLabels map[string]string,
	mergedCfg any,
	logDataBuilder RoleLoggingDataBuilder,
) *OverrideExistLoggingRecociler[T, any] {
	return &OverrideExistLoggingRecociler[T, any]{
		GeneralResourceStyleReconciler: *core.NewGeneraResourceStyleReconciler(
			scheme,
			instance,
			client,
			groupName,
			mergedLabels,
			mergedCfg,
		),
		RoleLoggingDataBuilder: logDataBuilder,
	}
}

// OverrideExist override exist logging config
func (l *OverrideExistLoggingRecociler[T, G]) OverrideExist(exist *corev1.ConfigMap) {
	exist.Data = l.RoleLoggingDataBuilder.MakeContainerLogData()
}

// LoggingContentGenerator all logging data builder abstract interface
type LoggingContentGenerator interface {
	OverrideAndGet(origin string) string
}

type LoggerLevel struct {
	Logger string
	Level  string
}

type LoggingAppender struct {
	Level              string
	DefaultLogLocation string
}

var _ LoggingContentGenerator = &TextTemplateLoggingDataBuilder{}

var logging = ctrl.Log.WithName("logging")

type TextTemplateLoggingDataBuilder struct {
	Loggers []LoggerLevel
	Console *LoggingAppender
	File    *LoggingAppender
}

func (l *TextTemplateLoggingDataBuilder) OverrideAndGet(tmpl string) string {
	t, err := template.New("logging").Parse(tmpl)
	if err != nil {
		logging.Error(err, "failed to parse template", "template", tmpl)
		return tmpl
	}
	var b bytes.Buffer
	if err := t.Execute(&b, l); err != nil {
		logging.Error(err, "failed to execute template", "template", tmpl, "data", l)
		return tmpl
	}
	return b.String()
}
