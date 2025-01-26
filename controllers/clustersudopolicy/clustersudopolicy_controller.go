package controllers

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/guybal/tarbac/api/v1"
	utils "github.com/guybal/tarbac/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	// 	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterSudoPolicyReconciler struct {
	client.Client
	Recorder record.EventRecorder
}

const ReconciliationInterval = time.Minute * 5

// Reconcile handles reconciliation for ClusterSudoPolicy objects
func (r *ClusterSudoPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	utils.LogInfo(logger, "Reconciling ClusterSudoPolicy", "name", req.Name)

	// Fetch the ClusterSudoPolicy object
	var clusterSudoPolicy v1.ClusterSudoPolicy
	if err := r.Get(ctx, req.NamespacedName, &clusterSudoPolicy); err != nil {
		if apierrors.IsNotFound(err) {
			utils.LogInfo(logger, "ClusterSudoPolicy resource not found. Ignoring since it must have been deleted.", "name", req.Name)
			return ctrl.Result{}, nil
		}
		utils.LogError(logger, err, "Unable to fetch ClusterSudoPolicy", "name", req.Name, "namespace", req.Namespace)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Validate maxDuration
	if _, err := time.ParseDuration(clusterSudoPolicy.Spec.MaxDuration); err != nil {
		return r.errorRequest(ctx, err, &clusterSudoPolicy, fmt.Sprintf("Invalid MaxDuration in ClusterSudoPolicy spec: %s", clusterSudoPolicy.Spec.MaxDuration))
	}

	if clusterSudoPolicy.Spec.AllowedNamespaces != nil && clusterSudoPolicy.Spec.AllowedNamespacesSelector != nil {
		errorMessage := "both allowedNamespaces and allowedNamespacesSelector cannot be set simultaneously"
		err := fmt.Errorf("%s", errorMessage)
		return r.errorRequest(ctx, err, &clusterSudoPolicy, errorMessage)

	}
	if clusterSudoPolicy.Spec.AllowedNamespaces == nil && clusterSudoPolicy.Spec.AllowedNamespacesSelector == nil {
		errorMessage := "either allowedNamespaces or allowedNamespacesSelector must be set"
		err := fmt.Errorf("%s", errorMessage)
		return r.errorRequest(ctx, err, &clusterSudoPolicy, errorMessage)
	}

	// Parse namespaces into status
	var namespaces []string
	if clusterSudoPolicy.Spec.AllowedNamespaces != nil {
		namespaces = clusterSudoPolicy.Spec.AllowedNamespaces
	} else if clusterSudoPolicy.Spec.AllowedNamespacesSelector != nil {
		// Fetch namespaces based on selector
		var namespaceList corev1.NamespaceList
		selector, err := metav1.LabelSelectorAsSelector(clusterSudoPolicy.Spec.AllowedNamespacesSelector)
		if err != nil {
			return r.errorRequest(ctx, err, &clusterSudoPolicy, "Invalid label selector in ClusterSudoPolicy spec")
		}
		if err := r.List(ctx, &namespaceList, &client.ListOptions{LabelSelector: selector}); err != nil {
			return r.errorRequest(ctx, err, &clusterSudoPolicy, "Failed to list namespaces in ClusterSudoPolicy spec")
		}
		for _, ns := range namespaceList.Items {
			namespaces = append(namespaces, ns.Name)
		}
	}

	// Update ClusterSudoPolicy status
	clusterSudoPolicy.Status.State = "Active"
	clusterSudoPolicy.Status.Namespaces = namespaces
	if err := r.Status().Update(ctx, &clusterSudoPolicy); err != nil {
		return r.errorRequest(ctx, err, &clusterSudoPolicy, "Failed to update ClusterSudoPolicy status")
	}

	utils.LogInfo(logger, "Successfully validated ClusterSudoPolicy", "name", clusterSudoPolicy.Name, "kind", clusterSudoPolicy.Kind)

	if clusterSudoPolicy.Spec.AllowedNamespacesSelector != nil {
		utils.LogInfo(logger, "Rescheduling reconciliation due to dynamic AllowedNamespacesSelector", "name", clusterSudoPolicy.Name, "kind", clusterSudoPolicy.Kind)
		return ctrl.Result{RequeueAfter: ReconciliationInterval}, nil
	}

	return ctrl.Result{}, nil
}

func (r *ClusterSudoPolicyReconciler) errorRequest(ctx context.Context, err error, clusterSudoPolicy *v1.ClusterSudoPolicy, message string) (ctrl.Result, error) {

	logger := log.FromContext(ctx)
	utils.LogError(logger, err, "ClusterSudoPolicy Error", "errorMessage", message)
	clusterSudoPolicy.Status.State = "Error"
	clusterSudoPolicy.Status.ErrorMessage = message
	if err := r.Status().Update(ctx, clusterSudoPolicy); err != nil {
		utils.LogError(logger, err, "Failed to update ClusterSudoPolicy status to Error")
		return ctrl.Result{}, err
	}
	r.Recorder.Event(clusterSudoPolicy, "Error", "ClusterSudoPolicyError", message)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterSudoPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("ClusterSudoPolicyController")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ClusterSudoPolicy{}).
		Complete(r)
}
