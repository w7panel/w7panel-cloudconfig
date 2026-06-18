package main

import (
	"flag"
	"os"

	cloudv1 "github.com/w7panel/w7panel-cloudconfig/api/v1alpha1"
	"github.com/w7panel/w7panel-cloudconfig/controllers"
	"github.com/w7panel/w7panel-cloudconfig/pkg/httpapi"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(cloudv1.AddToScheme(scheme))
}

func envString(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func main() {
	var metricsAddr string
	var probeAddr string
	var httpAddr string
	var leaderElection bool
	flag.StringVar(&metricsAddr, "metrics-bind-address", envString("METRICS_BIND_ADDRESS", ":18080"), "metrics bind address")
	flag.StringVar(&probeAddr, "health-probe-bind-address", envString("HEALTH_PROBE_BIND_ADDRESS", ":8081"), "health probe bind address")
	flag.StringVar(&httpAddr, "http-bind-address", envString("HTTP_BIND_ADDRESS", ":8001"), "HTTP API bind address")
	flag.BoolVar(&leaderElection, "leader-elect", false, "enable leader election")
	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         leaderElection,
		LeaderElectionID:       "w7panel-cloudconfig",
	})
	if err != nil {
		os.Exit(1)
	}
	if err := (&controllers.CloudConfigReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()}).SetupWithManager(mgr); err != nil {
		os.Exit(1)
	}
	if err := mgr.Add(&httpapi.Server{Addr: httpAddr, Client: mgr.GetClient(), APIReader: mgr.GetAPIReader()}); err != nil {
		os.Exit(1)
	}
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		os.Exit(1)
	}
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		os.Exit(1)
	}
}
