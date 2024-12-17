package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	tarbacv1 "github.com/guybal/tarbac/api/v1" // Adjust to match your actual module path
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apierrors "k8s.io/apimachinery/pkg/api/errors" // Import for IsAlreadyExists
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

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
	if tempRBAC.Spec.Subject != (rbacv1.Subject{}) {
		subjects = append(subjects, tempRBAC.Spec.Subject)
	}
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

	var lastChildResource *tarbacv1.ChildResource

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
		if tempRBAC.Spec.RoleRef.Kind == "ClusterRole" && tempRBAC.Spec.RoleRef.Namespace != "" {
			binding = &rbacv1.RoleBinding{
                ObjectMeta: metav1.ObjectMeta{
                    Name:      generateBindingName(subject, roleRef),
                    Namespace: tempRBAC.Spec.RoleRef.Namespace,
                    Labels: map[string]string{
                        "tarbac.io/owner": tempRBAC.Name,
                    },
                },
                Subjects: []rbacv1.Subject{subject},
                RoleRef:  roleRef,
            }
//             bindingKind = "RoleBinding"
            binding.GetObjectKind().SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("RoleBinding"))
		} else if tempRBAC.Spec.RoleRef.Kind == "ClusterRole" {
			binding = &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: generateBindingName(subject, roleRef),
					Labels: map[string]string{
						"tarbac.io/owner": tempRBAC.Name,
					},
				},
				Subjects: []rbacv1.Subject{subject},
				RoleRef:  roleRef,
			}
			binding.GetObjectKind().SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("ClusterRoleBinding"))
		} else if tempRBAC.Spec.RoleRef.Kind == "Role" {
            binding = &rbacv1.RoleBinding{
                ObjectMeta: metav1.ObjectMeta{
                    Name:      generateBindingName(subject, roleRef),
                    Namespace: tempRBAC.Spec.RoleRef.Namespace,
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

		// Attempt to create the binding
		if err := r.Client.Create(ctx, binding); err != nil && !apierrors.IsAlreadyExists(err) {
			logger.Error(err, "Failed to create binding", "binding", binding)
			return err
		}

		// Record the last binding for status update
		lastChildResource = &tarbacv1.ChildResource{
			APIVersion: binding.GetObjectKind().GroupVersionKind().GroupVersion().String(),
			Kind:       binding.GetObjectKind().GroupVersionKind().Kind,
			Name:       binding.GetName(),
			Namespace:  tempRBAC.Spec.RoleRef.Namespace,
		}
	}

	// Update the status with the last created child resource
	tempRBAC.Status.ChildResource = lastChildResource
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

    if tempRBAC.Status.ChildResource != nil {

        logger.Info("Cleaning up child resource", "kind", tempRBAC.Status.ChildResource.Kind, "name", tempRBAC.Status.ChildResource.Name, "namespace", tempRBAC.Status.ChildResource.Namespace)

        switch tempRBAC.Status.ChildResource.Kind {
        case "ClusterRoleBinding":
            // Delete ClusterRoleBinding
            err := r.Client.Delete(ctx, &rbacv1.ClusterRoleBinding{
                ObjectMeta: metav1.ObjectMeta{Name: tempRBAC.Status.ChildResource.Name},
            })
            if err != nil && !apierrors.IsNotFound(err) {
                logger.Error(err, "Failed to delete ClusterRoleBinding", "name", tempRBAC.Status.ChildResource.Name)
                return err
            }
            logger.Info("Successfully deleted ClusterRoleBinding", "name", tempRBAC.Status.ChildResource.Name)

        case "RoleBinding":
            // Ensure namespace is not empty
            if tempRBAC.Status.ChildResource.Namespace == "" {
                return fmt.Errorf("namespace is empty for RoleBinding: %s", tempRBAC.Status.ChildResource.Name)
            }
            // Delete RoleBinding
            err := r.Client.Delete(ctx, &rbacv1.RoleBinding{
                ObjectMeta: metav1.ObjectMeta{
                    Name:      tempRBAC.Status.ChildResource.Name,
                    Namespace: tempRBAC.Status.ChildResource.Namespace,
                },
            })
            if err != nil && !apierrors.IsNotFound(err) {
                logger.Error(err, "Failed to delete RoleBinding", "name", tempRBAC.Status.ChildResource.Name, "namespace", tempRBAC.Status.ChildResource.Namespace)
                return err
            }
            logger.Info("Successfully deleted RoleBinding", "name", tempRBAC.Status.ChildResource.Name, "namespace", tempRBAC.Status.ChildResource.Namespace)

        default:
            logger.Error(fmt.Errorf("unsupported child resource kind"), "Unsupported child resource kind", "kind", tempRBAC.Status.ChildResource.Kind)
            return fmt.Errorf("unsupported child resource kind: %s", tempRBAC.Status.ChildResource.Kind)
        }
    }

    // Reset TemporaryRBAC status
    tempRBAC.Status.ChildResource = nil
    tempRBAC.Status.State = "Expired"

    // Update status in Kubernetes
    if err := r.Status().Update(ctx, tempRBAC); err != nil {
        logger.Error(err, "Failed to update TemporaryRBAC status to Expired")
        return err
    }
    logger.Info("TemporaryRBAC status updated to Expired", "name", tempRBAC.Name)
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
