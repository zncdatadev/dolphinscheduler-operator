package common

import (
	"context"

	"emperror.dev/errors"
	v1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/builder"
	"github.com/zncdatadev/operator-go/pkg/productlogging"
	"github.com/zncdatadev/operator-go/pkg/util"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var vectorLogger = ctrl.Log.WithName("vector")

const ContainerVector = "vector"

func IsVectorEnable(roleLoggingConfig *v1alpha1.ContainerLoggingSpec) bool {
	if roleLoggingConfig != nil {
		return roleLoggingConfig.EnableVectorAgent
	}
	return false

}

type VectorConfigParams struct {
	Client        ctrlclient.Client
	ClusterConfig *v1alpha1.ClusterConfigSpec
	Namespace     string
	InstanceName  string
	Role          string
	GroupName     string
}

func generateVectorYAML(ctx context.Context, params VectorConfigParams) (string, error) {
	aggregatorConfigMapName := params.ClusterConfig.VectorAggregatorConfigMapName
	if aggregatorConfigMapName == "" {
		return "", errors.New("vectorAggregatorConfigMapName is not set")
	}
	return productlogging.MakeVectorYaml(ctx, params.Client, params.Namespace, params.InstanceName, params.Role,
		params.GroupName, aggregatorConfigMapName)
}

func ExtendConfigMapDataByVector(ctx context.Context, params VectorConfigParams, data map[string]string) {
	vectorYaml, err := generateVectorYAML(ctx, params)
	if err != nil {
		vectorLogger.Error(errors.Wrap(err, "error creating vector YAML"), "failed to create vector YAML")
	} else {
		data[builder.VectorConfigFile] = vectorYaml
	}
}

func ExtendWorkloadByVector(
	image *util.Image,
	workloadObject ctrlclient.Object,
	vectorConfigMapName string) {
	decorator := builder.NewVectorDecorator(workloadObject, image, v1alpha1.LoggingVolumeName, v1alpha1.ConfigVolumeName, vectorConfigMapName)
	err := decorator.Decorate()
	if err != nil {
		vectorLogger.Error(errors.Wrap(err, "error decorating workload"), "failed to decorate workload")
		return
	}
}
