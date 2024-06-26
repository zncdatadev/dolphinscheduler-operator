package core

import (
	"strings"
)

type RoleLabels struct {
	InstanceName string
	Name         string
}

func (r *RoleLabels) GetLabels() map[string]string {
	res := map[string]string{
		"app.kubernetes.io/Name":       strings.ToLower(r.InstanceName),
		"app.kubernetes.io/managed-by": "dolphinscheduler-operator",
	}
	if r.Name != "" {
		res["app.kubernetes.io/component"] = r.Name
	}
	return res
}

func GetListenerLabels(listenerClass ListenerClass) map[string]string {
	return map[string]string{
		ListenerAnnotationKey: string(listenerClass),
	}
}
