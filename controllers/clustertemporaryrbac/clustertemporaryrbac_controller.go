package controllers

import (
	"context"
	"fmt"
	"time"
// 	"strings"

	tarbacv1 "github.com/guybal/tarbac/api/v1" // Adjust to match your actual module path
    utils "github.com/guybal/tarbac/utils"
    rbacv1 "k8s.io/api/rbac/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
    apierrors "k8s.io/apimachinery/pkg/api/errors" // Import for IsAlreadyExists
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/log"
    "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type ClusterTemporaryRBACReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile performs reconciliation for ClusterTemporaryRBAC objects
func (r *ClusterTemporaryRBACReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	currentTime := time.Now()
	logger := log.FromContext(ctx)
    var requestId string
	logger.Info("Reconciling ClusterTemporaryRBAC", "name", req.Name, "namespace", req.Namespace)

	// Fetch the ClusterTemporaryRBAC object (cluster-scoped, so no namespace)
    var clusterTempRBAC tarbacv1.ClusterTemporaryRBAC
    if err := r.Get(ctx, client.ObjectKey{Name: req.Name}, &clusterTempRBAC); err != nil {
        if apierrors.IsNotFound(err) {
            logger.Info("ClusterTemporaryRBAC resource not found. Ignoring since it must have been deleted.")
            return ctrl.Result{}, nil
        }
        logger.Error(err, "Unable to fetch ClusterTemporaryRBAC")
        return ctrl.Result{}, err
    }

	// Parse the duration from the spec
	duration, err := time.ParseDuration(clusterTempRBAC.Spec.Duration)
	if err != nil {
		logger.Error(err, "Invalid duration in ClusterTemporaryRBAC spec", "duration", clusterTempRBAC.Spec.Duration)
		return ctrl.Result{}, err
	}

    if len(clusterTempRBAC.OwnerReferences) > 0 {
        if clusterTempRBAC.Status.RequestID == "" {
            if err := r.fetchAndSetRequestID(ctx, &clusterTempRBAC); err != nil {
                logger.Error(err, "Failed to fetch and set RequestID", "ClusterTemporaryRBAC", clusterTempRBAC.Name)
                return ctrl.Result{}, err
            }
        }
    } else {
        if clusterTempRBAC.Status.RequestID == "" {
            clusterTempRBAC.Status.RequestID = string(clusterTempRBAC.ObjectMeta.UID)
            if err := r.Status().Update(ctx, &clusterTempRBAC); err != nil {
                logger.Error(err, "Failed to update ClusterTemporaryRBAC status")
                return ctrl.Result{}, err
            }
        }
    }
    requestId = clusterTempRBAC.Status.RequestID

	if clusterTempRBAC.Status.CreatedAt == nil {
		// Ensure bindings are created and status is updated
		if err := r.ensureBindings(ctx, &clusterTempRBAC, duration, requestId); err != nil {
			logger.Error(err, "Failed to ensure bindings for ClusterTemporaryRBAC")
			return ctrl.Result{}, err
		}
	}

	// Calculate expiration time if not already set
	if clusterTempRBAC.Status.ExpiresAt == nil {
		expiration := clusterTempRBAC.Status.CreatedAt.Time.Add(duration)
		clusterTempRBAC.Status.ExpiresAt = &metav1.Time{Time: expiration}
		// Commit the status update to the API server
		if err := r.Status().Update(ctx, &clusterTempRBAC); err != nil {
			logger.Error(err, "Failed to update ClusterTemporaryRBAC status with expiration date")
			return ctrl.Result{}, err
		}
	}

//     if len(clusterTempRBAC.OwnerReferences) > 0 {
//         if err := r.fetchAndSetRequestID(ctx, &clusterTempRBAC); err != nil {
//             logger.Error(err, "Failed to fetch and set RequestID", "ClusterTemporaryRBAC", clusterTempRBAC.Name)
//             return ctrl.Result{}, err
//         }
//     }

	// Check expiration status
	logger.Info("Checking expiration", "currentTime", currentTime, "expiresAt", clusterTempRBAC.Status.ExpiresAt)

	if currentTime.After(clusterTempRBAC.Status.ExpiresAt.Time) {
		logger.Info("ClusterTemporaryRBAC expired, cleaning up associated bindings", "name", clusterTempRBAC.Name)

		// Cleanup expired bindings
		if err := r.cleanupBindings(ctx, &clusterTempRBAC); err != nil {
			logger.Error(err, "Failed to clean up bindings for expired ClusterTemporaryRBAC")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Calculate time until expiration
	timeUntilExpiration := time.Until(clusterTempRBAC.Status.ExpiresAt.Time)
	logger.Info("ClusterTemporaryRBAC is still valid", "name", clusterTempRBAC.Name, "timeUntilExpiration", timeUntilExpiration)

	// If expiration is very close, requeue with a smaller interval
	if timeUntilExpiration <= 1*time.Second {
		logger.Info("Requeueing closer to expiration for final check", "timeUntilExpiration", timeUntilExpiration)
		return ctrl.Result{RequeueAfter: 100 * time.Millisecond}, nil
	}

	// Requeue for regular reconciliation
	logger.Info("ClusterTemporaryRBAC successfully reconciled, requeueing for expiration", "name", clusterTempRBAC.Name)
	return ctrl.Result{RequeueAfter: timeUntilExpiration.Truncate(time.Second)}, nil
}

// ensureBindings creates ClusterRoleBindings for the ClusterTemporaryRBAC resource
func (r *ClusterTemporaryRBACReconciler) ensureBindings(ctx context.Context, clusterTempRBAC *tarbacv1.ClusterTemporaryRBAC, duration time.Duration, requestId string) error {
	logger := log.FromContext(ctx)

	var subjects []rbacv1.Subject
	if len(clusterTempRBAC.Spec.Subjects) > 0 {
		subjects = append(subjects, clusterTempRBAC.Spec.Subjects...)
	}

	if len(subjects) == 0 {
		logger.Error(nil, "No subjects specified in ClusterTemporaryRBAC")
		return fmt.Errorf("no subjects specified")
	}

	// Set CreatedAt if not already set
	if clusterTempRBAC.Status.CreatedAt == nil {
		clusterTempRBAC.Status.CreatedAt = &metav1.Time{Time: time.Now()}
	}

	var childResources = []tarbacv1.ChildResource{}

	for _, subject := range subjects {
		roleBinding := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: utils.GenerateBindingName(subject, clusterTempRBAC.Spec.RoleRef, requestId), //generateBindingName(subject, clusterTempRBAC.Spec.RoleRef),
				Labels: map[string]string{
					"tarbac.io/owner": clusterTempRBAC.Name,
					"tarbac.io/request-id": requestId,
				},
			},
			Subjects: []rbacv1.Subject{subject},
			RoleRef: rbacv1.RoleRef{
				APIGroup: clusterTempRBAC.Spec.RoleRef.APIGroup,
                Kind:     clusterTempRBAC.Spec.RoleRef.Kind,
                Name:     clusterTempRBAC.Spec.RoleRef.Name,
			},
		}

		// Set the OwnerReference on the ClusterRoleBinding
		if err := controllerutil.SetControllerReference(clusterTempRBAC, roleBinding, r.Scheme); err != nil {
			logger.Error(err, "Failed to set OwnerReference for ClusterRoleBinding", "ClusterRoleBinding", roleBinding.Name)
			return err
		}

		// Create the ClusterRoleBinding
		if err := r.Client.Create(ctx, roleBinding); err != nil && !apierrors.IsAlreadyExists(err) {
			logger.Error(err, "Failed to create ClusterRoleBinding", "ClusterRoleBinding", roleBinding.Name)
			return err
		}

        // Add to childResources with proper Kind and APIVersion
		childResources = append(childResources, tarbacv1.ChildResource{
			APIVersion: rbacv1.SchemeGroupVersion.String(), // Correctly set the APIVersion
			Kind:       "ClusterRoleBinding",               // Correctly set the Kind
			Name:       roleBinding.GetName(),
		})

		logger.Info("Successfully created ClusterRoleBinding with OwnerReference", "ClusterRoleBinding", roleBinding.Name)
	}

	// Update the status with created child resources
	clusterTempRBAC.Status.ChildResource = childResources
	clusterTempRBAC.Status.State = "Created"

	if err := r.Status().Update(ctx, clusterTempRBAC); err != nil {
		logger.Error(err, "Failed to update ClusterTemporaryRBAC status after ensuring bindings")
		return err
	}

	logger.Info("Successfully ensured bindings and updated status", "ClusterTemporaryRBAC", clusterTempRBAC.Name)
	return nil
}

// cleanupBindings deletes the ClusterRoleBindings associated with the ClusterTemporaryRBAC resource
func (r *ClusterTemporaryRBACReconciler) cleanupBindings(ctx context.Context, clusterTempRBAC *tarbacv1.ClusterTemporaryRBAC) error {
	logger := log.FromContext(ctx)
	var remainingChildResources []tarbacv1.ChildResource

	for _, child := range clusterTempRBAC.Status.ChildResource {
		logger.Info("Cleaning up child resource", "kind", child.Kind, "name", child.Name)

		if child.Kind == "ClusterRoleBinding" {
			// Delete ClusterRoleBinding
			if err := r.Client.Delete(ctx, &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: child.Name,
				},
			}); err != nil && !apierrors.IsNotFound(err) {
				logger.Error(err, "Failed to delete ClusterRoleBinding", "name", child.Name)
				remainingChildResources = append(remainingChildResources, child)
				continue
			}
			logger.Info("Successfully deleted ClusterRoleBinding", "name", child.Name)
		} else {
			logger.Error(fmt.Errorf("unsupported child resource kind"), "Unsupported child resource kind", "kind", child.Kind)
			remainingChildResources = append(remainingChildResources, child)
		}
	}

	// Update the ChildResource slice after cleanup
	if len(remainingChildResources) == 0 {
		clusterTempRBAC.Status.ChildResource = nil // Reset if all resources are deleted
	} else {
		clusterTempRBAC.Status.ChildResource = remainingChildResources
	}

    // Update the state if no child resources remain
    if clusterTempRBAC.Status.ChildResource == nil {
        clusterTempRBAC.Status.State = "Expired"
    }

	// Check RetentionPolicy
	if clusterTempRBAC.Spec.RetentionPolicy == "delete" && clusterTempRBAC.Status.ChildResource == nil {
		logger.Info("RetentionPolicy is set to delete, deleting ClusterTemporaryRBAC resource", "name", clusterTempRBAC.Name)
		if err := r.Client.Delete(ctx, clusterTempRBAC); err != nil {
			logger.Error(err, "Failed to delete ClusterTemporaryRBAC resource")
			return err
		}
		return nil // Exit since resource is deleted
	}

	if err := r.Status().Update(ctx, clusterTempRBAC); err != nil {
		logger.Error(err, "Failed to update ClusterTemporaryRBAC status")
		return err
	}

	logger.Info("ClusterTemporaryRBAC status updated", "name", clusterTempRBAC.Name, "state", clusterTempRBAC.Status.State)
	return nil
}

func (r *ClusterTemporaryRBACReconciler) fetchAndSetRequestID(ctx context.Context, tempRBAC *tarbacv1.ClusterTemporaryRBAC) error {
    logger := log.FromContext(ctx)

    if tempRBAC.Status.RequestID == "" && len(tempRBAC.OwnerReferences) > 0 {
        logger.Info("ClusterTemporaryRBAC missing RequestID, attempting to fetch owner reference")

        // Loop through owner references
        for _, ownerRef := range tempRBAC.OwnerReferences {
            var ownerRequestID string

            switch ownerRef.Kind {
            case "ClusterSudoRequest":
                var clusterSudoRequest tarbacv1.ClusterSudoRequest
                err := r.Client.Get(ctx, client.ObjectKey{Name: ownerRef.Name}, &clusterSudoRequest)
                if err != nil {
                    logger.Error(err, "Failed to fetch ClusterSudoRequest", "ownerRef", ownerRef.Name)
                    continue
                }
                ownerRequestID = clusterSudoRequest.Status.RequestID
            case "SudoRequest":
                var sudoRequest tarbacv1.SudoRequest
                err := r.Client.Get(ctx, client.ObjectKey{Name: ownerRef.Name, Namespace: tempRBAC.Namespace}, &sudoRequest)
                if err != nil {
                    logger.Error(err, "Failed to fetch SudoRequest", "ownerRef", ownerRef.Name)
                    continue
                }
                ownerRequestID = sudoRequest.Status.RequestID
            default:
                logger.Info("Unsupported owner reference kind, skipping", "kind", ownerRef.Kind)
                continue
            }

            // Update the RequestID if found
            if ownerRequestID != "" {
                // Add a label with the RequestID
                if tempRBAC.Labels == nil {
                    tempRBAC.Labels = make(map[string]string)
                }
                tempRBAC.Labels["tarbac.io/request-id"] = ownerRequestID

                // Update the object with the new label
                if err := r.Update(ctx, tempRBAC); err != nil {
                    logger.Error(err, "Failed to update ClusterTemporaryRBAC labels with RequestID", "ClusterTemporaryRBAC", tempRBAC.Name)
                    return err
                }

                tempRBAC.Status.RequestID = ownerRequestID
                if err := r.Status().Update(ctx, tempRBAC); err != nil {
                    logger.Error(err, "Failed to update ClusterTemporaryRBAC status with RequestID", "ClusterTemporaryRBAC", tempRBAC.Name)
                    return err
                }
                logger.Info("ClusterTemporaryRBAC status updated with RequestID", "RequestID", ownerRequestID)
                break
            }
        }
    }
    return nil
}

// func generateBindingName(subject rbacv1.Subject, roleRef rbacv1.RoleRef) string {
// 	return fmt.Sprintf("%s-%s-%s", strings.ToLower(subject.Kind), subject.Name, roleRef.Name)
// }

func (r *ClusterTemporaryRBACReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tarbacv1.ClusterTemporaryRBAC{}).
		Complete(r)
}