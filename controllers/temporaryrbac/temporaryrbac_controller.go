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

func AddToScheme(scheme *runtime.Scheme) error {
	return tarbacv1.AddToScheme(scheme)
}

type TemporaryRBACReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// Reconcile performs reconciliation for TemporaryRBAC objects
func (r *TemporaryRBACReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	currentTime := time.Now()
	logger := log.FromContext(ctx)
	var requestId string

	utils.LogInfo(logger, "Reconciling TemporaryRBAC", "name", req.Name, "namespace", req.Namespace)

	// Fetch the TemporaryRBAC object
	var tempRBAC tarbacv1.TemporaryRBAC
	if err := r.Get(ctx, req.NamespacedName, &tempRBAC); err != nil {
		if apierrors.IsNotFound(err) {
			utils.LogInfo(logger, "TemporaryRBAC resource not found. Ignoring since it must have been deleted.", "name", req.Name, "namespace", req.Namespace)
			return ctrl.Result{}, nil
		}
		utils.LogError(logger, err, "Unable to fetch TemporaryRBAC", "name", req.Name, "namespace", req.Namespace)
		return ctrl.Result{}, err
	}

	if len(tempRBAC.OwnerReferences) > 0 {
		if tempRBAC.Status.RequestID == "" {
			if err := r.fetchAndSetRequestID(ctx, &tempRBAC, requestId); err != nil {
				utils.LogError(logger, err, "Failed to fetch and set RequestID", "name", tempRBAC.Name, "namespace", req.Namespace)
				return ctrl.Result{}, err
			}
		}
	} else {
		if tempRBAC.Status.RequestID == "" {
			tempRBAC.Status.RequestID = string(tempRBAC.ObjectMeta.UID)
			if err := r.Status().Update(ctx, &tempRBAC); err != nil {
				utils.LogError(logger, err, "Failed to update TemporaryRBAC status", "name", tempRBAC.Name, "namespace", req.Namespace)
				return ctrl.Result{}, err
			}
		}
	}

	requestId = r.getRequestID(&tempRBAC)

	// Parse the duration from the spec
	duration, err := time.ParseDuration(tempRBAC.Spec.Duration)
	if err != nil {
		utils.LogErrorUID(logger, err, "Invalid duration in TemporaryRBAC spec", requestId, "duration", tempRBAC.Spec.Duration)
		return ctrl.Result{}, err
	}

	if tempRBAC.Status.CreatedAt == nil ||
		(tempRBAC.Status.ExpiresAt != nil && currentTime.Before(tempRBAC.Status.ExpiresAt.Time)) && currentTime.After(tempRBAC.Status.CreatedAt.Time) {
		// Ensure bindings are created and status is updated
		if err := r.ensureBindings(ctx, &tempRBAC, requestId); err != nil {
			utils.LogErrorUID(logger, err, "Failed to ensure bindings for TemporaryRBAC", requestId, "createdAt", tempRBAC.Status.CreatedAt, "expiresAt", tempRBAC.Status.ExpiresAt)
			return ctrl.Result{}, err
		}
	}

	// Calculate expiration time if not already set
	if tempRBAC.Status.ExpiresAt == nil {
		expiration := tempRBAC.Status.CreatedAt.Time.Add(duration)
		tempRBAC.Status.ExpiresAt = &metav1.Time{Time: expiration}
		// Commit the status update to the API server
		if err := r.Status().Update(ctx, &tempRBAC); err != nil {
			utils.LogErrorUID(logger, err, "Failed to update TemporaryRBAC status with expiration date", requestId, "expiration", expiration)
			return ctrl.Result{}, err
		}
	}

	// Check expiration status
	utils.LogInfoUID(logger, "Checking expiration", requestId, "currentTime", currentTime, "expiresAt", tempRBAC.Status.ExpiresAt)

	if currentTime.After(tempRBAC.Status.ExpiresAt.Time) {
		utils.LogInfoUID(logger, "TemporaryRBAC expired, cleaning up associated bindings", requestId, "currentTime", currentTime, "expiresAt", tempRBAC.Status.ExpiresAt)

		// Cleanup expired bindings
		if err := r.cleanupBindings(ctx, &tempRBAC, requestId); err != nil {
			utils.LogErrorUID(logger, err, "Failed to clean up bindings for expired TemporaryRBAC", requestId)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Calculate time until expiration
	timeUntilExpiration := time.Until(tempRBAC.Status.ExpiresAt.Time)
	utils.LogInfoUID(logger, "TemporaryRBAC is still valid", requestId, "timeUntilExpiration", timeUntilExpiration)

	// If expiration is very close, requeue with a smaller interval
	if timeUntilExpiration <= 1*time.Second {
		utils.LogInfoUID(logger, "Requeueing closer to expiration for final check", requestId, "timeUntilExpiration", timeUntilExpiration)
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	// Requeue for regular reconciliation
	utils.LogInfoUID(logger, "TemporaryRBAC successfully reconciled, requeueing for expiration", requestId, "timeUntilExpiration", timeUntilExpiration)
	return ctrl.Result{RequeueAfter: timeUntilExpiration.Truncate(time.Second)}, nil
}

func (r *TemporaryRBACReconciler) getRequestID(tempRBAC *tarbacv1.TemporaryRBAC) string {

	var requestId string

	if tempRBAC.Status.RequestID != "" {
		requestId = tempRBAC.Status.RequestID
	} else {
		requestId = string(tempRBAC.ObjectMeta.UID)
	}

	return requestId
}

// ensureBindings creates or updates the RoleBinding or ClusterRoleBinding for the TemporaryRBAC resource
func (r *TemporaryRBACReconciler) ensureBindings(ctx context.Context, tempRBAC *tarbacv1.TemporaryRBAC, requestId string) error {
	logger := log.FromContext(ctx)

	var subjects []rbacv1.Subject

	if len(tempRBAC.Spec.Subjects) > 0 {
		subjects = append(subjects, tempRBAC.Spec.Subjects...)
	}

	if len(subjects) == 0 {
		utils.LogErrorUID(logger, nil, "No subjects specified in TemporaryRBAC", requestId)
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
					Name:      utils.GenerateBindingName(subject, roleRef, requestId),
					Namespace: tempRBAC.ObjectMeta.Namespace,
					Labels: map[string]string{
						"tarbac.io/owner":      tempRBAC.Name,
						"tarbac.io/request-id": requestId,
					},
				},
				Subjects: []rbacv1.Subject{subject},
				RoleRef:  roleRef,
			}
			binding.GetObjectKind().SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("RoleBinding")) // Remove

		} else if tempRBAC.Spec.RoleRef.Kind == "Role" {
			binding = &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      utils.GenerateBindingName(subject, roleRef, requestId),
					Namespace: tempRBAC.ObjectMeta.Namespace,
					Labels: map[string]string{
						"tarbac.io/owner":      tempRBAC.Name,
						"tarbac.io/request-id": requestId,
					},
				},
				Subjects: []rbacv1.Subject{subject},
				RoleRef:  roleRef,
			}
			binding.GetObjectKind().SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("RoleBinding")) // Remove
		} else {
			utils.LogErrorUID(logger, nil, fmt.Sprintf("unsupported roleRef.kind: %s", tempRBAC.Spec.RoleRef.Kind), requestId)
			return fmt.Errorf("unsupported roleRef.kind: %s", tempRBAC.Spec.RoleRef.Kind)
		}

		// Set the OwnerReference on the RoleBinding
		if err := controllerutil.SetControllerReference(tempRBAC, binding, r.Scheme); err != nil {
			utils.LogErrorUID(logger, err, "Failed to set OwnerReference for RoleBinding", requestId, "RoleBinding", binding)
			return err
		}

		// Attempt to create the binding
		if err := r.Client.Create(ctx, binding); err != nil && !apierrors.IsAlreadyExists(err) {
			utils.LogErrorUID(logger, err, "Failed to create binding", requestId, "RoleBinding", binding)
			return err
		}

		child_resources = append(child_resources, tarbacv1.ChildResource{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
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
		utils.LogErrorUID(logger, err, "Failed to update TemporaryRBAC status after ensuring bindings", requestId, "childResources", child_resources)
		return err
	}

	// r.Recorder.Event(tempRBAC, "Normal", "PermissionsGranted", fmt.Sprintf("Temporary permissions were granted in namespace %s [UID: %s]", tempRBAC.ObjectMeta.Namespace, requestId))
	eventMessage := fmt.Sprintf("Temporary permissions were granted for %s in namespace %s", tempRBAC.Name, tempRBAC.Namespace)
	r.Recorder.Event(tempRBAC, "Normal", "PermissionsGranted", utils.FormatEventMessage(eventMessage, requestId))
	logger.Info("Successfully ensured bindings and updated status", "TemporaryRBAC", tempRBAC.Name)
	return nil
}

// cleanupBindings deletes the RoleBinding or ClusterRoleBinding associated with the TemporaryRBAC resource
func (r *TemporaryRBACReconciler) cleanupBindings(ctx context.Context, tempRBAC *tarbacv1.TemporaryRBAC, requestId string) error {
	logger := log.FromContext(ctx)
	var remainingChildResources []tarbacv1.ChildResource

	if len(tempRBAC.Status.ChildResource) > 0 {
		for _, child := range tempRBAC.Status.ChildResource {
			utils.LogInfoUID(logger, "Cleaning up child resource", requestId, "kind", child.Kind, "name", child.Name, "namespace", child.Namespace)

			switch child.Kind {
			case "RoleBinding":
				// Ensure namespace is not empty
				if child.Namespace == "" {
					utils.LogErrorUID(logger, nil, fmt.Sprintf("namespace is empty for RoleBinding: %s", child.Name), requestId)
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
					utils.LogErrorUID(logger, err, "Failed to delete RoleBinding", requestId, "kind", child.Kind, "name", child.Name, "namespace", child.Namespace)
					// Keep the resource in the list if deletion fails
					remainingChildResources = append(remainingChildResources, child)
					continue
				}
				utils.LogInfoUID(logger, "Successfully deleted RoleBinding", requestId, "kind", child.Kind, "name", child.Name, "namespace", child.Namespace)
				// r.Recorder.Event(tempRBAC, "Normal", "PermissionsRevoked", fmt.Sprintf("Temporary permissions were revoked in namespace %s [UID: %s]", tempRBAC.ObjectMeta.Namespace, requestId))
				eventMessage := fmt.Sprintf("Temporary permissions were revoked for %s in namespace %s", tempRBAC.Name, tempRBAC.Namespace)
				r.Recorder.Event(tempRBAC, "Normal", "PermissionsRevoked", utils.FormatEventMessage(eventMessage, requestId))
			default:
				utils.LogErrorUID(logger, nil, "Unsupported child resource kind", requestId, "kind", child.Kind)
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
		utils.LogInfoUID(logger, "RetentionPolicy is set to delete, deleting TemporaryRBAC resource", requestId, "kind", tempRBAC.Kind, "name", tempRBAC.Name, "namespace", tempRBAC.Namespace)
		if err := r.Client.Delete(ctx, tempRBAC); err != nil {
			utils.LogErrorUID(logger, err, "Failed to delete TemporaryRBAC resource", requestId)
			return err
		}
		return nil // Exit since resource is deleted
	}

	// Update status in Kubernetes
	if err := r.Status().Update(ctx, tempRBAC); err != nil {
		utils.LogErrorUID(logger, err, "Failed to update TemporaryRBAC status", requestId)
		return err
	}

	utils.LogInfoUID(logger, "TemporaryRBAC status updated", requestId, "kind", tempRBAC.Kind, "name", tempRBAC.Name, "namespace", tempRBAC.Namespace, "state", tempRBAC.Status.State)
	return nil
}

func (r *TemporaryRBACReconciler) fetchAndSetRequestID(ctx context.Context, tempRBAC *tarbacv1.TemporaryRBAC, requestId string) error {
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
					utils.LogErrorUID(logger, err, "Failed to update TemporaryRBAC labels with RequestID", requestId, "TemporaryRBAC", tempRBAC.Name)
					return err
				}

				tempRBAC.Status.RequestID = ownerRequestID
				if err := r.Status().Update(ctx, tempRBAC); err != nil {
					utils.LogErrorUID(logger, err, "Failed to update TemporaryRBAC status with RequestID", requestId, "TemporaryRBAC", tempRBAC.Name)
					return err
				}
				utils.LogInfoUID(logger, "TemporaryRBAC status updated with RequestID from owner", requestId, "TemporaryRBAC", tempRBAC.Name, "ownerRef", ownerRef.Name, "ownerRefKind", ownerRef.Kind)
				break
			}
		}
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager
func (r *TemporaryRBACReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("TemporaryRBACController")
	return ctrl.NewControllerManagedBy(mgr).
		For(&tarbacv1.TemporaryRBAC{}).
		Complete(r)
}
