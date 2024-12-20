package controllers

import (
	"context"
	"time"
	"fmt"

	v1 "github.com/guybal/tarbac/api/v1"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ClusterSudoRequestReconciler struct {
	client.Client
}

// Reconcile handles reconciliation for ClusterSudoRequest objects
func (r *ClusterSudoRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling ClusterSudoRequest", "name", req.Name)

	// Fetch the ClusterSudoRequest object
	var clusterSudoRequest v1.ClusterSudoRequest
	if err := r.Get(ctx, req.NamespacedName, &clusterSudoRequest); err != nil {
		logger.Error(err, "Unable to fetch ClusterSudoRequest")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Parse the duration
	duration, err := time.ParseDuration(clusterSudoRequest.Spec.Duration)
	if err != nil {
		logger.Error(err, "Invalid duration in ClusterSudoRequest spec", "duration", clusterSudoRequest.Spec.Duration)
		return ctrl.Result{}, err
	}

	// Set CreatedAt and ExpiresAt if not already set
	if clusterSudoRequest.Status.CreatedAt == nil {
// 		now := metav1.Now()
// 		clusterSudoRequest.Status.CreatedAt = &now
// 		clusterSudoRequest.Status.ExpiresAt = &metav1.Time{Time: now.Add(duration)}
		clusterSudoRequest.Status.State = "Pending"

		if err := r.Status().Update(ctx, &clusterSudoRequest); err != nil {
			logger.Error(err, "Failed to update ClusterSudoRequest status")
			return ctrl.Result{}, err
		}
        logger.Info(fmt.Sprintf("Set initial status for ClusterSudoRequest, requested duration: %s", duration.String()))
	}

	// Check if the ClusterSudoRequest has expired
	if clusterSudoRequest.Status.ExpiresAt != nil {

		if time.Now().After(clusterSudoRequest.Status.ExpiresAt.Time) {
            clusterSudoRequest.Status.State = "Expired"
            if err := r.Status().Update(ctx, &clusterSudoRequest); err != nil {
                logger.Error(err, "Failed to update expired ClusterSudoRequest status")
                return ctrl.Result{}, err
            }
            logger.Info("ClusterSudoRequest has expired", "name", clusterSudoRequest.Name)
            return ctrl.Result{}, nil
        }

        // Requeue until expiration
        timeUntilExpiration := time.Until(clusterSudoRequest.Status.ExpiresAt.Time)
        logger.Info("Requeueing for expiration check", "timeUntilExpiration", timeUntilExpiration)
        return ctrl.Result{RequeueAfter: timeUntilExpiration}, nil
	}
    return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterSudoRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ClusterSudoRequest{}).
		Complete(r)
}
