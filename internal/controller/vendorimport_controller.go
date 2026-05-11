// SPDX-License-Identifier: AGPL-3.0-only

package controller

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	compliancev1alpha1 "go.miloapis.com/compliance/api/v1alpha1"
	"go.miloapis.com/compliance/internal/adapter/ramp"
)

const (
	// defaultResyncInterval is used when VendorImport.spec.resyncInterval
	// is unset. Picked to balance "operators see new Ramp vendors within
	// a day" against "we don't hammer Ramp at the rate limit on a quiet
	// week".
	defaultResyncInterval = 24 * time.Hour

	// secretKeyRampClientID and secretKeyRampClientSecret are the data
	// keys we read from the credentials Secret.
	secretKeyRampClientID     = "client_id"
	secretKeyRampClientSecret = "client_secret"

	// labelImportedFrom marks Vendor records that originate from a
	// VendorImport. Mirrors AnnotationImportedFrom so List can use a
	// label selector to narrow the search space; annotations are not
	// selectable.
	labelImportedFrom = "compliance.miloapis.com/imported-from"

	// conditionReady mirrors the existing "Ready" convention.
	conditionReady = "Ready"

	// conditionImportComplete signals that a sync pass finished
	// successfully (whether or not it produced any vendor changes).
	conditionImportComplete = "ImportComplete"
)

// VendorImportReconciler watches VendorImport CRs and, for each, pulls
// vendors from the configured source (today: Ramp) and upserts Draft
// Vendor CRs. Active Vendors are never overwritten — they're recorded in
// status.skippedActiveRefs so an operator can decide whether to re-draft.
type VendorImportReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger

	// NowFunc allows tests to control LastSyncTime; defaults to time.Now.
	NowFunc func() time.Time
}

// +kubebuilder:rbac:groups=compliance.miloapis.com,resources=vendorimports,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=compliance.miloapis.com,resources=vendorimports/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=compliance.miloapis.com,resources=vendorimports/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *VendorImportReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("vendorimport", req.NamespacedName)

	vi := &compliancev1alpha1.VendorImport{}
	if err := r.Get(ctx, req.NamespacedName, vi); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("fetching VendorImport: %w", err)
	}

	if !vi.DeletionTimestamp.IsZero() {
		// Nothing to clean up — imported Vendors are intentionally left in
		// place when a VendorImport is removed (operators keep manual
		// control over deletes via the staff portal).
		return ctrl.Result{}, nil
	}

	switch vi.Spec.Source {
	case compliancev1alpha1.ImportSourceRamp:
		return r.reconcileRamp(ctx, log, vi)
	case "":
		return r.markFailure(ctx, vi, "InvalidSpec", "spec.source is required")
	default:
		return r.markFailure(ctx, vi, "UnsupportedSource", fmt.Sprintf("source %q is not supported", vi.Spec.Source))
	}
}

func (r *VendorImportReconciler) reconcileRamp(ctx context.Context, log logr.Logger, vi *compliancev1alpha1.VendorImport) (ctrl.Result, error) {
	if vi.Spec.Ramp == nil {
		return r.markFailure(ctx, vi, "InvalidSpec", "spec.ramp is required when source is ramp")
	}

	clientID, clientSecret, err := r.loadRampCredentials(ctx, vi)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.markFailure(ctx, vi, "CredentialsNotConfigured",
				fmt.Sprintf("Secret %q not found in namespace %q", vi.Spec.Ramp.CredentialsRef.Name, vi.Spec.Ramp.CredentialsRef.Namespace))
		}
		return r.markFailure(ctx, vi, "CredentialsError", err.Error())
	}

	client, err := ramp.NewClient(ramp.Config{
		Endpoint:     vi.Spec.Ramp.Endpoint,
		TokenURL:     vi.Spec.Ramp.TokenURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
	})
	if err != nil {
		return r.markFailure(ctx, vi, "AdapterError", err.Error())
	}

	rampVendors, err := client.ListActiveVendors(ctx)
	if err != nil {
		return r.markFailure(ctx, vi, "SyncFailed", err.Error())
	}
	log.Info("ramp returned vendors", "count", len(rampVendors))

	existing, err := r.listImportedVendors(ctx, string(vi.Spec.Source))
	if err != nil {
		return r.markFailure(ctx, vi, "ListExistingFailed", err.Error())
	}

	imported := make([]string, 0, len(rampVendors))
	skipped := make([]string, 0)

	for _, v := range rampVendors {
		mapped, ok := ramp.MapVendor(v)
		if !ok {
			log.Info("skipping ramp vendor with blank id/name", "rampId", v.ID, "name", v.Name)
			continue
		}

		current, found := existing[mapped.RampVendorID]
		if !found {
			created, err := r.createVendor(ctx, log, vi, mapped)
			if err != nil {
				return r.markFailure(ctx, vi, "CreateFailed", err.Error())
			}
			imported = append(imported, created)
			continue
		}

		if current.Spec.ComplianceProfile != nil &&
			current.Spec.ComplianceProfile.Phase == compliancev1alpha1.ComplianceProfilePhaseActive {
			skipped = append(skipped, current.Name)
			continue
		}

		updated, err := r.updateVendor(ctx, log, current, mapped)
		if err != nil {
			return r.markFailure(ctx, vi, "UpdateFailed", err.Error())
		}
		imported = append(imported, updated)
	}

	sort.Strings(imported)
	sort.Strings(skipped)

	return r.markSuccess(ctx, vi, imported, skipped)
}

func (r *VendorImportReconciler) loadRampCredentials(ctx context.Context, vi *compliancev1alpha1.VendorImport) (string, string, error) {
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      vi.Spec.Ramp.CredentialsRef.Name,
		Namespace: vi.Spec.Ramp.CredentialsRef.Namespace,
	}, secret); err != nil {
		return "", "", err
	}

	clientID := string(secret.Data[secretKeyRampClientID])
	clientSecret := string(secret.Data[secretKeyRampClientSecret])
	if clientID == "" || clientSecret == "" {
		return "", "", errors.New("credentials Secret must contain client_id and client_secret keys")
	}
	return clientID, clientSecret, nil
}

// listImportedVendors returns the Vendor CRs that were previously created
// by an import of the same source, keyed by their source-vendor-id
// annotation. Other Vendors (manually created, imported from a different
// source) are ignored.
func (r *VendorImportReconciler) listImportedVendors(ctx context.Context, source string) (map[string]*compliancev1alpha1.Vendor, error) {
	list := &compliancev1alpha1.VendorList{}
	if err := r.List(ctx, list, client.MatchingLabels{labelImportedFrom: source}); err != nil {
		return nil, fmt.Errorf("listing imported vendors: %w", err)
	}

	out := make(map[string]*compliancev1alpha1.Vendor, len(list.Items))
	for i := range list.Items {
		v := &list.Items[i]
		id := v.Annotations[compliancev1alpha1.AnnotationRampVendorID]
		if id == "" {
			continue
		}
		out[id] = v
	}
	return out, nil
}

func (r *VendorImportReconciler) createVendor(
	ctx context.Context,
	log logr.Logger,
	vi *compliancev1alpha1.VendorImport,
	mapped ramp.MappedVendor,
) (string, error) {
	vendor := &compliancev1alpha1.Vendor{
		ObjectMeta: metav1.ObjectMeta{
			Name: mapped.Name,
			Labels: map[string]string{
				labelImportedFrom: string(vi.Spec.Source),
			},
			Annotations: map[string]string{
				compliancev1alpha1.AnnotationImportedFrom: string(vi.Spec.Source),
				compliancev1alpha1.AnnotationRampVendorID: mapped.RampVendorID,
			},
		},
		Spec: mapped.SpecPatch,
	}
	if err := r.Create(ctx, vendor); err != nil {
		return "", fmt.Errorf("creating vendor %q: %w", mapped.Name, err)
	}
	log.Info("created vendor from ramp", "vendor", vendor.Name, "rampId", mapped.RampVendorID)
	return vendor.Name, nil
}

func (r *VendorImportReconciler) updateVendor(
	ctx context.Context,
	log logr.Logger,
	current *compliancev1alpha1.Vendor,
	mapped ramp.MappedVendor,
) (string, error) {
	patch := client.MergeFrom(current.DeepCopy())

	// Refresh only the adapter-owned identity fields. Operators may have
	// already filled in country during review, so don't clobber a real
	// value with the "UN" sentinel.
	current.Spec.DisplayName = mapped.SpecPatch.DisplayName
	current.Spec.LegalEntity = mapped.SpecPatch.LegalEntity
	if current.Spec.CountryOfIncorporation == "" || current.Spec.CountryOfIncorporation == "UN" {
		current.Spec.CountryOfIncorporation = mapped.SpecPatch.CountryOfIncorporation
	}

	if current.Annotations == nil {
		current.Annotations = map[string]string{}
	}
	current.Annotations[compliancev1alpha1.AnnotationImportedFrom] = string(compliancev1alpha1.ImportSourceRamp)
	current.Annotations[compliancev1alpha1.AnnotationRampVendorID] = mapped.RampVendorID
	if current.Labels == nil {
		current.Labels = map[string]string{}
	}
	current.Labels[labelImportedFrom] = string(compliancev1alpha1.ImportSourceRamp)

	if err := r.Patch(ctx, current, patch); err != nil {
		return "", fmt.Errorf("patching vendor %q: %w", current.Name, err)
	}
	log.Info("refreshed vendor from ramp", "vendor", current.Name, "rampId", mapped.RampVendorID)
	return current.Name, nil
}

func (r *VendorImportReconciler) markFailure(ctx context.Context, vi *compliancev1alpha1.VendorImport, reason, message string) (ctrl.Result, error) {
	now := r.now()
	patch := client.MergeFrom(vi.DeepCopy())
	vi.Status.ObservedGeneration = vi.Generation
	setCondition(&vi.Status.Conditions, metav1.Condition{
		Type:               conditionReady,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: vi.Generation,
		LastTransitionTime: now,
	})
	setCondition(&vi.Status.Conditions, metav1.Condition{
		Type:               conditionImportComplete,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: vi.Generation,
		LastTransitionTime: now,
	})
	if err := r.Status().Patch(ctx, vi, patch); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating VendorImport status: %w", err)
	}
	// Requeue after the configured resync so an operator who fixes the
	// credentials Secret (or whatever else broke) doesn't have to wait
	// for the next manual re-apply.
	return ctrl.Result{RequeueAfter: r.resyncInterval(vi)}, nil
}

func (r *VendorImportReconciler) markSuccess(ctx context.Context, vi *compliancev1alpha1.VendorImport, imported, skipped []string) (ctrl.Result, error) {
	now := r.now()
	patch := client.MergeFrom(vi.DeepCopy())
	vi.Status.ObservedGeneration = vi.Generation
	vi.Status.LastSyncTime = &now
	vi.Status.ImportedVendorRefs = imported
	vi.Status.SkippedActiveRefs = skipped

	setCondition(&vi.Status.Conditions, metav1.Condition{
		Type:               conditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             "Reconciled",
		Message:            "VendorImport reconciled successfully",
		ObservedGeneration: vi.Generation,
		LastTransitionTime: now,
	})
	setCondition(&vi.Status.Conditions, metav1.Condition{
		Type:               conditionImportComplete,
		Status:             metav1.ConditionTrue,
		Reason:             "Reconciled",
		Message:            fmt.Sprintf("Imported %d, skipped %d active", len(imported), len(skipped)),
		ObservedGeneration: vi.Generation,
		LastTransitionTime: now,
	})
	if err := r.Status().Patch(ctx, vi, patch); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating VendorImport status: %w", err)
	}

	return ctrl.Result{RequeueAfter: r.resyncInterval(vi)}, nil
}

func (r *VendorImportReconciler) resyncInterval(vi *compliancev1alpha1.VendorImport) time.Duration {
	if vi.Spec.ResyncInterval == nil {
		return defaultResyncInterval
	}
	if vi.Spec.ResyncInterval.Duration <= 0 {
		return 0
	}
	return vi.Spec.ResyncInterval.Duration
}

func (r *VendorImportReconciler) now() metav1.Time {
	if r.NowFunc != nil {
		return metav1.NewTime(r.NowFunc())
	}
	return metav1.Now()
}

func (r *VendorImportReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&compliancev1alpha1.VendorImport{}).
		Complete(r)
}
