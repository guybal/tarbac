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
		logger.Error(err, "Unable to fetch TemporaryRBAC")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Parse the duration from the spec
	duration, err := time.ParseDuration(tempRBAC.Spec.Duration)
	if err != nil {
		logger.Error(err, "Invalid duration in TemporaryRBAC spec", "duration", tempRBAC.Spec.Duration)
		return ctrl.Result{}, err
	}

	// Calculate expiration time
	expiration := tempRBAC.CreationTimestamp.Add(duration)

	// **Check if the resource is expired**
	if time.Now().After(expiration) {
		logger.Info("TemporaryRBAC expired, cleaning up resources", "name", tempRBAC.Name)
		if err := r.cleanupBindings(ctx, &tempRBAC); err != nil {
			logger.Error(err, "Failed to clean up associated bindings")
			return ctrl.Result{}, err
		}
		if err := r.Delete(ctx, &tempRBAC); err != nil {
			logger.Error(err, "Failed to delete TemporaryRBAC resource")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Ensure RoleBinding or ClusterRoleBinding exists
	if err := r.ensureBindings(ctx, &tempRBAC); err != nil {
		logger.Error(err, "Failed to ensure bindings for TemporaryRBAC")
		return ctrl.Result{}, err
	}

	// Update status to "Created"
	tempRBAC.Status.State = "Created"
	if err := r.Status().Update(ctx, &tempRBAC); err != nil {
		logger.Error(err, "Failed to update TemporaryRBAC status")
		return ctrl.Result{}, err
	}

	// Requeue for expiration
	logger.Info("TemporaryRBAC successfully reconciled, requeueing for expiration", "name", tempRBAC.Name)
	return ctrl.Result{RequeueAfter: time.Until(expiration)}, nil
}

// ensureBindings creates or updates the RoleBinding or ClusterRoleBinding for the TemporaryRBAC resource
func (r *TemporaryRBACReconciler) ensureBindings(ctx context.Context, tempRBAC *tarbacv1.TemporaryRBAC) error {
	var binding client.Object
	if tempRBAC.Spec.RoleRef.Kind == "ClusterRole" {
		binding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tempRBAC.Name,
				Namespace: tempRBAC.Namespace,
			},
			Subjects: []rbacv1.Subject{tempRBAC.Spec.Subject},
			RoleRef:  tempRBAC.Spec.RoleRef,
		}
	} else {
		binding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tempRBAC.Name,
				Namespace: tempRBAC.Namespace,
			},
			Subjects: []rbacv1.Subject{tempRBAC.Spec.Subject},
			RoleRef:  tempRBAC.Spec.RoleRef,
		}
	}

	err := r.Client.Create(ctx, binding)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	// Update the TemporaryRBAC status
	tempRBAC.Status.State = "Created"
	tempRBAC.Status.ChildResource = &tarbacv1.ChildResource{
		APIVersion: binding.GetObjectKind().GroupVersionKind().GroupVersion().String(),
		Kind:       binding.GetObjectKind().GroupVersionKind().Kind,
		Name:       binding.GetName(),
	}

	return r.Client.Status().Update(ctx, tempRBAC)
}

// cleanupBindings deletes the RoleBinding or ClusterRoleBinding associated with the TemporaryRBAC resource
func (r *TemporaryRBACReconciler) cleanupBindings(ctx context.Context, tempRBAC *tarbacv1.TemporaryRBAC) error {
	if tempRBAC.Spec.RoleRef.Kind == "ClusterRole" {
		err := r.Client.Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: tempRBAC.Name}})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	} else {
		err := r.Client.Delete(ctx, &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: tempRBAC.Name}})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	// Update the status to Expired
	tempRBAC.Status.State = "Expired"
	tempRBAC.Status.ChildResource = nil
	return r.Client.Status().Update(ctx, tempRBAC)
}


