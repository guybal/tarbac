package controllers

import (
	"context"
	"fmt"
	"time"

	tarbacv1 "github.com/guybal/tarbac/api/v1"
	utils "github.com/guybal/tarbac/utils"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ClusterTemporaryRBACReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// Reconcile performs reconciliation for ClusterTemporaryRBAC objects
func (r *ClusterTemporaryRBACReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	currentTime := time.Now()
	logger := log.FromContext(ctx)
	var requestId string

	utils.LogInfo(logger, "Reconciling ClusterTemporaryRBAC", "name", req.Name, "namespace", req.Namespace)

	// Fetch the ClusterTemporaryRBAC object (cluster-scoped, so no namespace)
	var clusterTempRBAC tarbacv1.ClusterTemporaryRBAC
	if err := r.Get(ctx, client.ObjectKey{Name: req.Name}, &clusterTempRBAC); err != nil {
		if apierrors.IsNotFound(err) {
			utils.LogInfo(logger, "ClusterTemporaryRBAC resource not found. Ignoring since it must have been deleted.", "name", req.Name, "namespace", req.Namespace)
			return ctrl.Result{}, nil
		}
		utils.LogError(logger, err, "Unable to fetch ClusterTemporaryRBAC")
		logger.Error(err, "Unable to fetch ClusterTemporaryRBAC", "name", req.Name, "namespace", req.Namespace)
		return ctrl.Result{}, err
	}

	if len(clusterTempRBAC.OwnerReferences) > 0 {
		if clusterTempRBAC.Status.RequestID == "" {
			if err := r.fetchAndSetRequestID(ctx, &clusterTempRBAC, requestId); err != nil {
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

	requestId = r.getRequestID(&clusterTempRBAC)

	// Parse the duration from the spec
	duration, err := time.ParseDuration(clusterTempRBAC.Spec.Duration)
	if err != nil {
		utils.LogErrorUID(logger, err, "Invalid duration in ClusterTemporaryRBAC spec", requestId, "duration", clusterTempRBAC.Spec.Duration)
		return ctrl.Result{}, err
	}

	if clusterTempRBAC.Status.CreatedAt == nil || isActive(clusterTempRBAC, currentTime) {
		// Ensure bindings are created and status is updated
		if err := r.ensureBindings(ctx, &clusterTempRBAC, requestId); err != nil {
			utils.LogErrorUID(logger, err, "Failed to ensure bindings for ClusterTemporaryRBAC", requestId, clusterTempRBAC.Status.CreatedAt, "expiresAt", clusterTempRBAC.Status.ExpiresAt)
			return ctrl.Result{}, err
		}
	}

	// Calculate expiration time if not already set
	if clusterTempRBAC.Status.ExpiresAt == nil {
		expiration := clusterTempRBAC.Status.CreatedAt.Time.Add(duration)
		clusterTempRBAC.Status.ExpiresAt = &metav1.Time{Time: expiration}
		// Commit the status update to the API server
		if err := r.Status().Update(ctx, &clusterTempRBAC); err != nil {
			utils.LogErrorUID(logger, err, "Failed to update ClusterTemporaryRBAC status with expiration date", requestId, "expiration", expiration)
			return ctrl.Result{}, err
		}
	}

	// Check expiration status
	utils.LogInfoUID(logger, "Checking expiration", requestId, "currentTime", currentTime, "expiresAt", clusterTempRBAC.Status.ExpiresAt)

	if currentTime.After(clusterTempRBAC.Status.ExpiresAt.Time) {
		utils.LogInfoUID(logger, "ClusterTemporaryRBAC expired, cleaning up associated bindings", requestId, "currentTime", currentTime, "expiresAt", clusterTempRBAC.Status.ExpiresAt)

		// Cleanup expired bindings
		if err := r.cleanupBindings(ctx, &clusterTempRBAC, requestId); err != nil {
			utils.LogErrorUID(logger, err, "Failed to clean up bindings for expired ClusterTemporaryRBAC", requestId)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Calculate time until expiration
	timeUntilExpiration := time.Until(clusterTempRBAC.Status.ExpiresAt.Time)
	utils.LogInfoUID(logger, "ClusterTemporaryRBAC is still valid", requestId, "timeUntilExpiration", timeUntilExpiration)

	// If expiration is very close, requeue with a smaller interval
	if timeUntilExpiration <= 1*time.Second {
		utils.LogInfoUID(logger, "Requeueing closer to expiration for final check", requestId, "timeUntilExpiration", timeUntilExpiration)
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	// Requeue for regular reconciliation
	utils.LogInfoUID(logger, "ClusterTemporaryRBAC successfully reconciled, requeueing for expiration", requestId, "timeUntilExpiration", timeUntilExpiration)
	return ctrl.Result{RequeueAfter: timeUntilExpiration.Truncate(time.Second)}, nil
}

func isActive(clusterTempRBAC tarbacv1.ClusterTemporaryRBAC, currentTime time.Time) bool {
	return clusterTempRBAC.Status.ExpiresAt != nil && currentTime.Before(clusterTempRBAC.Status.ExpiresAt.Time) && currentTime.After(clusterTempRBAC.Status.CreatedAt.Time)
}

func (r *ClusterTemporaryRBACReconciler) getRequestID(clusterTempRBAC *tarbacv1.ClusterTemporaryRBAC) string {

	var requestId string
	if clusterTempRBAC.Status.RequestID != "" {
		requestId = clusterTempRBAC.Status.RequestID
	} else {
		requestId = string(clusterTempRBAC.ObjectMeta.UID)
	}
	return requestId
}

// ensureBindings creates ClusterRoleBindings for the ClusterTemporaryRBAC resource
func (r *ClusterTemporaryRBACReconciler) ensureBindings(ctx context.Context, clusterTempRBAC *tarbacv1.ClusterTemporaryRBAC, requestId string) error {
	logger := log.FromContext(ctx)

	var subjects []rbacv1.Subject
	if len(clusterTempRBAC.Spec.Subjects) > 0 {
		subjects = append(subjects, clusterTempRBAC.Spec.Subjects...)
	}

	if len(subjects) == 0 {
		utils.LogErrorUID(logger, nil, "No subjects specified in ClusterTemporaryRBAC", requestId)
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
				Name: utils.GenerateBindingName(subject, clusterTempRBAC.Spec.RoleRef, requestId),
				Labels: map[string]string{
					"tarbac.io/owner":      clusterTempRBAC.Name,
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
			utils.LogErrorUID(logger, err, "Failed to set OwnerReference for ClusterRoleBinding", requestId, "ClusterRoleBinding", roleBinding.Name)
			return err
		}

		// Create the ClusterRoleBinding
		if err := r.Client.Create(ctx, roleBinding); err != nil && !apierrors.IsAlreadyExists(err) {
			utils.LogErrorUID(logger, err, "Failed to create ClusterRoleBinding", requestId, "ClusterRoleBinding", roleBinding.Name)
			return err
		}

		// Add to childResources with proper Kind and APIVersion
		childResources = append(childResources, tarbacv1.ChildResource{
			APIVersion: rbacv1.SchemeGroupVersion.String(), // Correctly set the APIVersion
			Kind:       "ClusterRoleBinding",               // Correctly set the Kind
			Name:       roleBinding.GetName(),
		})
	}

	// Update the status with created child resources
	clusterTempRBAC.Status.ChildResource = childResources
	clusterTempRBAC.Status.State = "Created"

	if err := r.Status().Update(ctx, clusterTempRBAC); err != nil {
		utils.LogErrorUID(logger, err, "Failed to update ClusterTemporaryRBAC status after ensuring bindings", requestId, "childResources", childResources)
		return err
	}

	r.Recorder.Event(clusterTempRBAC, "Normal", "PermissionsGranted", fmt.Sprintf("Temporary permissions were granted in cluster scope [UID: %s]", requestId))
	logger.Info("Successfully ensured bindings and updated status", "ClusterTemporaryRBAC", clusterTempRBAC.Name)

	return nil
}

// cleanupBindings deletes the ClusterRoleBindings associated with the ClusterTemporaryRBAC resource
func (r *ClusterTemporaryRBACReconciler) cleanupBindings(ctx context.Context, clusterTempRBAC *tarbacv1.ClusterTemporaryRBAC, requestId string) error {
	logger := log.FromContext(ctx)
	var remainingChildResources []tarbacv1.ChildResource

	for _, child := range clusterTempRBAC.Status.ChildResource {
		utils.LogInfoUID(logger, "Cleaning up child resource", requestId, "kind", child.Kind, "name", child.Name, "namespace", child.Namespace)

		if child.Kind == "ClusterRoleBinding" {
			// Delete ClusterRoleBinding
			if err := r.Client.Delete(ctx, &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: child.Name,
				},
			}); err != nil && !apierrors.IsNotFound(err) {
				utils.LogErrorUID(logger, err, "Failed to delete ClusterRoleBinding", requestId, "kind", child.Kind, "name", child.Name, "namespace", child.Namespace)
				remainingChildResources = append(remainingChildResources, child)
				continue
			}
			utils.LogInfoUID(logger, "Successfully deleted RoleBinding", requestId, "kind", child.Kind, "name", child.Name, "namespace", child.Namespace)
			r.Recorder.Event(clusterTempRBAC, "Normal", "PermissionsRevoked", fmt.Sprintf("Temporary permissions were revoked in cluster scope [UID: %s]", requestId))
		} else {
			utils.LogErrorUID(logger, nil, "Unsupported child resource kind", requestId, "kind", child.Kind)
			remainingChildResources = append(remainingChildResources, child)
		}
	}

	// Update the ChildResource slice after cleanup
	if len(remainingChildResources) == 0 {
		clusterTempRBAC.Status.ChildResource = nil
	} else {
		clusterTempRBAC.Status.ChildResource = remainingChildResources
	}

	// Update the state if no child resources remain
	if clusterTempRBAC.Status.ChildResource == nil {
		clusterTempRBAC.Status.State = "Expired"
	}

	// Check RetentionPolicy
	if clusterTempRBAC.Spec.RetentionPolicy == "delete" && clusterTempRBAC.Status.ChildResource == nil {
		utils.LogInfoUID(logger, "RetentionPolicy is set to delete, deleting ClusterTemporaryRBAC resource", requestId, "kind", clusterTempRBAC.Kind, "name", clusterTempRBAC.Name)

		if err := r.Client.Delete(ctx, clusterTempRBAC); err != nil {
			utils.LogErrorUID(logger, err, "Failed to delete ClusterTemporaryRBAC resource", requestId)
			return err
		}
		return nil // Exit since resource is deleted
	}

	if err := r.Status().Update(ctx, clusterTempRBAC); err != nil {
		utils.LogErrorUID(logger, err, "Failed to update ClusterTemporaryRBAC status", requestId)
		return err
	}

	utils.LogInfoUID(logger, "TemporaryRBAC status updated", requestId, "kind", clusterTempRBAC.Kind, "name", clusterTempRBAC.Name, "state", clusterTempRBAC.Status.State)
	return nil
}

func (r *ClusterTemporaryRBACReconciler) fetchAndSetRequestID(ctx context.Context, tempRBAC *tarbacv1.ClusterTemporaryRBAC, requestId string) error {
	logger := log.FromContext(ctx)

	if tempRBAC.Status.RequestID == "" && len(tempRBAC.OwnerReferences) > 0 {
		utils.LogInfoUID(logger, "TemporaryRBAC missing RequestID, attempting to fetch owner reference", requestId)

		// Loop through owner references
		for _, ownerRef := range tempRBAC.OwnerReferences {
			var ownerRequestID string

			switch ownerRef.Kind {
			case "ClusterSudoRequest":
				var clusterSudoRequest tarbacv1.ClusterSudoRequest
				err := r.Client.Get(ctx, client.ObjectKey{Name: ownerRef.Name}, &clusterSudoRequest)
				if err != nil {
					utils.LogErrorUID(logger, err, "Failed to fetch ClusterSudoRequest", requestId, "ownerRef", ownerRef.Name)
					continue
				}
				ownerRequestID = clusterSudoRequest.Status.RequestID
			case "SudoRequest":
				var sudoRequest tarbacv1.SudoRequest
				err := r.Client.Get(ctx, client.ObjectKey{Name: ownerRef.Name, Namespace: tempRBAC.Namespace}, &sudoRequest)
				if err != nil {
					utils.LogErrorUID(logger, err, "Failed to fetch Failed to fetch SudoRequest", requestId, "ownerRef", ownerRef.Name)
					continue
				}
				ownerRequestID = sudoRequest.Status.RequestID
			default:
				utils.LogInfoUID(logger, "Unsupported owner reference kind, skipping", requestId, "ownerRef", ownerRef.Name, "ownerRefKind", ownerRef.Kind)
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
					utils.LogErrorUID(logger, err, "Failed to update ClusterTemporaryRBAC labels with RequestID", requestId, "ClusterTemporaryRBAC", tempRBAC.Name)
					return err
				}

				tempRBAC.Status.RequestID = ownerRequestID
				if err := r.Status().Update(ctx, tempRBAC); err != nil {
					utils.LogErrorUID(logger, err, "Failed to update ClusterTemporaryRBAC status with RequestID", requestId, "ClusterTemporaryRBAC", tempRBAC.Name)
					return err
				}
				utils.LogInfoUID(logger, "ClusterTemporaryRBAC status updated with RequestID from owner", requestId, "ClusterTemporaryRBAC", tempRBAC.Name, "ownerRef", ownerRef.Name, "ownerRefKind", ownerRef.Kind)
				break
			}
		}
	}
	return nil
}

func (r *ClusterTemporaryRBACReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("ClusterTemporaryRBACController")
	return ctrl.NewControllerManagedBy(mgr).
		For(&tarbacv1.ClusterTemporaryRBAC{}).
		Complete(r)
}
