package controllers

import (
	"context"
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

	// Log the full resource for debugging
	logger.Info("Current TemporaryRBAC object state", "TemporaryRBAC", tempRBAC)

	// Parse the duration from the spec
	duration, err := time.ParseDuration(tempRBAC.Spec.Duration)
	if err != nil {
		logger.Error(err, "Invalid duration in TemporaryRBAC spec", "duration", tempRBAC.Spec.Duration)
		return ctrl.Result{}, err
	}

	// Calculate expiration time from `status.createdAt`
	var expiration time.Time
	if tempRBAC.Status.CreatedAt != nil {
		expiration = tempRBAC.Status.CreatedAt.Time.Add(duration)
	} else {
		// If `createdAt` is not set, default to resource's creation timestamp
		tempRBAC.Status.CreatedAt = &metav1.Time{Time: tempRBAC.CreationTimestamp.Time}
		expiration = tempRBAC.CreationTimestamp.Add(duration)
	}

	logger.Info("Calculated expiration time", "expiration", expiration)

	// Check if the resource is expired
	if time.Now().After(expiration) {
		logger.Info("TemporaryRBAC expired, cleaning up associated bindings", "name", tempRBAC.Name)

		// Update the status to Expired before cleanup
		tempRBAC.Status.State = "Expired"
		if err := r.Status().Update(ctx, &tempRBAC); err != nil {
			logger.Error(err, "Failed to update TemporaryRBAC status to Expired")
			return ctrl.Result{}, err
		}

		// Clean up associated bindings
		if err := r.cleanupBindings(ctx, &tempRBAC); err != nil {
			logger.Error(err, "Failed to clean up bindings for expired TemporaryRBAC")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Ensure RoleBinding or ClusterRoleBinding exists
	if err := r.ensureBindings(ctx, &tempRBAC, duration); err != nil {
		logger.Error(err, "Failed to ensure bindings for TemporaryRBAC")
		return ctrl.Result{}, err
	}

	// Set the state to Created if bindings are successfully ensured
	tempRBAC.Status.State = "Created"
	if err := r.Status().Update(ctx, &tempRBAC); err != nil {
		logger.Error(err, "Failed to update TemporaryRBAC status after creating bindings")
		return ctrl.Result{}, err
	}

	// Log the final state of TemporaryRBAC
	logger.Info("Final TemporaryRBAC state before requeueing", "TemporaryRBAC", tempRBAC)

	// Requeue for expiration
	timeUntilExpiration := time.Until(expiration)
	logger.Info("TemporaryRBAC successfully reconciled, requeueing for expiration", "name", tempRBAC.Name)
	return ctrl.Result{RequeueAfter: timeUntilExpiration.Truncate(time.Second)}, nil
}

// ensureBindings creates or updates the RoleBinding or ClusterRoleBinding for the TemporaryRBAC resource
func (r *TemporaryRBACReconciler) ensureBindings(ctx context.Context, tempRBAC *tarbacv1.TemporaryRBAC, duration time.Duration) error {
	logger := log.FromContext(ctx)

	var binding client.Object

	if tempRBAC.Spec.RoleRef.Kind == "ClusterRole" {
		binding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: tempRBAC.Name,
			},
			Subjects: []rbacv1.Subject{tempRBAC.Spec.Subject},
			RoleRef:  tempRBAC.Spec.RoleRef,
		}
		binding.GetObjectKind().SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("ClusterRoleBinding"))
	} else {
		binding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tempRBAC.Name,
				Namespace: tempRBAC.Namespace,
			},
			Subjects: []rbacv1.Subject{tempRBAC.Spec.Subject},
			RoleRef:  tempRBAC.Spec.RoleRef,
		}
		binding.GetObjectKind().SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("RoleBinding"))
	}

	// Attempt to create the binding
	err := r.Client.Create(ctx, binding)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	// If the resource already exists, fetch its current state
	if apierrors.IsAlreadyExists(err) {
		logger.Info("Binding already exists, fetching existing resource", "name", binding.GetName(), "namespace", binding.GetNamespace())
		if err := r.Client.Get(ctx, client.ObjectKey{Name: binding.GetName(), Namespace: binding.GetNamespace()}, binding); err != nil {
			logger.Error(err, "Failed to fetch existing binding")
			return err
		}
	}

	// Update the TemporaryRBAC status with the child resource details
	tempRBAC.Status.ChildResource = &tarbacv1.ChildResource{
		APIVersion: binding.GetObjectKind().GroupVersionKind().GroupVersion().String(),
		Kind:       binding.GetObjectKind().GroupVersionKind().Kind,
		Name:       binding.GetName(),
		Namespace:  binding.GetNamespace(),
	}

	// Set the creation timestamp based on the child resource
	tempRBAC.Status.CreatedAt = &metav1.Time{Time: binding.GetCreationTimestamp().Time}

	// Calculate expiration time
	expiration := binding.GetCreationTimestamp().Add(duration)
	tempRBAC.Status.ExpiresAt = &metav1.Time{Time: expiration}

	logger.Info("Binding Details After Initialization",
		"GroupVersionKind", binding.GetObjectKind().GroupVersionKind(),
		"Name", binding.GetName(),
		"Namespace", binding.GetNamespace(),
		"CreatedAt", tempRBAC.Status.CreatedAt,
		"ExpiresAt", tempRBAC.Status.ExpiresAt,
	)

	// Update the status of the TemporaryRBAC
	if err := r.Status().Update(ctx, tempRBAC); err != nil {
		logger.Error(err, "Failed to update TemporaryRBAC status")
		return err
	}

	return nil
}

// cleanupBindings deletes the RoleBinding or ClusterRoleBinding associated with the TemporaryRBAC resource
func (r *TemporaryRBACReconciler) cleanupBindings(ctx context.Context, tempRBAC *tarbacv1.TemporaryRBAC) error {
	logger := log.FromContext(ctx)

	// Delete the associated RoleBinding or ClusterRoleBinding
	if tempRBAC.Spec.RoleRef.Kind == "ClusterRole" {
		logger.Info("Cleaning up ClusterRoleBinding", "name", tempRBAC.Name)
		err := r.Client.Delete(ctx, &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: tempRBAC.Name},
		})
		if err != nil && !apierrors.IsNotFound(err) {
			logger.Error(err, "Failed to delete associated ClusterRoleBinding")
			return err
		}
	} else {
		logger.Info("Cleaning up RoleBinding", "name", tempRBAC.Name)
		err := r.Client.Delete(ctx, &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tempRBAC.Name,
				Namespace: tempRBAC.Namespace,
			},
		})
		if err != nil && !apierrors.IsNotFound(err) {
			logger.Error(err, "Failed to delete associated RoleBinding")
			return err
		}
	}

	// Update the status of the TemporaryRBAC to "Expired"
	tempRBAC.Status.State = "Expired"
	tempRBAC.Status.ChildResource = nil // Clear child resource reference

	if err := r.Status().Update(ctx, tempRBAC); err != nil {
		logger.Error(err, "Failed to update TemporaryRBAC status to Expired")
		return err
	}

	logger.Info("Successfully cleaned up bindings and updated status", "TemporaryRBAC", tempRBAC.Name)
	return nil
}

// SetupWithManager sets up the controller with the Manager
func (r *TemporaryRBACReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tarbacv1.TemporaryRBAC{}).
		Complete(r)
}
