package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	tarbacv1 "github.com/guybal/tarbac/api/v1" // Adjust to match your actual module path
// 	utils "github.com/guybal/tarbac/utils"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apierrors "k8s.io/apimachinery/pkg/api/errors" // Import for IsAlreadyExists
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
//     "github.com/go-logr/logr"
)

func AddToScheme(scheme *runtime.Scheme) error {
    return tarbacv1.AddToScheme(scheme)
}

type TemporaryRBACReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile performs reconciliation for TemporaryRBAC objects
func (r *TemporaryRBACReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    currentTime := time.Now()
    logger := log.FromContext(ctx)
    logger.Info("Reconciling TemporaryRBAC", "namespace", req.Namespace, "name", req.Name)

    // Fetch the TemporaryRBAC object
    var tempRBAC tarbacv1.TemporaryRBAC
    if err := r.Get(ctx, req.NamespacedName, &tempRBAC); err != nil {
        if apierrors.IsNotFound(err) {
            logger.Info("TemporaryRBAC resource not found. Ignoring since it must have been deleted.")
            return ctrl.Result{}, nil
        }
        logger.Error(err, "Unable to fetch TemporaryRBAC")
        return ctrl.Result{}, err
    }

    // Parse the duration from the spec
    duration, err := time.ParseDuration(tempRBAC.Spec.Duration)
    if err != nil {
        logger.Error(err, "Invalid duration in TemporaryRBAC spec", "duration", tempRBAC.Spec.Duration)
        return ctrl.Result{}, err
    }

    if tempRBAC.Status.CreatedAt == nil || (tempRBAC.Status.ExpiresAt != nil && currentTime.Before(tempRBAC.Status.ExpiresAt.Time)) && currentTime.After(tempRBAC.Status.CreatedAt.Time) {
        // Ensure bindings are created and status is updated
        if err := r.ensureBindings(ctx, &tempRBAC, duration); err != nil {
            logger.Error(err, "Failed to ensure bindings for TemporaryRBAC")
            return ctrl.Result{}, err
        }
    }

    // Calculate expiration time if not already set
    if tempRBAC.Status.ExpiresAt == nil {
        expiration := tempRBAC.Status.CreatedAt.Time.Add(duration)
        tempRBAC.Status.ExpiresAt = &metav1.Time{Time: expiration}
        // Commit the status update to the API server
        if err := r.Status().Update(ctx, &tempRBAC); err != nil {
            logger.Error(err, "Failed to update TemporaryRBAC status with expiration date")
            return ctrl.Result{}, err
        }
    }

//     if tempRBAC.Status.RequestID == nil && hasOwnerRef {
//         //fetch owner which is either a clustersudorequest or a sudorequest
//         //fetch owner.status.requestID
//         // set tempRBAC.Status.RequestID to owner.status.requestID
//     }

    if len(tempRBAC.OwnerReferences) > 0 {
        if err := r.fetchAndSetRequestID(ctx, &tempRBAC); err != nil {
            logger.Error(err, "Failed to fetch and set RequestID", "TemporaryRBAC", tempRBAC.Name)
            return ctrl.Result{}, err
        }
    }

    // Check expiration status
    logger.Info("Checking expiration", "currentTime", currentTime, "expiresAt", tempRBAC.Status.ExpiresAt)

    if currentTime.After(tempRBAC.Status.ExpiresAt.Time) {
        logger.Info("TemporaryRBAC expired, cleaning up associated bindings", "name", tempRBAC.Name)

        // Cleanup expired bindings
        if err := r.cleanupBindings(ctx, &tempRBAC); err != nil {
            logger.Error(err, "Failed to clean up bindings for expired TemporaryRBAC")
            return ctrl.Result{}, err
        }
        return ctrl.Result{}, nil
    }

    // Calculate time until expiration
    timeUntilExpiration := time.Until(tempRBAC.Status.ExpiresAt.Time)
    logger.Info("TemporaryRBAC is still valid", "name", tempRBAC.Name, "timeUntilExpiration", timeUntilExpiration)

    // If expiration is very close, requeue with a smaller interval
    if timeUntilExpiration <= 1*time.Second {
        logger.Info("Requeueing closer to expiration for final check", "timeUntilExpiration", timeUntilExpiration)
        return ctrl.Result{RequeueAfter: 100 * time.Millisecond}, nil
    }

    // Requeue for regular reconciliation
    logger.Info("TemporaryRBAC successfully reconciled, requeueing for expiration", "name", tempRBAC.Name)
    return ctrl.Result{RequeueAfter: timeUntilExpiration.Truncate(time.Second)}, nil
}

// ensureBindings creates or updates the RoleBinding or ClusterRoleBinding for the TemporaryRBAC resource
func (r *TemporaryRBACReconciler) ensureBindings(ctx context.Context, tempRBAC *tarbacv1.TemporaryRBAC, duration time.Duration) error {
	logger := log.FromContext(ctx)

	// Collect subjects from both `spec.subject` and `spec.subjects`
	var subjects []rbacv1.Subject
    // 	if tempRBAC.Spec.Subject != (rbacv1.Subject{}) {
    // 		subjects = append(subjects, tempRBAC.Spec.Subject)
    // 	}
	if len(tempRBAC.Spec.Subjects) > 0 {
		subjects = append(subjects, tempRBAC.Spec.Subjects...)
	}

	if len(subjects) == 0 {
		logger.Error(nil, "No subjects specified in TemporaryRBAC")
		return fmt.Errorf("no subjects specified")
	}

	// Set CreatedAt if not already set
	if tempRBAC.Status.CreatedAt == nil {
		tempRBAC.Status.CreatedAt = &metav1.Time{Time: time.Now()}
	}

	var child_resources = []tarbacv1.ChildResource{}

	// Iterate over all subjects and create corresponding bindings
	for _, subject := range subjects {
        var binding client.Object
        // Convert RoleRefWithNamespace to rbacv1.RoleRef

        roleRef := rbacv1.RoleRef{
            APIGroup: tempRBAC.Spec.RoleRef.APIGroup,
            Kind:     tempRBAC.Spec.RoleRef.Kind,
            Name:     tempRBAC.Spec.RoleRef.Name,
        }

		// Generate the binding based on the RoleRef kind
		if tempRBAC.Spec.RoleRef.Kind == "ClusterRole" {
			binding = &rbacv1.RoleBinding{
                ObjectMeta: metav1.ObjectMeta{
                    Name:      generateBindingName(subject, roleRef),  // utils.GenerateBindingName
                    Namespace: tempRBAC.ObjectMeta.Namespace,
                    Labels: map[string]string{
                        "tarbac.io/owner": tempRBAC.Name,
                    },
                },
                Subjects: []rbacv1.Subject{subject},
                RoleRef:  roleRef,
            }
//             bindingKind = "RoleBinding"
            binding.GetObjectKind().SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("RoleBinding"))
 		} else if tempRBAC.Spec.RoleRef.Kind == "Role" {
            binding = &rbacv1.RoleBinding{
                ObjectMeta: metav1.ObjectMeta{
                    Name:      generateBindingName(subject, roleRef), // utils.GenerateBindingName
                    Namespace: tempRBAC.ObjectMeta.Namespace,
                    Labels: map[string]string{
                        "tarbac.io/owner": tempRBAC.Name,
                    },
                },
                Subjects: []rbacv1.Subject{subject},
                RoleRef:  roleRef,
            }
            binding.GetObjectKind().SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("RoleBinding"))
        } else {
			return fmt.Errorf("unsupported roleRef.kind: %s", tempRBAC.Spec.RoleRef.Kind)
		}

        // Set the OwnerReference on the RoleBinding
        if err := controllerutil.SetControllerReference(tempRBAC, binding, r.Scheme); err != nil {
            logger.Error(err, "Failed to set OwnerReference for RoleBinding", "RoleBinding", binding)
            return err
        }

		// Attempt to create the binding
		if err := r.Client.Create(ctx, binding); err != nil && !apierrors.IsAlreadyExists(err) {
			logger.Error(err, "Failed to create binding", "binding", binding)
			return err
		}

        child_resources = append(child_resources, tarbacv1.ChildResource{
            APIVersion: rbacv1.SchemeGroupVersion.String(), // binding.GetObjectKind().GroupVersionKind().GroupVersion().String(),
            Kind:       binding.GetObjectKind().GroupVersionKind().Kind,
            Name:       binding.GetName(),
            Namespace:  binding.GetNamespace(),
        })
	}

	// Update the status with the last created child resource
	tempRBAC.Status.ChildResource = child_resources
	tempRBAC.Status.State = "Created"

	// Commit the status update to the API server
	if err := r.Status().Update(ctx, tempRBAC); err != nil {
		logger.Error(err, "Failed to update TemporaryRBAC status after ensuring bindings")
		return err
	}

	logger.Info("Successfully ensured bindings and updated status", "TemporaryRBAC", tempRBAC.Name)
	return nil
}

// cleanupBindings deletes the RoleBinding or ClusterRoleBinding associated with the TemporaryRBAC resource
func (r *TemporaryRBACReconciler) cleanupBindings(ctx context.Context, tempRBAC *tarbacv1.TemporaryRBAC) error {
    logger := log.FromContext(ctx)
    var remainingChildResources []tarbacv1.ChildResource

    if len(tempRBAC.Status.ChildResource) > 0 {
        for _, child := range tempRBAC.Status.ChildResource {
            logger.Info("Cleaning up child resource", "kind", child.Kind, "name", child.Name, "namespace", child.Namespace)

            switch child.Kind {
            case "RoleBinding":
                // Ensure namespace is not empty
                if child.Namespace == "" {
                    return fmt.Errorf("namespace is empty for RoleBinding: %s", child.Name)
                }

                // Delete RoleBinding
                err := r.Client.Delete(ctx, &rbacv1.RoleBinding{
                    ObjectMeta: metav1.ObjectMeta{
                        Name:      child.Name,
                        Namespace: child.Namespace,
                    },
                })
                if err != nil && !apierrors.IsNotFound(err) {
                    logger.Error(err, "Failed to delete RoleBinding", "name", child.Name, "namespace", child.Namespace)
                    // Keep the resource in the list if deletion fails
                    remainingChildResources = append(remainingChildResources, child)
                    continue
                }
                logger.Info("Successfully deleted RoleBinding", "name", child.Name, "namespace", child.Namespace)

            default:
                logger.Error(fmt.Errorf("unsupported child resource kind"), "Unsupported child resource kind", "kind", child.Kind)
                remainingChildResources = append(remainingChildResources, child)
            }
        }
    }

    // Update the ChildResource slice after cleanup
    if len(remainingChildResources) == 0 {
        tempRBAC.Status.ChildResource = nil // Reset if all resources are deleted
    } else {
        tempRBAC.Status.ChildResource = remainingChildResources // Retain resources that couldn't be deleted
    }

    // Update the state if no child resources remain
    if tempRBAC.Status.ChildResource == nil {
        tempRBAC.Status.State = "Expired"
    }

    // Check DeletionPolicy
    if tempRBAC.Spec.RetentionPolicy == "delete" {
        logger.Info("RetentionPolicy is set to delete, deleting TemporaryRBAC resource", "name", tempRBAC.Name)
        if err := r.Client.Delete(ctx, tempRBAC); err != nil {
            logger.Error(err, "Failed to delete TemporaryRBAC resource")
            return err
        }
        return nil // Exit since resource is deleted
    }

    // Update status in Kubernetes
    if err := r.Status().Update(ctx, tempRBAC); err != nil {
        logger.Error(err, "Failed to update TemporaryRBAC status")
        return err
    }

    logger.Info("TemporaryRBAC status updated", "name", tempRBAC.Name, "state", tempRBAC.Status.State)
    return nil
}

// updateStatusWithRetry retries the status update to handle conflicts
func (r *TemporaryRBACReconciler) updateStatusWithRetry(ctx context.Context, tempRBAC *tarbacv1.TemporaryRBAC) error {
	logger := log.FromContext(ctx)

	for i := 0; i < 5; i++ {
		if err := r.Status().Update(ctx, tempRBAC); err != nil {
			if apierrors.IsConflict(err) {
				logger.Info("Conflict detected, retrying status update")
				if err := r.Get(ctx, client.ObjectKeyFromObject(tempRBAC), tempRBAC); err != nil {
					logger.Error(err, "Failed to fetch latest TemporaryRBAC resource during retry")
					return err
				}
				continue
			}
			return err
		}
		return nil
	}

	return fmt.Errorf("status update failed after retries for TemporaryRBAC %s", tempRBAC.Name)
}

func (r *TemporaryRBACReconciler) fetchAndSetRequestID(ctx context.Context, tempRBAC *tarbacv1.TemporaryRBAC) error {
    logger := log.FromContext(ctx)

    if tempRBAC.Status.RequestID == "" && len(tempRBAC.OwnerReferences) > 0 {
        logger.Info("TemporaryRBAC missing RequestID, attempting to fetch owner reference")

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
                tempRBAC.Status.RequestID = ownerRequestID
                if err := r.Status().Update(ctx, tempRBAC); err != nil {
                    logger.Error(err, "Failed to update TemporaryRBAC status with RequestID", "TemporaryRBAC", tempRBAC.Name)
                    return err
                }
                logger.Info("TemporaryRBAC status updated with RequestID", "RequestID", ownerRequestID)
                break
            }
        }
    }
    return nil
}



// generateBindingName generates a unique name for the binding
func generateBindingName(subject rbacv1.Subject, roleRef rbacv1.RoleRef) string {
	return fmt.Sprintf("%s-%s-%s", strings.ToLower(subject.Kind), subject.Name, roleRef.Name)
}

// SetupWithManager sets up the controller with the Manager
func (r *TemporaryRBACReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tarbacv1.TemporaryRBAC{}).
		Complete(r)
}
