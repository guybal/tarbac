package controllers

import (
	"context"
	"time"

	v1 "github.com/guybal/tarbac/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
    apierrors "k8s.io/apimachinery/pkg/api/errors"
)

type SudoPolicyReconciler struct {
	client.Client
}

// Reconcile handles reconciliation for SudoPolicy objects
func (r *SudoPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling SudoPolicy", "name", req.Name, "namespace", req.Namespace)

	// Fetch the SudoPolicy object
	var sudoPolicy v1.SudoPolicy
	if err := r.Get(ctx, req.NamespacedName, &sudoPolicy); err != nil {
		if apierrors.IsNotFound(err) {
            logger.Info("SudoPolicy resource not found. Ignoring since it must have been deleted.")
            return ctrl.Result{}, nil
        }
		logger.Error(err, "Unable to fetch SudoPolicy")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Validate maxDuration
	if _, err := time.ParseDuration(sudoPolicy.Spec.MaxDuration); err != nil {
		logger.Error(err, "Invalid maxDuration in SudoPolicy spec", "maxDuration", sudoPolicy.Spec.MaxDuration)
		return ctrl.Result{}, err
	}

	// Update SudoPolicy status
	sudoPolicy.Status.State = "Active"
	if err := r.Status().Update(ctx, &sudoPolicy); err != nil {
		logger.Error(err, "Failed to update SudoPolicy status")
		return ctrl.Result{}, err
	}

	logger.Info("Successfully validated SudoPolicy", "name", sudoPolicy.Name)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SudoPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.SudoPolicy{}).
		Complete(r)
}