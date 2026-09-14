/*
Copyright 2024 zncdatadev.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"crypto/tls"
	"flag"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	dolphinschedulerv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdatadev/dolphinscheduler-operator/internal/controller"
	authv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/authentication/v1alpha1"
	commonsv1alph1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	s3v1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/s3/v1alpha1"
	opcommon "github.com/zncdatadev/operator-go/pkg/common"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(dolphinschedulerv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
	utilruntime.Must(commonsv1alph1.AddToScheme(scheme))
	utilruntime.Must(authv1alpha1.AddToScheme(scheme))
	utilruntime.Must(s3v1alpha1.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var tlsOpts []func(*tls.Config)
	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: tlsOpts,
	})

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.1/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.1/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		WebhookServer:          webhookServer,
		LeaderElectionID:       "935091a9.kubedoop.dev",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Cheap start-up insurance: the product config type must stay foldable (it currently only
	// embeds the commons RoleGroupConfigSpec, which validates clean).
	if err := reconciler.ValidateProductConfigType[dolphinschedulerv1alpha1.ConfigSpec](); err != nil {
		setupLog.Error(err, "product config type is not foldable")
		os.Exit(1)
	}

	// The extension registry runs the cluster extension that provisions the "<cluster>-envs"
	// ConfigMap and the create-once DB schema init Job before any role group (plus the
	// Deployment-to-StatefulSet upgrade guard).
	extensionRegistry := opcommon.NewExtensionRegistry[*dolphinschedulerv1alpha1.DolphinschedulerCluster]()
	extensionRegistry.RegisterClusterExtension(controller.NewClusterExtension(mgr.GetScheme()))

	roleGroupHandler := controller.NewDolphinSchedulerRoleGroupHandler(mgr.GetScheme())

	dolphinReconciler, err := reconciler.NewGenericReconciler(
		&reconciler.GenericReconcilerConfig[*dolphinschedulerv1alpha1.DolphinschedulerCluster]{
			Client: mgr.GetClient(),
			// Uncached: refreshes the resourceVersion after a conflicting status write, which
			// the informer cache is by definition too stale to serve.
			APIReader: mgr.GetAPIReader(),
			Scheme:    mgr.GetScheme(),
			// operator-go's Recorder field is the (deprecated) record.EventRecorder; the
			// replacement GetEventRecorder returns the incompatible events.EventRecorder.
			Recorder:         mgr.GetEventRecorderFor("dolphinscheduler-cluster-controller"), //nolint:staticcheck
			RoleGroupHandler: roleGroupHandler,
			// The handler declares the four roles; the reconciler resolves images and folds the
			// config defaults from the catalog.
			RoleProvider: roleGroupHandler,
			ImageResolution: reconciler.ImageResolution{
				ProductName: dolphinschedulerv1alpha1.DefaultProductName,
				Defaults:    controller.ProductImageDefaults(),
			},
			RoleGroupResolver: reconciler.RoleGroupResolverFunc[*dolphinschedulerv1alpha1.DolphinschedulerCluster](
				controller.ResolveRoleGroup),
			// The workload pods' permissions, bound to the derived ServiceAccount
			// ("dolphinschedulercluster-<cluster>"): the registry discovery ConfigMap read.
			WorkloadRBACRules: func(_ *dolphinschedulerv1alpha1.DolphinschedulerCluster) []rbacv1.PolicyRule {
				return []rbacv1.PolicyRule{{
					APIGroups: []string{""},
					Resources: []string{"configmaps"},
					Verbs:     []string{"get", "list", "watch"},
				}}
			},
			Dependencies: func(cr *dolphinschedulerv1alpha1.DolphinschedulerCluster) []reconciler.Dependency {
				var deps []reconciler.Dependency
				if cc := cr.Spec.ClusterConfig; cc != nil {
					if cc.ZookeeperConfigMapName != "" {
						deps = append(deps, reconciler.Dependency{
							Kind: reconciler.DependencyConfigMap,
							Name: cc.ZookeeperConfigMapName,
						})
					}
					if cc.Database != nil && cc.Database.CredentialsSecret != "" {
						deps = append(deps, reconciler.Dependency{
							Kind: reconciler.DependencySecret,
							Name: cc.Database.CredentialsSecret,
						})
					}
					for _, authentication := range cc.Authentication {
						if authentication.Oidc != nil && authentication.Oidc.Secret != "" {
							deps = append(deps, reconciler.Dependency{
								Kind: reconciler.DependencySecret,
								Name: authentication.Oidc.Secret,
							})
						}
					}
				}
				return deps
			},
			Prototype:         &dolphinschedulerv1alpha1.DolphinschedulerCluster{},
			ExtensionRegistry: extensionRegistry,
		})
	if err != nil {
		setupLog.Error(err, "unable to create reconciler", "controller", "DolphinschedulerCluster")
		os.Exit(1)
	}

	if err := dolphinReconciler.SetupWithManagerOpts(mgr, reconciler.SetupWithManagerOptions{
		// The extension-owned kinds: watched for out-of-band edits. The framework registers the
		// Role/RoleBinding watches itself when WorkloadRBACRules is set.
		ExtraOwns: []ctrlclient.Object{
			&batchv1.Job{},
			&corev1.Secret{},
		},
		Watches: controller.ReferenceWatches(mgr.GetClient()),
	}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DolphinschedulerCluster")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
