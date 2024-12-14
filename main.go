package main

import (
	"flag"
	"os"

	"github.com/guybal/tarbac/api/v1" // Update with your module path
	"github.com/guybal/tarbac/controllers"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	var enableLeaderElection bool

	// Define flags for the manager
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false, "Enable leader election for controller manager.")
	flag.Parse()

	// Set up the logger
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	// Initialize a new scheme
	scheme := runtime.NewScheme()
	utilruntime.Must(v1.AddToScheme(scheme))

	// Create the manager
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:           scheme,
		LeaderElection:   enableLeaderElection,
		LeaderElectionID: "temporary-rbac-controller",
	})
	if err != nil {
		ctrl.Log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Set up the controller
	if err := (&controllers.TemporaryRBACReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		ctrl.Log.Error(err, "unable to create controller", "controller", "TemporaryRBAC")
		os.Exit(1)
	}

	// Start the manager
	ctrl.Log.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		ctrl.Log.Error(err, "problem running manager")
		os.Exit(1)
	}
}
