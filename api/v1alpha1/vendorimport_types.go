// SPDX-License-Identifier: AGPL-3.0-only

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Annotation keys set on Vendor records that originate from a VendorImport.
const (
	// AnnotationImportedFrom records the originating adapter (e.g. "ramp")
	// for a Vendor created by an import. Useful for queries / filters and
	// for the controller's "is this Vendor adapter-owned?" check on re-sync.
	AnnotationImportedFrom = "compliance.miloapis.com/imported-from"

	// AnnotationRampVendorID records the stable Ramp vendor identifier on a
	// Vendor created by a Ramp VendorImport. Used to look up the existing
	// Vendor on subsequent syncs without relying on display name.
	AnnotationRampVendorID = "compliance.miloapis.com/ramp-vendor-id"
)

// ImportSource enumerates the third-party data sources we can sync vendors
// from. Each value pairs with a matching configuration field on
// VendorImportSpec.
//
// +kubebuilder:validation:Enum=ramp
type ImportSource string

const (
	// ImportSourceRamp pulls vendors from Ramp's accounting vendors endpoint.
	ImportSourceRamp ImportSource = "ramp"
)

// VendorImportSpec defines the desired state of a VendorImport. The
// reconciler watches these and, for each, calls out to the configured
// third-party source to create or refresh Draft Vendor records.
type VendorImportSpec struct {
	// Source identifies which adapter fulfils this import. Today only
	// "ramp" is supported; additional adapters can be added without breaking
	// existing imports.
	//
	// +kubebuilder:validation:Required
	Source ImportSource `json:"source"`

	// Ramp configures the Ramp adapter when Source is "ramp". Required for
	// that source; ignored otherwise.
	//
	// +optional
	Ramp *RampImportSource `json:"ramp,omitempty"`

	// ResyncInterval controls how often the controller re-runs the import in
	// addition to on-demand triggers (re-applies, edits). The default is 24h.
	// Setting this to zero disables periodic re-sync; the import then only
	// fires when the CR is created or otherwise reconciled.
	//
	// +kubebuilder:validation:Format=duration
	// +optional
	ResyncInterval *metav1.Duration `json:"resyncInterval,omitempty"`
}

// RampImportSource configures the Ramp adapter. Credentials live in a
// Kubernetes Secret referenced by CredentialsRef; the Secret must contain
// `client_id` and `client_secret` keys (case sensitive).
type RampImportSource struct {
	// CredentialsRef points at a Secret holding Ramp's OAuth2
	// client_credentials. The Secret must expose at least `client_id` and
	// `client_secret` data keys. The reconciler reads it on every reconcile
	// so rotated secrets take effect on the next requeue.
	//
	// +kubebuilder:validation:Required
	CredentialsRef corev1.SecretReference `json:"credentialsRef"`

	// Endpoint overrides the Ramp API base URL. Defaults to
	// https://api.ramp.com. Useful for staging/sandbox environments and for
	// tests that point at a mock server.
	//
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// TokenURL overrides the Ramp OAuth2 token endpoint. Defaults to
	// https://api.ramp.com/v1/public/customer/token (the Ramp production
	// token endpoint). Useful for sandbox.
	//
	// +optional
	TokenURL string `json:"tokenURL,omitempty"`
}

// VendorImportStatus defines the observed state of a VendorImport.
type VendorImportStatus struct {
	// ObservedGeneration is the most recent generation observed for this
	// VendorImport by the controller.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// LastSyncTime is the timestamp of the most recent successful import
	// pass.
	//
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// ImportedVendorRefs lists the Vendor resource names this import has
	// created or updated. The list is replaced on every sync — it represents
	// the vendors the source returned in the last pass, not a cumulative
	// history.
	//
	// +optional
	ImportedVendorRefs []string `json:"importedVendorRefs,omitempty"`

	// SkippedActiveRefs lists the Vendor resource names the import would
	// have updated had they been in Draft, but were left untouched because
	// their compliance profile is already Active. Surfacing these lets
	// operators decide whether to re-draft an out-of-date subprocessor.
	//
	// +optional
	SkippedActiveRefs []string `json:"skippedActiveRefs,omitempty"`

	// Conditions represents the observations of a VendorImport's current
	// state. Known condition types: "Ready", "ImportComplete".
	//
	// +kubebuilder:default={{type: "Ready", status: "Unknown", reason: "Unknown", message: "Waiting for control plane to reconcile", lastTransitionTime: "1970-01-01T00:00:00Z"}}
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=vendorimports,scope=Cluster,categories=compliance,singular=vendorimport
// +kubebuilder:metadata:annotations="discovery.miloapis.com/parent-contexts=Platform"
// +kubebuilder:printcolumn:name="Source",type="string",JSONPath=".spec.source"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Last Sync",type="date",JSONPath=".status.lastSyncTime"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"

// VendorImport is a staff-managed request to populate the Vendor catalogue
// from an external source (today: Ramp). The reconciler pulls vendors from
// the configured source on a schedule and creates Draft Vendor records,
// leaving the operator to fill in the compliance-profile fields the source
// doesn't carry. Existing Draft Vendors with a matching annotation are
// refreshed in place; Active Vendors are never overwritten.
type VendorImport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VendorImportSpec   `json:"spec,omitempty"`
	Status VendorImportStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VendorImportList contains a list of VendorImport.
type VendorImportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VendorImport `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VendorImport{}, &VendorImportList{})
}
