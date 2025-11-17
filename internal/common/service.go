package common

import (
	"fmt"
	"strconv"

	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/util"
	"github.com/zncdatadev/operator-go/pkg/builder"
	"github.com/zncdatadev/operator-go/pkg/client"
	opconstants "github.com/zncdatadev/operator-go/pkg/constants"
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
	var ports = make([]corev1.ContainerPort, 0)
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

// NewRoleGroupMetricsService creates a metrics service reconciler using a simple function approach
// This creates a headless service for metrics with Prometheus labels and annotations
func NewRoleGroupMetricsService(
	client *client.Client,
	roleGroupInfo *reconciler.RoleGroupInfo,
) reconciler.Reconciler {
	roleName := roleGroupInfo.GetRoleName()
	role := util.Role(roleName)
	// Get metrics port
	metricsPort, err := GetMetricsPort(role)
	if err != nil {
		// Return empty reconciler on error - should not happen
		fmt.Printf("GetMetricsPort error for role %v: %v. Skipping metrics service creation.\n", roleName, err)
		return nil
	}

	// Create service ports
	servicePorts := []corev1.ContainerPort{
		{
			Name:          dolphinv1alpha1.MetricsPortName,
			ContainerPort: metricsPort,
			Protocol:      corev1.ProtocolTCP,
		},
	}

	// Create service name with -metrics suffix
	serviceName := CreateServiceMetricsName(roleGroupInfo)

	scheme := "http"

	// Prepare labels (copy from roleGroupInfo and add metrics labels)
	labels := make(map[string]string)
	for k, v := range roleGroupInfo.GetLabels() {
		labels[k] = v
	}
	labels["prometheus.io/scrape"] = "true"

	// Prepare annotations (copy from roleGroupInfo and add Prometheus annotations)
	annotations := make(map[string]string)
	for k, v := range roleGroupInfo.GetAnnotations() {
		annotations[k] = v
	}
	annotations["prometheus.io/scrape"] = "true"
	annotations["prometheus.io/path"] = "/actuator/prometheus"
	annotations["prometheus.io/port"] = strconv.Itoa(int(metricsPort))
	annotations["prometheus.io/scheme"] = scheme

	// Create base service builder
	baseBuilder := builder.NewServiceBuilder(
		client,
		serviceName,
		servicePorts,
		func(sbo *builder.ServiceBuilderOptions) {
			sbo.Headless = true
			sbo.ListenerClass = opconstants.ClusterInternal
			sbo.Labels = labels
			sbo.MatchingLabels = roleGroupInfo.GetLabels() // Use original labels for matching
			sbo.Annotations = annotations
		},
	)

	return reconciler.NewGenericResourceReconciler(
		client,
		baseBuilder,
	)
}
