package main

import (
	"crypto/tls"
	"flag"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	kubeconfigprovider "sigs.k8s.io/multicluster-runtime/providers/kubeconfig"

	demov1 "github.com/v47/mcr-demo/api/v1"
	"github.com/v47/mcr-demo/internal/controller"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(demov1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var secretNamespace string
	var tlsOpts []func(*tls.Config)

	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true, "Serve the metrics endpoint securely via HTTPS.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false, "Enable HTTP/2 for the metrics and webhook servers.")
	flag.StringVar(&secretNamespace, "kubeconfig-secret-namespace", "default",
		"Namespace watched for labeled kubeconfig Secrets that register member clusters.")

	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}
	webhookServer := webhook.NewServer(webhook.Options{TLSOpts: tlsOpts})

	ctx := ctrl.SetupSignalHandler()

	cfg, err := ctrl.GetConfig()
	if err != nil {
		setupLog.Error(err, "unable to get kubeconfig")
		os.Exit(1)
	}

	// The kubeconfig provider watches labeled Secrets (one per member cluster) in
	// secretNamespace and registers/deregisters clusters with the manager at
	// runtime. ClusterOptions injects our scheme into every engaged cluster so
	// their caches and clients understand this project's CRDs.
	provider := kubeconfigprovider.New(kubeconfigprovider.Options{
		Namespace: secretNamespace,
		ClusterOptions: []cluster.Option{
			func(o *cluster.Options) { o.Scheme = scheme },
		},
	})

	// mcmanager.New takes the provider as its second argument; mcmanager.Options
	// is a type alias for controller-runtime's manager.Options.
	mgr, err := mcmanager.New(cfg, provider, mcmanager.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:   metricsAddr,
			SecureServing: secureMetrics,
			TLSOpts:       tlsOpts,
		},
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "mcr-demo.example.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := (&controller.WidgetReconciler{
		Manager: mgr,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "widget")
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

	if err := provider.SetupWithManager(ctx, mgr); err != nil {
		setupLog.Error(err, "unable to setup kubeconfig provider")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
