package controllers

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/guybal/tarbac/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type SudoRequestReconciler struct {
	client.Client
}

// Reconcile handles reconciliation for SudoRequest objects
func (r *SudoRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling SudoRequest", "name", req.Name, "namespace", req.Namespace)

	// Fetch the SudoRequest object
	var sudoRequest v1.SudoRequest
	if err := r.Get(ctx, req.NamespacedName, &sudoRequest); err != nil {
		logger.Error(err, "Unable to fetch SudoRequest")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Parse the duration
	duration, err := time.ParseDuration(sudoRequest.Spec.Duration)
	if err != nil {
		logger.Error(err, "Invalid duration in SudoRequest spec", "duration", sudoRequest.Spec.Duration)
		return ctrl.Result{}, err
	}

	// Set CreatedAt and ExpiresAt if not already set
	if sudoRequest.Status.CreatedAt == nil {
		now := metav1.Now()
		sudoRequest.Status.CreatedAt = &now
		sudoRequest.Status.ExpiresAt = &metav1.Time{Time: now.Add(duration)}
		sudoRequest.Status.State = "Pending"

		if err := r.Status().Update(ctx, &sudoRequest); err != nil {
			logger.Error(err, "Failed to update SudoRequest status")
			return ctrl.Result{}, err
		}
		logger.Info("Set initial status for SudoRequest")
	}

	// Check if the SudoRequest has expired
	if time.Now().After(sudoRequest.Status.ExpiresAt.Time) {
		sudoRequest.Status.State = "Expired"
		if err := r.Status().Update(ctx, &sudoRequest); err != nil {
			logger.Error(err, "Failed to update expired SudoRequest status")
			return ctrl.Result{}, err
		}
		logger.Info("SudoRequest has expired", "name", sudoRequest.Name)
		return ctrl.Result{}, nil
	}

	// Requeue until expiration
	timeUntilExpiration := time.Until(sudoRequest.Status.ExpiresAt.Time)
	logger.Info("Requeueing for expiration check", "timeUntilExpiration", timeUntilExpiration)
	return ctrl.Result{RequeueAfter: timeUntilExpiration}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SudoRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.SudoRequest{}).
		Complete(r)
}
