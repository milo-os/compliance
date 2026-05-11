// SPDX-License-Identifier: AGPL-3.0-only

package ramp

import (
	"crypto/sha1" //nolint:gosec // SHA-1 here is used as a stable name suffix, not a security boundary.
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	compliancev1alpha1 "go.miloapis.com/compliance/api/v1alpha1"
)

// MappedVendor is the importer-owned shape produced from a single Ramp
// accounting vendor. The controller is responsible for turning these into
// either a new Vendor CR or a patch over an existing Draft Vendor.
type MappedVendor struct {
	// Name is a deterministic, RFC 1123-safe Kubernetes resource name
	// derived from the Ramp vendor name and ID. Used when creating new
	// Vendor records; existing CRs are looked up by the RampVendorID
	// annotation, not by name.
	Name string

	// RampVendorID is the stable Ramp identifier. Stored on the Vendor as
	// the `compliance.miloapis.com/ramp-vendor-id` annotation and used as
	// the lookup key on subsequent syncs.
	RampVendorID string

	// SpecPatch is the subset of VendorSpec the importer owns. Fields the
	// adapter can't determine (purpose, dataCategories, etc.) are
	// intentionally left unset so the controller can merge without
	// overwriting operator edits to other fields.
	SpecPatch compliancev1alpha1.VendorSpec
}

// MapVendor turns a Ramp accounting vendor into a MappedVendor. Returns
// false when the vendor is not eligible for import (e.g. blank name).
func MapVendor(v AccountingVendor) (MappedVendor, bool) {
	name := strings.TrimSpace(v.Name)
	if name == "" || v.ID == "" {
		return MappedVendor{}, false
	}

	return MappedVendor{
		Name:         BuildResourceName(name, v.ID),
		RampVendorID: v.ID,
		SpecPatch: compliancev1alpha1.VendorSpec{
			DisplayName: name,
			// Ramp doesn't distinguish legal entity from display name.
			// We seed both with the same value; operators can split them
			// before activating the Vendor's compliance profile.
			LegalEntity: name,
			// CountryOfIncorporation is a required field on VendorSpec
			// (the CRD schema rejects empty strings via MinLength), but
			// Ramp's accounting vendors endpoint doesn't currently expose
			// a country. Default to "UN" (a clearly-not-a-real-country
			// ISO 3166 sentinel) so the CRD validator doesn't reject the
			// import; operators must update it during review before they
			// can Activate the profile.
			CountryOfIncorporation: "UN",
		},
	}, true
}

// slugifyPattern matches one or more characters that aren't a lowercase
// alphanumeric and replaces each run with a single hyphen. The result is
// then trimmed of leading/trailing hyphens.
var slugifyPattern = regexp.MustCompile(`[^a-z0-9]+`)

// BuildResourceName produces a stable, RFC 1123-compliant Kubernetes
// resource name from a Ramp vendor name plus a short hash of its ID.
// The hash suffix keeps collisions deterministic when two vendors share a
// name and means re-runs always derive the same K8s name for the same
// Ramp vendor.
func BuildResourceName(displayName, vendorID string) string {
	slug := strings.ToLower(displayName)
	slug = slugifyPattern.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "ramp-vendor"
	}
	// Cap the slug so the suffixed name still fits inside the 63-char
	// DNS label limit (54 + "-" + 8-hex = 63).
	if len(slug) > 54 {
		slug = strings.TrimRight(slug[:54], "-")
	}

	sum := sha1.Sum([]byte(vendorID)) //nolint:gosec
	suffix := hex.EncodeToString(sum[:])[:8]
	return fmt.Sprintf("%s-%s", slug, suffix)
}
