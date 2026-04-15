// SPDX-License-Identifier: AGPL-3.0-only

package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	complianceV1alpha1 "go.datum.net/compliance-service/api/v1alpha1"
)

const (
	vendorFinalizer = "compliance.datumapis.com/vendor-protection"
)

// VendorReconciler reconciles Vendor resources and derives corresponding
// Subprocessor resources for vendors with an Active compliance profile.
type VendorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

// +kubebuilder:rbac:groups=compliance.datumapis.com,resources=vendors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=compliance.datumapis.com,resources=vendors/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=compliance.datumapis.com,resources=vendors/finalizers,verbs=update
// +kubebuilder:rbac:groups=compliance.datumapis.com,resources=subprocessors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=compliance.datumapis.com,resources=subprocessors/status,verbs=get;update;patch

func (r *VendorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("vendor", req.NamespacedName)

	vendor := &complianceV1alpha1.Vendor{}
	if err := r.Get(ctx, req.NamespacedName, vendor); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("fetching vendor: %w", err)
	}

	// Handle deletion: remove the derived Subprocessor if it exists.
	if !vendor.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, log, vendor)
	}

	// Ensure finalizer is present so we can clean up the Subprocessor on delete.
	if !controllerutil.ContainsFinalizer(vendor, vendorFinalizer) {
		controllerutil.AddFinalizer(vendor, vendorFinalizer)
		if err := r.Update(ctx, vendor); err != nil {
			return ctrl.Result{}, fmt.Errorf("adding finalizer: %w", err)
		}
		return ctrl.Result{}, nil
	}

	// Reconcile the derived Subprocessor based on profile state.
	if err := r.reconcileSubprocessor(ctx, log, vendor); err != nil {
		return ctrl.Result{}, err
	}

	return r.updateStatus(ctx, log, vendor)
}

func (r *VendorReconciler) handleDeletion(ctx context.Context, log logr.Logger, vendor *complianceV1alpha1.Vendor) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(vendor, vendorFinalizer) {
		return ctrl.Result{}, nil
	}

	// Delete derived Subprocessor if it exists.
	sub := &complianceV1alpha1.Subprocessor{}
	if err := r.Get(ctx, client.ObjectKey{Name: vendor.Name}, sub); err == nil {
		log.Info("deleting derived subprocessor", "subprocessor", vendor.Name)
		if err := r.Delete(ctx, sub); err != nil && !apierrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("deleting subprocessor: %w", err)
		}
	} else if !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("fetching subprocessor for deletion: %w", err)
	}

	controllerutil.RemoveFinalizer(vendor, vendorFinalizer)
	if err := r.Update(ctx, vendor); err != nil {
		return ctrl.Result{}, fmt.Errorf("removing finalizer: %w", err)
	}
	return ctrl.Result{}, nil
}

func (r *VendorReconciler) reconcileSubprocessor(ctx context.Context, log logr.Logger, vendor *complianceV1alpha1.Vendor) error {
	profile := vendor.Spec.ComplianceProfile

	// No profile, or profile is not Active: ensure no Subprocessor exists.
	if profile == nil || profile.Phase != complianceV1alpha1.ComplianceProfilePhaseActive {
		return r.deleteSubprocessorIfExists(ctx, log, vendor.Name)
	}

	// Profile is Active: create or update the derived Subprocessor.
	desired := r.buildSubprocessor(vendor)

	existing := &complianceV1alpha1.Subprocessor{}
	err := r.Get(ctx, client.ObjectKey{Name: vendor.Name}, existing)
	if apierrors.IsNotFound(err) {
		log.Info("creating derived subprocessor", "subprocessor", vendor.Name)
		if err := r.Create(ctx, desired); err != nil {
			return fmt.Errorf("creating subprocessor: %w", err)
		}
		return r.patchSubprocessorStatus(ctx, desired)
	}
	if err != nil {
		return fmt.Errorf("fetching subprocessor: %w", err)
	}

	// Update spec if vendorRef drifted (shouldn't happen, but be safe).
	existing.Spec = desired.Spec
	if err := r.Update(ctx, existing); err != nil {
		return fmt.Errorf("updating subprocessor spec: %w", err)
	}

	// Always sync the disclosure status.
	existing.Status.Disclosure = desired.Status.Disclosure
	return r.patchSubprocessorStatus(ctx, existing)
}

func (r *VendorReconciler) deleteSubprocessorIfExists(ctx context.Context, log logr.Logger, name string) error {
	sub := &complianceV1alpha1.Subprocessor{}
	if err := r.Get(ctx, client.ObjectKey{Name: name}, sub); apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("fetching subprocessor: %w", err)
	}
	log.Info("removing subprocessor: compliance profile no longer active", "subprocessor", name)
	if err := r.Delete(ctx, sub); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting subprocessor: %w", err)
	}
	return nil
}

func (r *VendorReconciler) buildSubprocessor(vendor *complianceV1alpha1.Vendor) *complianceV1alpha1.Subprocessor {
	profile := vendor.Spec.ComplianceProfile

	disclosure := &complianceV1alpha1.SubprocessorDisclosure{
		DisplayName:            vendor.Spec.DisplayName,
		LegalEntity:            vendor.Spec.LegalEntity,
		CountryOfIncorporation: vendor.Spec.CountryOfIncorporation,
		Website:                vendor.Spec.Website,
		Purpose:                profile.Purpose,
		DataCategories:         append([]complianceV1alpha1.DataCategory(nil), profile.DataCategories...),
		ProcessingRegions:      append([]string(nil), profile.ProcessingRegions...),
		TransferMechanism:      profile.TransferMechanism,
		Phase:                  profile.Phase,
	}
	if profile.EffectiveDate != nil {
		t := profile.EffectiveDate.DeepCopy()
		disclosure.EffectiveDate = t
	}

	sub := &complianceV1alpha1.Subprocessor{
		ObjectMeta: metav1.ObjectMeta{
			Name: vendor.Name,
		},
		Spec: complianceV1alpha1.SubprocessorSpec{
			VendorRef: vendor.Name,
		},
		Status: complianceV1alpha1.SubprocessorStatus{
			Disclosure: disclosure,
		},
	}
	return sub
}

func (r *VendorReconciler) patchSubprocessorStatus(ctx context.Context, sub *complianceV1alpha1.Subprocessor) error {
	if err := r.Status().Update(ctx, sub); err != nil {
		return fmt.Errorf("updating subprocessor status: %w", err)
	}
	return nil
}

func (r *VendorReconciler) updateStatus(ctx context.Context, log logr.Logger, vendor *complianceV1alpha1.Vendor) (ctrl.Result, error) {
	patch := client.MergeFrom(vendor.DeepCopy())

	vendor.Status.ObservedGeneration = vendor.Generation

	profile := vendor.Spec.ComplianceProfile
	if profile != nil && profile.Phase == complianceV1alpha1.ComplianceProfilePhaseActive {
		vendor.Status.SubprocessorRef = vendor.Name
	} else {
		vendor.Status.SubprocessorRef = ""
	}

	readyCondition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "Reconciled",
		Message:            "Vendor reconciled successfully",
		ObservedGeneration: vendor.Generation,
		LastTransitionTime: metav1.Now(),
	}

	existing := findCondition(vendor.Status.Conditions, "Ready")
	if existing != nil && existing.Status == readyCondition.Status {
		readyCondition.LastTransitionTime = existing.LastTransitionTime
	}

	setCondition(&vendor.Status.Conditions, readyCondition)

	if err := r.Status().Patch(ctx, vendor, patch); err != nil {
		log.Error(err, "updating vendor status")
		return ctrl.Result{}, fmt.Errorf("updating vendor status: %w", err)
	}
	return ctrl.Result{}, nil
}

func (r *VendorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&complianceV1alpha1.Vendor{}).
		Owns(&complianceV1alpha1.Subprocessor{}).
		Complete(r)
}

// findCondition returns a pointer to the condition with the given type, or nil.
func findCondition(conditions []metav1.Condition, condType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}

// setCondition upserts a condition into the slice by type.
func setCondition(conditions *[]metav1.Condition, condition metav1.Condition) {
	for i, c := range *conditions {
		if c.Type == condition.Type {
			(*conditions)[i] = condition
			return
		}
	}
	*conditions = append(*conditions, condition)
}
