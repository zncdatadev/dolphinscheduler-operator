package util

import (
	"github.com/zncdatadev/operator-go/pkg/constants"
	"github.com/zncdatadev/operator-go/pkg/errors"
	corev1 "k8s.io/api/core/v1"
)

func ServiceTypeToListenerClass(serviceType corev1.ServiceType) (listenerClass constants.ListenerClass, err error) {
	switch serviceType {
	case corev1.ServiceTypeClusterIP:
		listenerClass = constants.ClusterInternal
	case corev1.ServiceTypeNodePort:
		listenerClass = constants.ExternalUnstable
	case corev1.ServiceTypeLoadBalancer:
		listenerClass = constants.ExternalStable
	default:
		err = errors.Errorf("unsupported service type to get listener class: %s", serviceType)
	}
	return
}
