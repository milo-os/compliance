// SPDX-License-Identifier: AGPL-3.0-only

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SubprocessorSpec defines the desired state of a Subprocessor. The
// compliance controller owns and manages all fields; this resource is
// not intended to be created or edited directly by staff.
type SubprocessorSpec struct {
	// VendorRef is the name of the Vendor resource from which this Subprocessor
	// was derived.
	//
	// +kubebuilder:validation:Required
	VendorRef string `json:"vendorRef"`
}

// SubprocessorDisclosure is the public disclosure surface projected from a
// Vendor's compliance profile. It contains only fields that are safe for
// unauthenticated consumption; sensitive staff-only fields remain on the
// Vendor record.
type SubprocessorDisclosure struct {
	// DisplayName is the vendor's public-facing name.
	DisplayName string `json:"displayName"`

	// LegalEntity is the registered legal name of the vendor entity.
	LegalEntity string `json:"legalEntity"`

	// CountryOfIncorporation is the ISO 3166-1 alpha-2 country code of the
	// vendor's incorporating jurisdiction.
	CountryOfIncorporation string `json:"countryOfIncorporation"`

	// Website is the vendor's primary public website.
	//
	// +optional
	Website string `json:"website,omitempty"`

	// Purpose describes what the vendor does with personal data.
	Purpose string `json:"purpose"`

	// DataCategories enumerates the personal data categories processed by this
	// vendor.
	DataCategories []DataCategory `json:"dataCategories"`

	// ProcessingRegions lists the regions where data is processed.
	//
	// +optional
	ProcessingRegions []string `json:"processingRegions,omitempty"`

	// TransferMechanism is the legal basis for international data transfers to
	// this vendor.
	TransferMechanism TransferMechanism `json:"transferMechanism"`

	// EffectiveDate is when this subprocessor relationship became effective.
	//
	// +optional
	EffectiveDate *metav1.Time `json:"effectiveDate,omitempty"`

	// Phase is the current lifecycle state. Only Active subprocessors appear
	// in the public feed at launch.
	Phase ComplianceProfilePhase `json:"phase"`
}

// SubprocessorStatus defines the observed state of a Subprocessor.
type SubprocessorStatus struct {
	// ObservedGeneration is the most recent generation observed for this
	// Subprocessor by the controller.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Disclosure is the public disclosure surface projected from the source
	// Vendor's compliance profile. Populated by the compliance controller.
	//
	// +optional
	Disclosure *SubprocessorDisclosure `json:"disclosure,omitempty"`

	// Conditions represents the observations of a Subprocessor's current
	// state. Known condition types: "Ready".
	//
	// +kubebuilder:default={{type: "Ready", status: "Unknown", reason: "Unknown", message: "Waiting for control plane to reconcile", lastTransitionTime: "1970-01-01T00:00:00Z"}}
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=subprocessors,scope=Cluster,categories=compliance,singular=subprocessor
// +kubebuilder:printcolumn:name="Vendor",type="string",JSONPath=".spec.vendorRef"
// +kubebuilder:printcolumn:name="Display Name",type="string",JSONPath=".status.disclosure.displayName"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.disclosure.phase"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"

// Subprocessor is a controller-derived resource representing a publicly
// disclosed subprocessor. It is generated automatically by the compliance
// controller for every Vendor whose compliance profile is in the Active phase.
// The status.disclosure field contains the public disclosure surface; the
// source Vendor record is not exposed.
//
// This resource is the compliance service's canonical source of truth for
// the subprocessor list; the marketing site renders directly from it.
type Subprocessor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SubprocessorSpec   `json:"spec,omitempty"`
	Status SubprocessorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SubprocessorList contains a list of Subprocessor.
type SubprocessorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Subprocessor `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Subprocessor{}, &SubprocessorList{})
}
