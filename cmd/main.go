// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"flag"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	complianceV1alpha1 "go.miloapis.com/compliance/api/v1alpha1"
	"go.miloapis.com/compliance/internal/controller"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(complianceV1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var platformKubeconfig string
	var leaderElectionNamespace string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&platformKubeconfig, "milo-kubeconfig", "",
		"Path to a kubeconfig file for the Milo API server. "+
			"If empty, the in-cluster client is used.")
	flag.StringVar(&leaderElectionNamespace, "leader-election-namespace", "",
		"Namespace where the leader election Lease is stored. When --milo-kubeconfig "+
			"is set, the controller pod's own namespace usually does not exist on Milo; "+
			"point this at a namespace that does (e.g. milo-system). If empty, "+
			"controller-runtime's default detection is used.")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Build the REST config. When --milo-kubeconfig is provided the manager
	// talks directly to the Milo aggregated API server (where the Vendor and
	// Subprocessor CRDs are installed). Otherwise fall back to the in-cluster
	// config for local development. The leader-election Lease lives on the
	// same cluster — Milo when wired up that way, otherwise the host cluster.
	var (
		restCfg *rest.Config
		err     error
	)
	if platformKubeconfig != "" {
		restCfg, err = clientcmd.BuildConfigFromFlags("", platformKubeconfig)
		if err != nil {
			setupLog.Error(err, "unable to build milo kubeconfig", "path", platformKubeconfig)
			os.Exit(1)
		}
		setupLog.Info("using milo kubeconfig", "path", platformKubeconfig)
	} else {
		restCfg = ctrl.GetConfigOrDie()
		setupLog.Info("using in-cluster config")
	}

	mgr, err := ctrl.NewManager(restCfg, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress:  probeAddr,
		LeaderElection:          enableLeaderElection,
		LeaderElectionID:        "compliance.miloapis.com",
		LeaderElectionNamespace: leaderElectionNamespace,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controller.VendorReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Log:    ctrl.Log.WithName("controllers").WithName("Vendor"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Vendor")
		os.Exit(1)
	}
	if err = (&controller.VendorImportReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Log:    ctrl.Log.WithName("controllers").WithName("VendorImport"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "VendorImport")
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
