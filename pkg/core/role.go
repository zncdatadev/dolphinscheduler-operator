package core

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/zncdatadev/dolphinscheduler-operator/pkg/util"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Role string

type RoleReconciler interface {
	MergeConfig()
	ReconcileRole(ctx context.Context) (ctrl.Result, error)
}

type RoleReconcilerRequirements interface {
	MergeConfig() map[string]any
	RegisterResources(ctx context.Context) map[string][]ResourceReconciler
}

// RoleGroupRecociler RoleReconcile role reconciler interface
// all role reconciler should implement this interface
type RoleGroupRecociler interface {
	ReconcileGroup(ctx context.Context) (ctrl.Result, error)
}

type RoleConfiguration struct {
	Config        any
	RoleGroups    []string
	RolePdbConfig *PdbConfig
}

//new role configuration

func NewRoleConfiguration(config any, roleGroups []string,
	rolePdbConfig *PdbConfig) *RoleConfiguration {
	return &RoleConfiguration{
		Config:        config,
		RoleGroups:    roleGroups,
		RolePdbConfig: rolePdbConfig,
	}
}

type ClusterConfigGetter interface {
	GetRoleConfig(role Role) *RoleConfiguration
}

type RoleConfigGetter interface {
	Config() any
}

type RoleGroupConfigGetter interface {
	Replicas() int32
	MergedConfig() any
	GroupPdbSpec() *PdbConfig
	NodeSelector() map[string]string
}

var _ RoleReconciler = &BaseRoleReconciler[client.Object]{}

type BaseRoleReconciler[T client.Object] struct {
	Scheme             *runtime.Scheme
	Instance           T
	Client             client.Client
	RoleLabels         map[string]string
	InstanceAttributes InstanceAttributes
	RoleReconcilerRequirements
	Role              Role
	RolePdbReconciler ResourceReconciler
}

func NewBaseRoleReconciler[T client.Object](scheme *runtime.Scheme, instance T, client client.Client, role Role,
	roleLabels map[string]string, instanceAttributes InstanceAttributes, helper RoleReconcilerRequirements,
	rolePdbReconciler ResourceReconciler) *BaseRoleReconciler[T] {
	return &BaseRoleReconciler[T]{
		Scheme:                     scheme,
		Instance:                   instance,
		Client:                     client,
		InstanceAttributes:         instanceAttributes,
		Role:                       role,
		RoleLabels:                 roleLabels,
		RoleReconcilerRequirements: helper,
		RolePdbReconciler:          rolePdbReconciler,
	}
}

func (r *BaseRoleReconciler[T]) MergeConfig() {
	mergedCfgs := r.RoleReconcilerRequirements.MergeConfig()
	for groupName, cfg := range mergedCfgs {
		StoreSingleGroupConfig(r.Instance.GetName(), r.Role, groupName, cfg)
	}
}

func (r *BaseRoleReconciler[T]) ReconcileRole(ctx context.Context) (ctrl.Result, error) {
	roleCfg := r.InstanceAttributes.GetRoleConfig(r.Role)
	if r.RolePdbReconciler != nil {
		res, err := SingleResourceDoReconcile(ctx, r.RolePdbReconciler)
		if err != nil {
			return ctrl.Result{}, err
		}
		if res.RequeueAfter > 0 {
			return res, nil
		}
	}

	// reconciler groups
	for _, name := range roleCfg.RoleGroups {
		resourceReconcilers := r.RoleReconcilerRequirements.RegisterResources(ctx)
		groupResources := resourceReconcilers[name]
		groupReconciler := NewBaseRoleGroupReconciler(r.Scheme, r.Instance, r.Client, r.Role, name, groupResources)
		res, err := groupReconciler.ReconcileGroup(ctx)
		if err != nil {
			return ctrl.Result{}, err
		}
		if res.RequeueAfter > 0 {
			return res, nil
		}
	}
	return ctrl.Result{}, nil
}

type BaseRoleGroupReconciler[T client.Object] struct {
	Scheme    *runtime.Scheme
	Instance  T
	Client    client.Client
	Role      Role
	GroupName string

	Reconcilers []ResourceReconciler
}

func NewBaseRoleGroupReconciler[T client.Object](scheme *runtime.Scheme, instance T, client client.Client, role Role,
	groupName string, resourceReconcilers []ResourceReconciler) *BaseRoleGroupReconciler[T] {
	return &BaseRoleGroupReconciler[T]{
		Scheme:      scheme,
		Instance:    instance,
		Client:      client,
		Role:        role,
		GroupName:   groupName,
		Reconcilers: resourceReconcilers,
	}
}

func ReconcilersDoReconcile(ctx context.Context, reconcilers []ResourceReconciler) (ctrl.Result, error) {
	for _, r := range reconcilers {
		res, err := SingleResourceDoReconcile(ctx, r)
		if err != nil {
			return ctrl.Result{}, err
		}
		if res.RequeueAfter > 0 {
			return res, nil
		}
	}
	return ctrl.Result{}, nil
}

func SingleResourceDoReconcile(ctx context.Context, r ResourceReconciler) (ctrl.Result, error) {
	if single, ok := r.(ResourceBuilder); ok {
		if reflect.ValueOf(r).IsNil() {
			return ctrl.Result{}, nil
		}

		res, err := r.ReconcileResource(ctx, NewSingleResourceBuilder(single))
		if err != nil {
			return ctrl.Result{}, err
		}
		return res, nil
	} else if multi, ok := r.(MultiResourceReconcilerBuilder); ok {
		// todo : assert reconciler is MultiResourceReconciler
		res, err := r.ReconcileResource(ctx, NewMultiResourceBuilder(multi))
		if err != nil {
			return ctrl.Result{}, err
		}
		return res, nil
	} else {
		panic(fmt.Sprintf("unknown resource reconciler builder, actual type: %T", r))
	}
}

// ReconcileGroup ReconcileRole implements the Role interface
func (g *BaseRoleGroupReconciler[T]) ReconcileGroup(ctx context.Context) (ctrl.Result, error) {
	return ReconcilersDoReconcile(ctx, g.Reconcilers)
}

// MergeObjects merge right to left, if field not in left, it will be added from right,
// else skip.
// Note: If variable is a pointer, it will be modified directly.
func MergeObjects(left interface{}, right interface{}, exclude []string) {

	leftValues := reflect.ValueOf(left)
	rightValues := reflect.ValueOf(right)

	if leftValues.Kind() == reflect.Ptr {
		leftValues = leftValues.Elem()
	}

	if rightValues.Kind() == reflect.Ptr {
		rightValues = rightValues.Elem()
	}

	for i := 0; i < rightValues.NumField(); i++ {
		rightField := rightValues.Field(i)
		rightFieldName := rightValues.Type().Field(i).Name
		if !contains(exclude, rightFieldName) {
			// if right field is zero value, skip
			if reflect.DeepEqual(rightField.Interface(), reflect.Zero(rightField.Type()).Interface()) {
				continue
			}
			leftField := leftValues.FieldByName(rightFieldName)

			// if left field is zero value, set it use right field, else skip
			if !reflect.DeepEqual(leftField.Interface(), reflect.Zero(leftField.Type()).Interface()) {
				continue
			}

			leftField.Set(rightField)
		}
	}
}

func contains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}

type RoleLabelHelper struct {
}

func (h *RoleLabelHelper) GroupLabels(roleLabels map[string]string, groupName string,
	nodeSelector map[string]string) map[string]string {
	mergeLabels := make(util.Map)
	mergeLabels.MapMerge(roleLabels, true)
	mergeLabels.MapMerge(nodeSelector, true)
	mergeLabels["app.kubernetes.io/instance"] = strings.ToLower(groupName)
	return mergeLabels
}

func (h *RoleLabelHelper) RoleLabels(instanceName string, role Role) map[string]string {
	roleLabels := RoleLabels{InstanceName: instanceName, Name: string(role)}
	mergeLabels := roleLabels.GetLabels()
	return mergeLabels
}

type PdbConfig struct {
	MinAvailable int32

	MaxUnavailable int32
}
