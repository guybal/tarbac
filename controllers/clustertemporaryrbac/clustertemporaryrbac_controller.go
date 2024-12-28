package controllers

import (
	"context"
	"fmt"
	"time"
	"strings"

	tarbacv1 "github.com/guybal/tarbac/api/v1" // Adjust to match your actual module path
//     utils "github.com/guybal/tarbac/utils"
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
//             if err := r.Status().Update(ctx, clusterTempRBAC); err != nil {
//                 logger.Error(err, "Failed to update ClusterTemporaryRBAC status")
//                 return ctrl.Result{}, err
//             }
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
				Name: generateBindingName(subject, clusterTempRBAC.Spec.RoleRef),
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

func generateBindingName(subject rbacv1.Subject, roleRef rbacv1.RoleRef) string {
	return fmt.Sprintf("%s-%s-%s", strings.ToLower(subject.Kind), subject.Name, roleRef.Name)
}

func (r *ClusterTemporaryRBACReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tarbacv1.ClusterTemporaryRBAC{}).
		Complete(r)
}

// package controllers
//
// import (
// 	"context"
// 	"fmt"
// 	"strings"
// 	"time"
//
// 	tarbacv1 "github.com/guybal/tarbac/api/v1"
// 	rbacv1 "k8s.io/api/rbac/v1"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"k8s.io/apimachinery/pkg/runtime"
// 	apierrors "k8s.io/apimachinery/pkg/api/errors"
// 	ctrl "sigs.k8s.io/controller-runtime"
// 	"sigs.k8s.io/controller-runtime/pkg/client"
// 	"sigs.k8s.io/controller-runtime/pkg/log"
// 	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
// 	"github.com/go-logr/logr"
// )
//
// type ClusterTemporaryRBACReconciler struct {
// 	client.Client
// 	Scheme *runtime.Scheme
// }
//
// // Reconcile performs reconciliation for ClusterTemporaryRBAC objects
// func (r *ClusterTemporaryRBACReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
// 	logger := log.FromContext(ctx)
// 	logger.Info("Reconciling ClusterTemporaryRBAC", "name", req.Name)
//
// 	defer func() {
// 		if rec := recover(); rec != nil {
// 			logger.Error(fmt.Errorf("%v", rec), "Recovered from panic during reconciliation")
// 		}
// 	}()
//
// 	// Fetch the ClusterTemporaryRBAC resource
// // 	clusterTempRBAC, err := r.fetchClusterTemporaryRBAC(ctx, req, logger)
// // 	if err != nil {
// // 		logger.Error(err, "Failed to fetch ClusterTemporaryRBAC")
// // 		return ctrl.Result{}, err
// // 	}
// // 	if clusterTempRBAC == nil {
// // 		logger.Info("ClusterTemporaryRBAC resource not found, possibly deleted")
// // 		return ctrl.Result{}, nil
// // 	}
//     // Fetch the ClusterTemporaryRBAC object
//     var clusterTempRBAC tarbacv1.ClusterTemporaryRBAC
//     logger.Info("Fetching ClusterTemporaryRBAC")
//     if err := r.Get(ctx, req.NamespacedName, &clusterTempRBAC); err != nil {
// //               r.Get(ctx, client.ObjectKey{Name: clusterSudoRequest.Spec.Policy}, &clusterSudoPolicy)
//         logger.Info("Failed to fetch ClusterTemporaryRBAC")
//         if apierrors.IsNotFound(err) {
//             logger.Info("ClusterTemporaryRBAC resource not found. Ignoring since it must have been deleted.")
//             return ctrl.Result{}, nil
//         }
//         logger.Error(err, "Unable to fetch ClusterTemporaryRBAC")
//         return ctrl.Result{}, err
//     }
//
// 	// Validate Spec
// 	if len(clusterTempRBAC.Spec.Subjects) == 0 || clusterTempRBAC.Spec.RoleRef.Name == "" {
// 		logger.Error(nil, "Invalid spec: missing subjects or roleRef")
// 		return ctrl.Result{}, fmt.Errorf("invalid spec: missing subjects or roleRef")
// 	}
//
// 	// Verify ClusterRole existence
// 	var clusterRole rbacv1.ClusterRole
// 	if err := r.Get(ctx, client.ObjectKey{Name: clusterTempRBAC.Spec.RoleRef.Name}, &clusterRole); err != nil {
// 		if apierrors.IsNotFound(err) {
// 			logger.Error(err, "ClusterRole not found", "roleRef", clusterTempRBAC.Spec.RoleRef.Name)
// 			return ctrl.Result{}, fmt.Errorf("ClusterRole not found")
// 		}
// 		logger.Error(err, "Failed to fetch ClusterRole")
// 		return ctrl.Result{}, err
// 	}
// 	logger.Info("Verified ClusterRole exists", "roleRef", clusterTempRBAC.Spec.RoleRef.Name)
//
// 	// Parse Duration
// 	duration, err := r.parseDuration(clusterTempRBAC.Spec.Duration, logger)
// 	if err != nil {
// 		logger.Error(err, "Failed to parse duration")
// 		return ctrl.Result{}, err
// 	}
//
// 	// Initialize Status
// 	if err := r.initializeStatus(ctx, &clusterTempRBAC, logger); err != nil {
// 		logger.Error(err, "Failed to initialize status")
// 		return ctrl.Result{}, err
// 	}
//
// 	// Handle Creation
// 	if clusterTempRBAC.Status.State == "Pending" {
// 		if err := r.handleCreation(ctx, &clusterTempRBAC, duration, logger); err != nil {
// 			logger.Error(err, "Failed to handle creation")
// 			return ctrl.Result{}, err
// 		}
// 	}
//
// 	// Handle Expiration
// 	if err := r.handleExpiration(ctx, &clusterTempRBAC, logger); err != nil {
// 		logger.Error(err, "Failed to handle expiration")
// 		return ctrl.Result{}, err
// 	}
//
// 	// Schedule Requeue
// 	requeueResult := r.scheduleRequeue(&clusterTempRBAC, logger)
// 	logger.Info("Reconciliation complete", "requeueAfter", requeueResult.RequeueAfter)
// 	return requeueResult, nil
// }
//
// // fetchClusterTemporaryRBAC retrieves the ClusterTemporaryRBAC resource
// func (r *ClusterTemporaryRBACReconciler) fetchClusterTemporaryRBAC(ctx context.Context, req ctrl.Request, logger logr.Logger) (*tarbacv1.ClusterTemporaryRBAC, error) {
//     var clusterTempRBAC tarbacv1.ClusterTemporaryRBAC
//     // Explicitly use only the name since the resource is cluster-scoped
//     logger.Info("Fetching ClusterTemporaryRBAC")
//     objectKey := client.ObjectKey{Name: req.Name}
//
//     if err := r.Get(ctx, objectKey, &clusterTempRBAC); err != nil {
//         if apierrors.IsNotFound(err) {
//             logger.Info("ClusterTemporaryRBAC resource not found. Ignoring since it must have been deleted.")
//             return nil, nil
//         }
//         logger.Error(err, "Unable to fetch ClusterTemporaryRBAC")
//         return nil, err
//     }
//     logger.Info("Resource Spec", "resource", clusterTempRBAC.Spec)
//     return &clusterTempRBAC, nil
// }
//
//
// // parseDuration validates and parses the duration string
// func (r *ClusterTemporaryRBACReconciler) parseDuration(durationStr string, logger logr.Logger) (time.Duration, error) {
// 	duration, err := time.ParseDuration(durationStr)
// 	if err != nil {
// 		logger.Error(err, "Invalid duration in ClusterTemporaryRBAC spec", "duration", durationStr)
// 		return 0, err
// 	}
// 	return duration, nil
// }
//
// // initializeStatus ensures that the status fields are initialized
// func (r *ClusterTemporaryRBACReconciler) initializeStatus(ctx context.Context, clusterTempRBAC *tarbacv1.ClusterTemporaryRBAC, logger logr.Logger) error {
// 	logger.Info("Initializing ClusterTemporaryRBAC status", "name", clusterTempRBAC.Name)
//
// 	if clusterTempRBAC.Status.CreatedAt == nil {
// 		clusterTempRBAC.Status.CreatedAt = &metav1.Time{Time: time.Now()}
// 	}
// 	if clusterTempRBAC.Status.State == "" {
// 		clusterTempRBAC.Status.State = "Pending"
// 	}
//
// 	if err := r.Status().Update(ctx, clusterTempRBAC); err != nil {
// 		logger.Error(err, "Failed to initialize ClusterTemporaryRBAC status")
// 		return err
// 	}
//
// 	return nil
// }
//
// // handleCreation ensures bindings and initializes the status
// func (r *ClusterTemporaryRBACReconciler) handleCreation(ctx context.Context, clusterTempRBAC *tarbacv1.ClusterTemporaryRBAC, duration time.Duration, logger logr.Logger) error {
// 	logger.Info("Handling ClusterTemporaryRBAC creation", "name", clusterTempRBAC.Name)
//
// 	if err := r.ensureBindings(ctx, clusterTempRBAC, logger); err != nil {
// 		return fmt.Errorf("failed to ensure bindings: %w", err)
// 	}
//
// 	// Set expiration time
// 	expiration := time.Now().Add(duration)
// 	clusterTempRBAC.Status.ExpiresAt = &metav1.Time{Time: expiration}
// 	clusterTempRBAC.Status.State = "Created"
//
// 	if err := r.Status().Update(ctx, clusterTempRBAC); err != nil {
// 		logger.Error(err, "Failed to update ClusterTemporaryRBAC status with expiration date")
// 		return err
// 	}
//
// 	logger.Info("ClusterTemporaryRBAC created successfully", "name", clusterTempRBAC.Name, "expiresAt", expiration)
// 	return nil
// }
//
// // ensureBindings creates the necessary ClusterRoleBindings
// func (r *ClusterTemporaryRBACReconciler) ensureBindings(ctx context.Context, clusterTempRBAC *tarbacv1.ClusterTemporaryRBAC, logger logr.Logger) error {
// 	logger.Info("Ensuring bindings for ClusterTemporaryRBAC", "name", clusterTempRBAC.Name)
//
// 	if len(clusterTempRBAC.Spec.Subjects) == 0 {
// 		return fmt.Errorf("no subjects specified in ClusterTemporaryRBAC")
// 	}
//
// 	var childResources []tarbacv1.ChildResource
// 	for _, subject := range clusterTempRBAC.Spec.Subjects {
// 		roleBindingName := generateBindingName(subject, clusterTempRBAC.Spec.RoleRef)
// 		roleBinding := &rbacv1.ClusterRoleBinding{
// 			ObjectMeta: metav1.ObjectMeta{
// 				Name: roleBindingName,
// 				Labels: map[string]string{
// 					"tarbac.io/owner": clusterTempRBAC.Name,
// 				},
// 			},
// 			Subjects: []rbacv1.Subject{subject},
// 			RoleRef:  clusterTempRBAC.Spec.RoleRef,
// 		}
//
// 		if err := controllerutil.SetControllerReference(clusterTempRBAC, roleBinding, r.Scheme); err != nil {
// 			logger.Error(err, "Failed to set OwnerReference for ClusterRoleBinding", "ClusterRoleBinding", roleBinding.Name)
// 			return err
// 		}
//
// 		if err := r.Client.Create(ctx, roleBinding); err != nil && !apierrors.IsAlreadyExists(err) {
// 			logger.Error(err, "Failed to create ClusterRoleBinding", "ClusterRoleBinding", roleBinding.Name)
// 			return err
// 		}
//
// 		childResources = append(childResources, tarbacv1.ChildResource{
// 			APIVersion: rbacv1.SchemeGroupVersion.String(),
// 			Kind:       "ClusterRoleBinding",
// 			Name:       roleBinding.Name,
// 		})
//
// 		if err := r.Client.Create(ctx, roleBinding); err != nil {
//         	if apierrors.IsAlreadyExists(err) {
//         		logger.Info("ClusterRoleBinding already exists", "ClusterRoleBinding", roleBinding.Name)
//         	} else {
//         		logger.Error(err, "Failed to create ClusterRoleBinding", "ClusterRoleBinding", roleBinding.Name)
//         		return fmt.Errorf("failed to create ClusterRoleBinding: %w", err)
//         	}
//         }
//
// 		logger.Info("ClusterRoleBinding created successfully", "ClusterRoleBinding", roleBinding.Name)
// 	}
//
// 	clusterTempRBAC.Status.ChildResource = childResources
//
// 	return nil
// }
//
// // handleExpiration checks if the resource is expired and performs cleanup
// func (r *ClusterTemporaryRBACReconciler) handleExpiration(ctx context.Context, clusterTempRBAC *tarbacv1.ClusterTemporaryRBAC, logger logr.Logger) error {
// 	currentTime := time.Now()
// 	if currentTime.After(clusterTempRBAC.Status.ExpiresAt.Time) {
// 		logger.Info("ClusterTemporaryRBAC expired, cleaning up associated bindings", "name", clusterTempRBAC.Name)
//
// 		if err := r.cleanupBindings(ctx, clusterTempRBAC, logger); err != nil {
// 			logger.Error(err, "Failed to clean up bindings for expired ClusterTemporaryRBAC")
// 			return err
// 		}
//
// 		clusterTempRBAC.Status.State = "Expired"
// 		if err := r.Status().Update(ctx, clusterTempRBAC); err != nil {
// 			logger.Error(err, "Failed to update status for expired ClusterTemporaryRBAC")
// 			return err
// 		}
// 	}
// 	return nil
// }
//
// // cleanupBindings removes associated bindings
// func (r *ClusterTemporaryRBACReconciler) cleanupBindings(ctx context.Context, clusterTempRBAC *tarbacv1.ClusterTemporaryRBAC, logger logr.Logger) error {
// 	logger.Info("Cleaning up bindings for ClusterTemporaryRBAC", "name", clusterTempRBAC.Name)
//
// 	var remainingChildResources []tarbacv1.ChildResource
// 	for _, child := range clusterTempRBAC.Status.ChildResource {
// 		if child.Kind == "ClusterRoleBinding" {
// 			roleBinding := &rbacv1.ClusterRoleBinding{
// 				ObjectMeta: metav1.ObjectMeta{Name: child.Name},
// 			}
// 			if err := r.Client.Delete(ctx, roleBinding); err != nil && !apierrors.IsNotFound(err) {
// 				logger.Error(err, "Failed to delete ClusterRoleBinding", "name", child.Name)
// 				remainingChildResources = append(remainingChildResources, child)
// 				continue
// 			}
// 			logger.Info("ClusterRoleBinding deleted successfully", "name", child.Name)
// 		} else {
// 			logger.Error(nil, "Unsupported child resource kind", "kind", child.Kind)
// 			remainingChildResources = append(remainingChildResources, child)
// 		}
// 	}
//
// 	clusterTempRBAC.Status.ChildResource = remainingChildResources
// 	return nil
// }
//
// // scheduleRequeue calculates the next requeue interval
// func (r *ClusterTemporaryRBACReconciler) scheduleRequeue(clusterTempRBAC *tarbacv1.ClusterTemporaryRBAC, logger logr.Logger) ctrl.Result {
// 	timeUntilExpiration := time.Until(clusterTempRBAC.Status.ExpiresAt.Time)
// 	if timeUntilExpiration <= 0 {
// 		logger.Info("ClusterTemporaryRBAC has already expired, skipping requeue.")
// 		return ctrl.Result{}
// 	}
//
// 	if timeUntilExpiration <= 1*time.Second {
// 		logger.Info("Requeueing closer to expiration", "timeUntilExpiration", timeUntilExpiration)
// 		return ctrl.Result{RequeueAfter: 100 * time.Millisecond}
// 	}
//
// 	logger.Info("Requeueing for regular expiration check", "timeUntilExpiration", timeUntilExpiration)
// 	return ctrl.Result{RequeueAfter: timeUntilExpiration.Truncate(time.Second)}
// }
//
// // generateBindingName generates a unique name for the role binding
// func generateBindingName(subject rbacv1.Subject, roleRef rbacv1.RoleRef) string {
// 	return fmt.Sprintf("%s-%s-%s", strings.ToLower(subject.Kind), subject.Name, roleRef.Name)
// }
//
// // SetupWithManager sets up the controller with the Manager
// func (r *ClusterTemporaryRBACReconciler) SetupWithManager(mgr ctrl.Manager) error {
// 	return ctrl.NewControllerManagedBy(mgr).
// 		For(&tarbacv1.ClusterTemporaryRBAC{}).
// 		Complete(r)
// }
//
//
// // package controllers
// //
// // import (
// // 	"context"
// // 	"fmt"
// // 	"time"
// // 	"strings"
// //
// // 	tarbacv1 "github.com/guybal/tarbac/api/v1" // Adjust to match your actual module path
// // //     utils "github.com/guybal/tarbac/utils"
// //     rbacv1 "k8s.io/api/rbac/v1"
// //     metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// //     "k8s.io/apimachinery/pkg/runtime"
// //     apierrors "k8s.io/apimachinery/pkg/api/errors" // Import for IsAlreadyExists
// //     ctrl "sigs.k8s.io/controller-runtime"
// //     "sigs.k8s.io/controller-runtime/pkg/client"
// //     "sigs.k8s.io/controller-runtime/pkg/log"
// //     "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
// // )
// // //
// // // func AddToScheme(scheme *runtime.Scheme) error {
// // //     return tarbacv1.AddToScheme(scheme)
// // // }
// // //
// // type ClusterTemporaryRBACReconciler struct {
// // 	client.Client
// // 	Scheme *runtime.Scheme
// // }
// //
// // // Reconcile performs reconciliation for ClusterTemporaryRBAC objects
// // func (r *ClusterTemporaryRBACReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
// // 	currentTime := time.Now()
// // 	logger := log.FromContext(ctx)
// // 	logger.Info("Reconciling ClusterTemporaryRBAC", "name", req.Name)
// //
// // 	// Fetch the ClusterTemporaryRBAC object
// // 	var clusterTempRBAC tarbacv1.ClusterTemporaryRBAC
// // 	if err := r.Get(ctx, req.NamespacedName, &clusterTempRBAC); err != nil {
// // 		if apierrors.IsNotFound(err) {
// // 			logger.Info("ClusterTemporaryRBAC resource not found. Ignoring since it must have been deleted.")
// // 			return ctrl.Result{}, nil
// // 		}
// // 		logger.Error(err, "Unable to fetch ClusterTemporaryRBAC")
// // 		return ctrl.Result{}, err
// // 	}
// //
// // 	// Parse the duration from the spec
// // 	duration, err := time.ParseDuration(clusterTempRBAC.Spec.Duration)
// // 	if err != nil {
// // 		logger.Error(err, "Invalid duration in ClusterTemporaryRBAC spec", "duration", clusterTempRBAC.Spec.Duration)
// // 		return ctrl.Result{}, err
// // 	}
// //
// // 	if clusterTempRBAC.Status.CreatedAt == nil {
// // 		// Ensure bindings are created and status is updated
// // 		if err := r.ensureBindings(ctx, &clusterTempRBAC, duration); err != nil {
// // 			logger.Error(err, "Failed to ensure bindings for ClusterTemporaryRBAC")
// // 			return ctrl.Result{}, err
// // 		}
// // 	}
// //
// // 	// Calculate expiration time if not already set
// // 	if clusterTempRBAC.Status.ExpiresAt == nil {
// // 		expiration := clusterTempRBAC.Status.CreatedAt.Time.Add(duration)
// // 		clusterTempRBAC.Status.ExpiresAt = &metav1.Time{Time: expiration}
// // 		// Commit the status update to the API server
// // 		if err := r.Status().Update(ctx, &clusterTempRBAC); err != nil {
// // 			logger.Error(err, "Failed to update ClusterTemporaryRBAC status with expiration date")
// // 			return ctrl.Result{}, err
// // 		}
// // 	}
// //
// //     if len(clusterTempRBAC.OwnerReferences) > 0 {
// //         if err := r.fetchAndSetRequestID(ctx, &clusterTempRBAC); err != nil {
// //             logger.Error(err, "Failed to fetch and set RequestID", "ClusterTemporaryRBAC", clusterTempRBAC.Name)
// //             return ctrl.Result{}, err
// //         }
// //     }
// //
// // 	// Check expiration status
// // 	logger.Info("Checking expiration", "currentTime", currentTime, "expiresAt", clusterTempRBAC.Status.ExpiresAt)
// //
// // 	if currentTime.After(clusterTempRBAC.Status.ExpiresAt.Time) {
// // 		logger.Info("ClusterTemporaryRBAC expired, cleaning up associated bindings", "name", clusterTempRBAC.Name)
// //
// // 		// Cleanup expired bindings
// // 		if err := r.cleanupBindings(ctx, &clusterTempRBAC); err != nil {
// // 			logger.Error(err, "Failed to clean up bindings for expired ClusterTemporaryRBAC")
// // 			return ctrl.Result{}, err
// // 		}
// // 		return ctrl.Result{}, nil
// // 	}
// //
// // 	// Calculate time until expiration
// // 	timeUntilExpiration := time.Until(clusterTempRBAC.Status.ExpiresAt.Time)
// // 	logger.Info("ClusterTemporaryRBAC is still valid", "name", clusterTempRBAC.Name, "timeUntilExpiration", timeUntilExpiration)
// //
// // 	// If expiration is very close, requeue with a smaller interval
// // 	if timeUntilExpiration <= 1*time.Second {
// // 		logger.Info("Requeueing closer to expiration for final check", "timeUntilExpiration", timeUntilExpiration)
// // 		return ctrl.Result{RequeueAfter: 100 * time.Millisecond}, nil
// // 	}
// //
// // 	// Requeue for regular reconciliation
// // 	logger.Info("ClusterTemporaryRBAC successfully reconciled, requeueing for expiration", "name", clusterTempRBAC.Name)
// // 	return ctrl.Result{RequeueAfter: timeUntilExpiration.Truncate(time.Second)}, nil
// // }
// //
// // // ensureBindings creates ClusterRoleBindings for the ClusterTemporaryRBAC resource
// // func (r *ClusterTemporaryRBACReconciler) ensureBindings(ctx context.Context, clusterTempRBAC *tarbacv1.ClusterTemporaryRBAC, duration time.Duration) error {
// // 	logger := log.FromContext(ctx)
// //
// // 	var subjects []rbacv1.Subject
// // 	if len(clusterTempRBAC.Spec.Subjects) > 0 {
// // 		subjects = append(subjects, clusterTempRBAC.Spec.Subjects...)
// // 	}
// //
// // 	if len(subjects) == 0 {
// // 		logger.Error(nil, "No subjects specified in ClusterTemporaryRBAC")
// // 		return fmt.Errorf("no subjects specified")
// // 	}
// //
// // 	// Set CreatedAt if not already set
// // 	if clusterTempRBAC.Status.CreatedAt == nil {
// // 		clusterTempRBAC.Status.CreatedAt = &metav1.Time{Time: time.Now()}
// // 	}
// //
// // 	var childResources = []tarbacv1.ChildResource{}
// //
// // 	for _, subject := range subjects {
// // 		roleBinding := &rbacv1.ClusterRoleBinding{
// // 			ObjectMeta: metav1.ObjectMeta{
// // 				Name: generateBindingName(subject, clusterTempRBAC.Spec.RoleRef), // utils.GenerateBindingName
// // 				Labels: map[string]string{
// // 					"tarbac.io/owner": clusterTempRBAC.Name,
// // 				},
// // 			},
// // 			Subjects: []rbacv1.Subject{subject},
// // 			RoleRef: rbacv1.RoleRef{
// // 				APIGroup: clusterTempRBAC.Spec.RoleRef.APIGroup,
// //                 Kind:     clusterTempRBAC.Spec.RoleRef.Kind,
// //                 Name:     clusterTempRBAC.Spec.RoleRef.Name,
// // 			},
// // 		}
// // //         roleBinding.GetObjectKind().SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("ClusterRoleBinding"))
// //
// // 		// Set the OwnerReference on the ClusterRoleBinding
// // 		if err := controllerutil.SetControllerReference(clusterTempRBAC, roleBinding, r.Scheme); err != nil {
// // 			logger.Error(err, "Failed to set OwnerReference for ClusterRoleBinding", "ClusterRoleBinding", roleBinding.Name)
// // 			return err
// // 		}
// //
// // 		// Create the ClusterRoleBinding
// // 		if err := r.Client.Create(ctx, roleBinding); err != nil && !apierrors.IsAlreadyExists(err) {
// // 			logger.Error(err, "Failed to create ClusterRoleBinding", "ClusterRoleBinding", roleBinding.Name)
// // 			return err
// // 		}
// //
// //         // Add to childResources with proper Kind and APIVersion
// // 		childResources = append(childResources, tarbacv1.ChildResource{
// // 			APIVersion: rbacv1.SchemeGroupVersion.String(), // Correctly set the APIGroup
// // 			Kind:       "ClusterRoleBinding",               // Correctly set the Kind
// // 			Name:       roleBinding.GetName(),
// // 		})
// //
// // 		logger.Info("Successfully created ClusterRoleBinding with OwnerReference", "ClusterRoleBinding", roleBinding.Name)
// // 	}
// //
// // 	// Update the status with created child resources
// // 	clusterTempRBAC.Status.ChildResource = childResources
// // 	clusterTempRBAC.Status.State = "Created"
// //
// // 	if err := r.Status().Update(ctx, clusterTempRBAC); err != nil {
// // 		logger.Error(err, "Failed to update ClusterTemporaryRBAC status after ensuring bindings")
// // 		return err
// // 	}
// //
// // 	logger.Info("Successfully ensured bindings and updated status", "ClusterTemporaryRBAC", clusterTempRBAC.Name)
// // 	return nil
// // }
// //
// // // cleanupBindings deletes the ClusterRoleBindings associated with the ClusterTemporaryRBAC resource
// // func (r *ClusterTemporaryRBACReconciler) cleanupBindings(ctx context.Context, clusterTempRBAC *tarbacv1.ClusterTemporaryRBAC) error {
// // 	logger := log.FromContext(ctx)
// // 	var remainingChildResources []tarbacv1.ChildResource
// //
// // 	for _, child := range clusterTempRBAC.Status.ChildResource {
// // 		logger.Info("Cleaning up child resource", "kind", child.Kind, "name", child.Name)
// //
// // 		if child.Kind == "ClusterRoleBinding" {
// // 			// Delete ClusterRoleBinding
// // 			if err := r.Client.Delete(ctx, &rbacv1.ClusterRoleBinding{
// // 				ObjectMeta: metav1.ObjectMeta{
// // 					Name: child.Name,
// // 				},
// // 			}); err != nil && !apierrors.IsNotFound(err) {
// // 				logger.Error(err, "Failed to delete ClusterRoleBinding", "name", child.Name)
// // 				remainingChildResources = append(remainingChildResources, child)
// // 				continue
// // 			}
// // 			logger.Info("Successfully deleted ClusterRoleBinding", "name", child.Name)
// // 		} else {
// // 			logger.Error(fmt.Errorf("unsupported child resource kind"), "Unsupported child resource kind", "kind", child.Kind)
// // 			remainingChildResources = append(remainingChildResources, child)
// // 		}
// // 	}
// //
// // 	// Update the ChildResource slice after cleanup
// // 	if len(remainingChildResources) == 0 {
// // 		clusterTempRBAC.Status.ChildResource = nil // Reset if all resources are deleted
// // 	} else {
// // 		clusterTempRBAC.Status.ChildResource = remainingChildResources
// // 	}
// //
// //     // Update the state if no child resources remain
// //     if clusterTempRBAC.Status.ChildResource == nil {
// //         clusterTempRBAC.Status.State = "Expired"
// //     }
// //
// // 	// Check RetentionPolicy
// // 	if clusterTempRBAC.Spec.RetentionPolicy == "delete" && clusterTempRBAC.Status.ChildResource == nil {
// // 		logger.Info("RetentionPolicy is set to delete, deleting ClusterTemporaryRBAC resource", "name", clusterTempRBAC.Name)
// // 		if err := r.Client.Delete(ctx, clusterTempRBAC); err != nil {
// // 			logger.Error(err, "Failed to delete ClusterTemporaryRBAC resource")
// // 			return err
// // 		}
// // 		return nil // Exit since resource is deleted
// // 	}
// //
// // 	if err := r.Status().Update(ctx, clusterTempRBAC); err != nil {
// // 		logger.Error(err, "Failed to update ClusterTemporaryRBAC status")
// // 		return err
// // 	}
// //
// // 	logger.Info("ClusterTemporaryRBAC status updated", "name", clusterTempRBAC.Name, "state", clusterTempRBAC.Status.State)
// // 	return nil
// // }
// //
// // func (r *ClusterTemporaryRBACReconciler) fetchAndSetRequestID(ctx context.Context, clusterTempRBAC *tarbacv1.ClusterTemporaryRBAC) error {
// //     logger := log.FromContext(ctx)
// //
// //     if clusterTempRBAC.Status.RequestID == "" && len(clusterTempRBAC.OwnerReferences) > 0 {
// //         logger.Info("ClusterTemporaryRBAC missing RequestID, attempting to fetch owner reference")
// //
// //         // Loop through owner references
// //         for _, ownerRef := range clusterTempRBAC.OwnerReferences {
// //             var ownerRequestID string
// //
// //             switch ownerRef.Kind {
// //             case "ClusterSudoRequest":
// //                 var clusterSudoRequest tarbacv1.ClusterSudoRequest
// //                 err := r.Client.Get(ctx, client.ObjectKey{Name: ownerRef.Name}, &clusterSudoRequest)
// //                 if err != nil {
// //                     logger.Error(err, "Failed to fetch ClusterSudoRequest", "ownerRef", ownerRef.Name)
// //                     continue
// //                 }
// //                 ownerRequestID = clusterSudoRequest.Status.RequestID
// //             case "SudoRequest":
// //                 var sudoRequest tarbacv1.SudoRequest
// //                 err := r.Client.Get(ctx, client.ObjectKey{Name: ownerRef.Name, Namespace: clusterTempRBAC.Namespace}, &sudoRequest)
// //                 if err != nil {
// //                     logger.Error(err, "Failed to fetch SudoRequest", "ownerRef", ownerRef.Name)
// //                     continue
// //                 }
// //                 ownerRequestID = sudoRequest.Status.RequestID
// //             default:
// //                 logger.Info("Unsupported owner reference kind, skipping", "kind", ownerRef.Kind)
// //                 continue
// //             }
// //
// //             // Update the RequestID if found
// //             if ownerRequestID != "" {
// //                 clusterTempRBAC.Status.RequestID = ownerRequestID
// //                 if err := r.Status().Update(ctx, clusterTempRBAC); err != nil {
// //                     logger.Error(err, "Failed to update TemporaryRBAC status with RequestID", "ClusterTemporaryRBAC", clusterTempRBAC.Name)
// //                     return err
// //                 }
// //                 logger.Info("TemporaryRBAC status updated with RequestID", "RequestID", ownerRequestID)
// //                 break
// //             }
// //         }
// //     }
// //     return nil
// // }
// //
// // func generateBindingName(subject rbacv1.Subject, roleRef rbacv1.RoleRef) string {
// // 	return fmt.Sprintf("%s-%s-%s", strings.ToLower(subject.Kind), subject.Name, roleRef.Name)
// // }
// //
// // func (r *ClusterTemporaryRBACReconciler) SetupWithManager(mgr ctrl.Manager) error {
// // 	return ctrl.NewControllerManagedBy(mgr).
// // 		For(&tarbacv1.ClusterTemporaryRBAC{}).
// // 		Complete(r)
// // }