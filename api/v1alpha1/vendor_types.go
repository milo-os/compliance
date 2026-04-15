// SPDX-License-Identifier: AGPL-3.0-only

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DataCategory is an enumerated taxonomy of personal data categories processed
// by a vendor. This taxonomy is load-bearing: retention, residency, and ROPA
// capabilities key off these values, so additions are preferred over renames.
//
// +kubebuilder:validation:Enum=identity;authentication;telemetry;billing;user-content;access-logs;audit-trail
type DataCategory string

const (
	// DataCategoryIdentity covers names, email addresses, and other identifying
	// attributes for users and organizations.
	DataCategoryIdentity DataCategory = "identity"

	// DataCategoryAuthentication covers credentials, tokens, session data, and
	// MFA factors.
	DataCategoryAuthentication DataCategory = "authentication"

	// DataCategoryTelemetry covers usage metrics, performance data, and
	// diagnostic information.
	DataCategoryTelemetry DataCategory = "telemetry"

	// DataCategoryBilling covers payment methods, invoices, and financial
	// transaction records.
	DataCategoryBilling DataCategory = "billing"

	// DataCategoryUserContent covers content created or uploaded by users,
	// such as configuration data, documents, or application payloads.
	DataCategoryUserContent DataCategory = "user-content"

	// DataCategoryAccessLogs covers records of who accessed what and when,
	// including API call logs and authentication events.
	DataCategoryAccessLogs DataCategory = "access-logs"

	// DataCategoryAuditTrail covers immutable records of administrative and
	// compliance-relevant actions.
	DataCategoryAuditTrail DataCategory = "audit-trail"
)

// DataSubjectType identifies the category of individuals whose data is processed.
//
// +kubebuilder:validation:Enum=organization-admin;consumer;platform-staff
type DataSubjectType string

const (
	// DataSubjectTypeOrgAdmin refers to administrators of organizations using
	// the platform.
	DataSubjectTypeOrgAdmin DataSubjectType = "organization-admin"

	// DataSubjectTypeConsumer refers to end users of service providers built on
	// the platform.
	DataSubjectTypeConsumer DataSubjectType = "consumer"

	// DataSubjectTypePlatformStaff refers to Datum Cloud staff who operate the
	// platform.
	DataSubjectTypePlatformStaff DataSubjectType = "platform-staff"
)

// TransferMechanism identifies the legal basis under which personal data is
// transferred to a vendor outside the EEA.
//
// +kubebuilder:validation:Enum=SCCs;AdequacyDecision;BCRs
type TransferMechanism string

const (
	// TransferMechanismSCCs indicates transfer under Standard Contractual
	// Clauses approved by the European Commission.
	TransferMechanismSCCs TransferMechanism = "SCCs"

	// TransferMechanismAdequacyDecision indicates transfer to a country or
	// territory covered by a European Commission adequacy decision.
	TransferMechanismAdequacyDecision TransferMechanism = "AdequacyDecision"

	// TransferMechanismBCRs indicates transfer under Binding Corporate Rules
	// approved by a supervisory authority.
	TransferMechanismBCRs TransferMechanism = "BCRs"
)

// RiskTier is a staff-assigned classification of the risk a vendor poses to
// the platform's compliance posture.
//
// +kubebuilder:validation:Enum=Low;Medium;High;Critical
type RiskTier string

const (
	RiskTierLow      RiskTier = "Low"
	RiskTierMedium   RiskTier = "Medium"
	RiskTierHigh     RiskTier = "High"
	RiskTierCritical RiskTier = "Critical"
)

// ComplianceProfilePhase describes the lifecycle state of a vendor's compliance
// profile.
//
// +kubebuilder:validation:Enum=Draft;Active
type ComplianceProfilePhase string

const (
	// ComplianceProfilePhaseDraft indicates the profile is under internal
	// preparation. No Subprocessor resource is generated; the vendor is not
	// publicly disclosed.
	ComplianceProfilePhaseDraft ComplianceProfilePhase = "Draft"

	// ComplianceProfilePhaseActive indicates the profile is live. A
	// corresponding Subprocessor resource is generated and appears in the
	// public disclosure feed.
	ComplianceProfilePhaseActive ComplianceProfilePhase = "Active"
)

// ComplianceProfile captures the regulatory overlay for a Vendor that
// processes personal data. Its presence on a Vendor record is what causes the
// compliance controller to derive a Subprocessor.
type ComplianceProfile struct {
	// Purpose describes what the vendor does with personal data and why it is
	// necessary for platform operation.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=1000
	Purpose string `json:"purpose"`

	// DataCategories enumerates the categories of personal data processed by
	// this vendor. At least one category is required.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	DataCategories []DataCategory `json:"dataCategories"`

	// DataSubjectTypes identifies the categories of individuals whose data is
	// processed.
	//
	// +kubebuilder:validation:Optional
	DataSubjectTypes []DataSubjectType `json:"dataSubjectTypes,omitempty"`

	// ProcessingRegions lists the countries or regions where the vendor
	// processes data, using ISO 3166-1 alpha-2 country codes or named regions
	// (e.g., "US", "EU").
	//
	// +kubebuilder:validation:Optional
	ProcessingRegions []string `json:"processingRegions,omitempty"`

	// TransferMechanism is the legal basis for transferring personal data to
	// this vendor where an international transfer occurs.
	//
	// +kubebuilder:validation:Required
	TransferMechanism TransferMechanism `json:"transferMechanism"`

	// RiskTier is a staff-assigned risk classification for this vendor.
	//
	// +kubebuilder:validation:Required
	RiskTier RiskTier `json:"riskTier"`

	// DPAReference is an opaque reference (e.g., URL or document identifier)
	// to the Data Processing Agreement with this vendor. A compliance profile
	// without a DPA reference is incomplete.
	//
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=2048
	DPAReference string `json:"dpaReference,omitempty"`

	// EffectiveDate is when this compliance profile became or will become
	// effective.
	//
	// +kubebuilder:validation:Optional
	EffectiveDate *metav1.Time `json:"effectiveDate,omitempty"`

	// Phase is the current lifecycle state of this compliance profile.
	// Transitioning to Active causes the controller to generate a Subprocessor.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:default=Draft
	Phase ComplianceProfilePhase `json:"phase"`
}

// VendorSpec defines the desired state of a Vendor.
type VendorSpec struct {
	// DisplayName is the human-readable name of the vendor as it will appear
	// in public disclosures (e.g., "Amazon Web Services").
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=256
	DisplayName string `json:"displayName"`

	// LegalEntity is the registered legal name of the vendor entity that is
	// party to the DPA.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=512
	LegalEntity string `json:"legalEntity"`

	// CountryOfIncorporation is the ISO 3166-1 alpha-2 country code of the
	// jurisdiction where the vendor's legal entity is incorporated.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=2
	// +kubebuilder:validation:MaxLength=2
	// +kubebuilder:validation:Pattern=`^[A-Z]{2}$`
	CountryOfIncorporation string `json:"countryOfIncorporation"`

	// Website is the vendor's primary public website.
	//
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=2048
	Website string `json:"website,omitempty"`

	// ComplianceProfile is the optional regulatory overlay for this vendor.
	// Its presence indicates the vendor processes personal data on behalf of
	// Datum Cloud and triggers generation of a Subprocessor resource when the
	// profile phase is Active.
	//
	// +kubebuilder:validation:Optional
	ComplianceProfile *ComplianceProfile `json:"complianceProfile,omitempty"`
}

// VendorStatus defines the observed state of a Vendor.
type VendorStatus struct {
	// ObservedGeneration is the most recent generation observed for this Vendor
	// by the controller.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// SubprocessorRef holds the name of the derived Subprocessor resource when
	// the vendor's compliance profile is Active. Empty when no Subprocessor
	// has been generated.
	//
	// +optional
	SubprocessorRef string `json:"subprocessorRef,omitempty"`

	// Conditions represents the observations of a Vendor's current state.
	// Known condition types: "Ready", "SubprocessorSynced".
	//
	// +kubebuilder:default={{type: "Ready", status: "Unknown", reason: "Unknown", message: "Waiting for control plane to reconcile", lastTransitionTime: "1970-01-01T00:00:00Z"}}
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=vendors,scope=Cluster,categories=compliance,singular=vendor
// +kubebuilder:printcolumn:name="Display Name",type="string",JSONPath=".spec.displayName"
// +kubebuilder:printcolumn:name="Legal Entity",type="string",JSONPath=".spec.legalEntity"
// +kubebuilder:printcolumn:name="Country",type="string",JSONPath=".spec.countryOfIncorporation"
// +kubebuilder:printcolumn:name="Profile Phase",type="string",JSONPath=".spec.complianceProfile.phase"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"

// Vendor is a staff-managed record representing a third-party vendor in Datum
// Cloud's platform supply chain. Vendors that process personal data carry a
// compliance profile, which the compliance controller uses to derive a
// publicly-disclosed Subprocessor resource.
type Vendor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   VendorSpec   `json:"spec,omitempty"`
	Status VendorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VendorList contains a list of Vendor.
type VendorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Vendor `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Vendor{}, &VendorList{})
}
