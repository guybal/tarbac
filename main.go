package main

import (
	"flag"
	"os"
    "fmt"

	tarbacv1 "github.com/guybal/tarbac/api/v1" // Adjust to match your actual module path
	sudorequest "github.com/guybal/tarbac/controllers/sudorequest"
	clustersudorequest "github.com/guybal/tarbac/controllers/clustersudorequest"
    temporaryrbac "github.com/guybal/tarbac/controllers/temporaryrbac"
	clustertemporaryrbac "github.com/guybal/tarbac/controllers/clustertemporaryrbac"
    sudopolicy "github.com/guybal/tarbac/controllers/sudopolicy"
	clustersudopolicy "github.com/guybal/tarbac/controllers/clustersudopolicy"
	"github.com/guybal/tarbac/webhooks"
    "sigs.k8s.io/controller-runtime/pkg/webhook"
	rbacv1 "k8s.io/api/rbac/v1"
    corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func main() {
	var enableLeaderElection bool
 	//var metricsAddr string

// 	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false, "Enable leader election for controller manager.")
	flag.Parse()

    defer func() {
        if r := recover(); r != nil {
            ctrl.Log.Error(fmt.Errorf("%v", r), "Application panic detected at startup")
            os.Exit(1)
        }
    }()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	// Create a runtime scheme
	scheme := runtime.NewScheme()


	// Register the tarbac.io/v1 API group
	utilruntime.Must(tarbacv1.AddToScheme(scheme))
    ctrl.Log.Info(fmt.Sprintf("Registered types in Scheme: %v", scheme.AllKnownTypes()))

	// Register the rbac.authorization.k8s.io/v1 API group
	utilruntime.Must(rbacv1.AddToScheme(scheme))

	utilruntime.Must(corev1.AddToScheme(scheme))

	// Create and start the manager
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:           scheme,
		LeaderElection:   enableLeaderElection,
		LeaderElectionID: "temporary-rbac-controller",
// 		Port:             "9443", // Webhook server port
//      CertDir:          "/tmp/k8s-webhook-server/serving-certs", // Directory for serving certificates
	})

	if err != nil {
		ctrl.Log.Error(err, "unable to start manager")
		os.Exit(1)
	}

    decoder := admission.NewDecoder(mgr.GetScheme())

    // Setup Webhooks
    mgr.GetWebhookServer().Register("/mutate-v1-sudorequest", &webhook.Admission{
        Handler: &webhooks.SudoRequestAnnotator{
            Scheme: mgr.GetScheme(), // Pass the manager's Scheme
            Decoder: decoder,
        },
    })
//     decoder := admission.NewDecoder(mgr.GetScheme())
//     annotator := &webhooks.SudoRequestAnnotator{Decoder: decoder, Scheme: mgr.GetScheme()}
//     mgr.GetWebhookServer().Register("/mutate-v1-sudorequest", &admission.Webhook{
//         Handler: annotator,
//     })

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

    // Add SudoRequestReconciler to the manager
    if err = (&sudorequest.SudoRequestReconciler{
        Client: mgr.GetClient(),
    }).SetupWithManager(mgr); err != nil {
        ctrl.Log.Error(err, "unable to create controller", "controller", "SudoRequest")
        os.Exit(1)
    }

    // Add ClusterSudoRequestReconciler to the manager
    if err = (&clustersudorequest.ClusterSudoRequestReconciler{
    	Client: mgr.GetClient(),
    }).SetupWithManager(mgr); err != nil {
    	ctrl.Log.Error(err, "unable to create controller", "controller", "ClusterSudoRequest")
    	os.Exit(1)
    }

    if err = (&sudopolicy.SudoPolicyReconciler{
        Client: mgr.GetClient(),
    }).SetupWithManager(mgr); err != nil {
        ctrl.Log.Error(err, "unable to create controller", "controller", "SudoPolicy")
        os.Exit(1)
    }

    if err = (&clustersudopolicy.ClusterSudoPolicyReconciler{
    	Client: mgr.GetClient(),
    }).SetupWithManager(mgr); err != nil {
    	ctrl.Log.Error(err, "unable to create controller", "controller", "ClusterSudoPolicy")
    	os.Exit(1)
    }

	ctrl.Log.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		ctrl.Log.Error(err, "problem running manager")
		os.Exit(1)
	}
}
