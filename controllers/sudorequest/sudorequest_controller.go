package controllers

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/guybal/tarbac/api/v1"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
    "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)
//
// func AddToScheme(scheme *runtime.Scheme) error {
//     return v1.AddToScheme(scheme)
// }
//
type SudoRequestReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile handles reconciliation for SudoRequest objects
func (r *SudoRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Reconciling SudoRequest", "name", req.Name, "namespace", req.Namespace)

	// Fetch the SudoRequest object
	var sudoRequest v1.SudoRequest
	if err := r.Get(ctx, req.NamespacedName, &sudoRequest); err != nil {
        if apierrors.IsNotFound(err) {
            logger.Info("SudoRequest resource not found. Ignoring since it must have been deleted.")
            return ctrl.Result{}, nil
        }
        logger.Error(err, "Unable to fetch SudoRequest")
        return ctrl.Result{}, nil
    }

    if sudoRequest.Status.State == "Expired" || sudoRequest.Status.State == "Rejected" {
        logger.Error(nil, "SudoRequest expired or rejected")
        return ctrl.Result{}, nil
    }

    requester := sudoRequest.Annotations["tarbac.io/requester"]

    // Check User
	if requester == "" {
		logger.Error(nil, "Requester annotation is missing")
		sudoRequest.Status.State = "Rejected"
		sudoRequest.Status.ErrorMessage = "Requester information is missing"
		r.Status().Update(ctx, &sudoRequest)
		return ctrl.Result{}, fmt.Errorf("missing requester annotation")
	}

    // Initial State
    if sudoRequest.Status.State == "" {
        sudoRequest.Status.State = "Pending"
        if err := r.Client.Status().Update(ctx, &sudoRequest); err != nil {
            logger.Error(err, "Failed to set initial 'Pending' status")
            return ctrl.Result{}, err
        }
    }

	// Parse the duration
	duration, err := time.ParseDuration(sudoRequest.Spec.Duration)
	if err != nil {
		logger.Error(err, "Invalid duration in SudoRequest spec", "duration", sudoRequest.Spec.Duration)
		return ctrl.Result{}, err
	}

    if sudoRequest.Status.ExpiresAt != nil {
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

    if sudoRequest.Status.State == "Pending" {
		logger.Info("Fetching SudoPolicy for SudoRequest", "policy", sudoRequest.Spec.Policy, "sudoRequest", sudoRequest.Name)

		// Fetch the referenced SudoPolicy
		var sudoPolicy v1.SudoPolicy
		if err := r.Get(ctx, client.ObjectKey{Namespace: sudoRequest.Namespace, Name: sudoRequest.Spec.Policy}, &sudoPolicy); err != nil {
			logger.Error(err, "Failed to fetch referenced SudoPolicy", "policy", sudoRequest.Spec.Policy)
			sudoRequest.Status.State = "Rejected"
			sudoRequest.Status.ErrorMessage = "Referenced policy not found"
			r.Status().Update(ctx, &sudoRequest)
			return ctrl.Result{}, err
		}

		// Check if the user is allowed
		allowed := false
		for _, user := range sudoPolicy.Spec.AllowedUsers {
			if user.Name == requester {
				allowed = true
				logger.Info("User approved by policy", "user", requester, "policy", sudoPolicy.Name)
				break
			}
		}

		if !allowed {
			sudoRequest.Status.State = "Rejected"
			sudoRequest.Status.ErrorMessage = "User not allowed by policy"
			r.Status().Update(ctx, &sudoRequest)
			logger.Info("SudoRequest rejected", "user", requester, "policy", sudoPolicy.Name)
			return ctrl.Result{}, nil
		}

		// Create TemporaryRBAC
        ctrl.Log.Info("Creating TemporaryRBAC", "roleRef", sudoPolicy.Spec.RoleRef)
		temporaryRBAC := &v1.TemporaryRBAC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("temporaryrbac-%s", sudoRequest.Name),
				Namespace: sudoRequest.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: sudoRequest.APIVersion,
						Kind:       sudoRequest.Kind,
						Name:       sudoRequest.Name,
						UID:        sudoRequest.UID,
					},
				},
			},
			Spec: v1.TemporaryRBACSpec{
				Subjects: []rbacv1.Subject{
					{
						Kind:      "User",
						Name:      requester,
						Namespace: sudoRequest.Namespace,
					},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: sudoPolicy.Spec.RoleRef.APIGroup,
					Kind:     sudoPolicy.Spec.RoleRef.Kind,
					Name:     sudoPolicy.Spec.RoleRef.Name,
				},
				Duration: sudoRequest.Spec.Duration,
			},
		}

        // Set OwnerReference on TemporaryRBAC
        if err := controllerutil.SetControllerReference(&sudoRequest, temporaryRBAC, r.Scheme); err != nil {
            logger.Error(err, "Failed to set OwnerReference for TemporaryRBAC", "TemporaryRBAC", temporaryRBAC)
            return ctrl.Result{}, err
        }

        // Create TemporaryRBAC
		if err := r.Create(ctx, temporaryRBAC); err != nil {
			logger.Error(err, "Failed to create TemporaryRBAC")
			sudoRequest.Status.State = "Error"
			sudoRequest.Status.ErrorMessage = "Failed to create TemporaryRBAC"
			r.Status().Update(ctx, &sudoRequest)
			return ctrl.Result{}, err
		}

		logger.Info("SudoRequest approved and TemporaryRBAC created", "name", sudoRequest.Name, "temporaryRBAC", temporaryRBAC.Name, "duration", duration)
// 		return ctrl.Result{}, nil

        // Fetch the TemporaryRBAC
        var updatedTemporaryRBAC v1.TemporaryRBAC
        if err := r.Get(ctx, client.ObjectKey{Name: temporaryRBAC.Name, Namespace: temporaryRBAC.Namespace}, &updatedTemporaryRBAC); err != nil {
            logger.Error(err, "Failed to fetch TemporaryRBAC", "name", temporaryRBAC.Name, "namespace", temporaryRBAC.Namespace)
            sudoRequest.Status.State = "Error"
            sudoRequest.Status.ErrorMessage = "Failed to fetch TemporaryRBAC status"
            r.Status().Update(ctx, &sudoRequest)
            return ctrl.Result{}, err
        }

        // Update SudoRequest status
        sudoRequest.Status.State = updatedTemporaryRBAC.Status.State
        sudoRequest.Status.ChildResource = []v1.ChildResource{
            {
                Kind:      "TemporaryRBAC",
                Name:      updatedTemporaryRBAC.Name,
                Namespace: updatedTemporaryRBAC.Namespace,
            },
        }
        sudoRequest.Status.CreatedAt = updatedTemporaryRBAC.Status.CreatedAt
        sudoRequest.Status.ExpiresAt = updatedTemporaryRBAC.Status.ExpiresAt

        if err := r.Status().Update(ctx, &sudoRequest); err != nil {
            logger.Error(err, "Failed to update SudoRequest status")
            return ctrl.Result{}, err
        }
        logger.Info("SudoRequest status updated with TemporaryRBAC details", "TemporaryRBAC", temporaryRBAC.Name)
	}
    return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SudoRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
    r.Scheme = mgr.GetScheme() // Initialize the Scheme field
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.SudoRequest{}).
		Complete(r)
}
