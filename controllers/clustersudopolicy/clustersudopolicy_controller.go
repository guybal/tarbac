package controllers

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/guybal/tarbac/api/v1"
	utils "github.com/guybal/tarbac/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	// 	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterSudoPolicyReconciler struct {
	client.Client
}

const ReconciliationInterval = time.Minute * 5

// Reconcile handles reconciliation for ClusterSudoPolicy objects
func (r *ClusterSudoPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	utils.LogInfo(logger, "Reconciling ClusterSudoPolicy")
	// 	logger.Info("Reconciling ClusterSudoPolicy", "name", req.Name, "namespace", req.Namespace)

	// Fetch the ClusterSudoPolicy object
	var ClusterSudoPolicy v1.ClusterSudoPolicy
	if err := r.Get(ctx, req.NamespacedName, &ClusterSudoPolicy); err != nil {
		logger.Error(err, "Unable to fetch ClusterSudoPolicy")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Validate maxDuration
	if _, err := time.ParseDuration(ClusterSudoPolicy.Spec.MaxDuration); err != nil {
		logger.Error(err, "Invalid maxDuration in ClusterSudoPolicy spec", "maxDuration", ClusterSudoPolicy.Spec.MaxDuration)
		return ctrl.Result{}, err
	}

	if ClusterSudoPolicy.Spec.AllowedNamespaces != nil && ClusterSudoPolicy.Spec.AllowedNamespacesSelector != nil {
		err := fmt.Errorf("Both allowedNamespaces and allowedNamespacesSelector cannot be set simultaneously")
		logger.Error(err, "Validation failed")
		return ctrl.Result{}, err
	}
	if ClusterSudoPolicy.Spec.AllowedNamespaces == nil && ClusterSudoPolicy.Spec.AllowedNamespacesSelector == nil {
		err := fmt.Errorf("Either allowedNamespaces or allowedNamespacesSelector must be set")
		logger.Error(err, "Validation failed")
		return ctrl.Result{}, err
	}

	// Check for mutual exclusivity of allowedNamespaces and allowedNamespacesSelector
	if ClusterSudoPolicy.Spec.AllowedNamespaces != nil && ClusterSudoPolicy.Spec.AllowedNamespacesSelector != nil {
		logger.Error(nil, "Both allowedNamespaces and allowedNamespacesSelector cannot be set simultaneously")
		return ctrl.Result{}, nil
	}
	if ClusterSudoPolicy.Spec.AllowedNamespaces == nil && ClusterSudoPolicy.Spec.AllowedNamespacesSelector == nil {
		logger.Error(nil, "Either allowedNamespaces or allowedNamespacesSelector must be set")
		return ctrl.Result{}, nil
	}

	// Parse namespaces into status
	var namespaces []string
	if ClusterSudoPolicy.Spec.AllowedNamespaces != nil {
		namespaces = ClusterSudoPolicy.Spec.AllowedNamespaces
	} else if ClusterSudoPolicy.Spec.AllowedNamespacesSelector != nil {
		// Fetch namespaces based on selector
		var namespaceList corev1.NamespaceList
		selector, err := metav1.LabelSelectorAsSelector(ClusterSudoPolicy.Spec.AllowedNamespacesSelector)
		if err != nil {
			logger.Error(err, "Invalid label selector in ClusterSudoPolicy spec")
			return ctrl.Result{}, err
		}
		if err := r.List(ctx, &namespaceList, &client.ListOptions{LabelSelector: selector}); err != nil {
			logger.Error(err, "Failed to list namespaces")
			return ctrl.Result{}, err
		}
		for _, ns := range namespaceList.Items {
			namespaces = append(namespaces, ns.Name)
		}
	}
	// Check for mutual exclusivity of allowedNamespaces and allowedNamespacesSelector
	if ClusterSudoPolicy.Spec.AllowedNamespaces != nil && ClusterSudoPolicy.Spec.AllowedNamespacesSelector != nil {
		logger.Error(nil, "Both allowedNamespaces and allowedNamespacesSelector cannot be set simultaneously")
		return ctrl.Result{}, nil
	}
	if ClusterSudoPolicy.Spec.AllowedNamespaces == nil && ClusterSudoPolicy.Spec.AllowedNamespacesSelector == nil {
		logger.Error(nil, "Either allowedNamespaces or allowedNamespacesSelector must be set")
		return ctrl.Result{}, nil
	}

	// Update ClusterSudoPolicy status
	ClusterSudoPolicy.Status.State = "Active"
	ClusterSudoPolicy.Status.Namespaces = namespaces
	if err := r.Status().Update(ctx, &ClusterSudoPolicy); err != nil {
		logger.Error(err, "Failed to update ClusterSudoPolicy status")
		return ctrl.Result{}, err
	}

	logger.Info("Successfully validated ClusterSudoPolicy", "name", ClusterSudoPolicy.Name)

	if ClusterSudoPolicy.Spec.AllowedNamespacesSelector != nil {
		logger.Info("Rescheduling reconciliation due to dynamic AllowedNamespacesSelector", "name", ClusterSudoPolicy.Name)
		return ctrl.Result{RequeueAfter: ReconciliationInterval}, nil
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterSudoPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ClusterSudoPolicy{}).
		Complete(r)
}
