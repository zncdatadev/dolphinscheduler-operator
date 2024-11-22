package common

import (
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/util"
	"github.com/zncdatadev/operator-go/pkg/builder"
	"github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func NewServiceReconciler(
	cli *client.Client,
	name string,
	headless bool,
	serviceType *corev1.ServiceType,
	ports map[string]int32, labels, annotations map[string]string) *reconciler.Service {
	b := NewServiceBuilder(cli, name, headless, serviceType, ports, labels, annotations)
	return &reconciler.Service{
		GenericResourceReconciler: *reconciler.NewGenericResourceReconciler(cli, b),
	}
}

func NewServiceBuilder(client *client.Client, name string, headless bool, serviceType *corev1.ServiceType,
	portMap map[string]int32, lables map[string]string, annotations map[string]string) builder.ServiceBuilder {
	var sortedPortMap util.SortedMap = make(map[string]interface{})
	// ports
	for k, v := range portMap {
		sortedPortMap[k] = v
	}
	var ports []corev1.ContainerPort = make([]corev1.ContainerPort, 0)
	sortedPortMap.Range(func(k string, v interface{}) bool {
		port, err := ToContainerPortInt32(v)
		if err != nil {
			contaienrBuilderLogger.Error(err, "convert port to int32 failed")
			return false
		}
		ports = append(ports, corev1.ContainerPort{
			Name:          k,
			ContainerPort: port,
		})
		return true
	})

	// listener class
	if serviceType == nil {
		serviceType = ptr.To(corev1.ServiceTypeClusterIP)
	}
	listenerClass, err := util.ServiceTypeToListenerClass(*serviceType)
	if err != nil {
		contaienrBuilderLogger.Error(err, "convert service type to listener class failed")
		return nil
	}

	b := builder.NewServiceBuilder(client, name, ports, func(sbo *builder.ServiceBuilderOptions) {
		sbo.Labels = lables
		sbo.Annotations = annotations
		sbo.Headless = headless
		sbo.MatchingLabels = lables
		sbo.ListenerClass = listenerClass
	})
	return b
}
