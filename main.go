package main

import (
	"flag"
	"os"

	tarbacv1 "github.com/guybal/tarbac/api/v1" // Adjust to match your actual module path
// 	controllers "github.com/guybal/tarbac/controllers"
    temporaryrbac "github.com/guybal/tarbac/controllers/temporaryrbac"
	clustertemporaryrbac "github.com/guybal/tarbac/controllers/clustertemporaryrbac"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	var enableLeaderElection bool

	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false, "Enable leader election for controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	// Create a runtime scheme
	scheme := runtime.NewScheme()

	// Register the tarbac.io/v1 API group
	utilruntime.Must(tarbacv1.AddToScheme(scheme))

	// Register the rbac.authorization.k8s.io/v1 API group
	utilruntime.Must(rbacv1.AddToScheme(scheme))

	// Create and start the manager
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:           scheme,
		LeaderElection:   enableLeaderElection,
		LeaderElectionID: "temporary-rbac-controller",
//         MetricsBindAddress: ":8081", // Change to a different port if needed
//         Port:               9443,   // For webhook server
	})
	if err != nil {
		ctrl.Log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Set up the TemporaryRBAC reconciler
	if err := (&temporaryrbac.TemporaryRBACReconciler{
    	Client: mgr.GetClient(),
    	Scheme: mgr.GetScheme(),
    }).SetupWithManager(mgr); err != nil {
    	ctrl.Log.Error(err, "unable to create controller", "controller", "TemporaryRBAC")
    	os.Exit(1)
    }

	// Set up the ClusterTemporaryRBAC reconciler
    if err := (&clustertemporaryrbac.ClusterTemporaryRBACReconciler{
    	Client: mgr.GetClient(),
    	Scheme: mgr.GetScheme(),
    }).SetupWithManager(mgr); err != nil {
    	ctrl.Log.Error(err, "unable to create controller", "controller", "ClusterTemporaryRBAC")
    	os.Exit(1)
    }

	ctrl.Log.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		ctrl.Log.Error(err, "problem running manager")
		os.Exit(1)
	}
}
