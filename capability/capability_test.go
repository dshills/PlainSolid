package capability

import "testing"

func TestCurrentReportsQualifiedSubsets(t *testing.T) {
	report := Current()
	if report.OCCTReplacement || report.Schema != 1 || len(report.ReferenceRevision) != 40 {
		t.Fatal("unexpected replacement qualification", report.OCCTReplacement, report.Schema, report.ReferenceRevision)
	}
	seen := make(map[string]bool)
	for _, feature := range report.Features {
		if feature.ID == "" || feature.Domain == "" || feature.Limits == "" || seen[feature.ID] {
			t.Fatal("invalid or duplicate capability", feature)
		}
		seen[feature.ID] = true
		if result, ok := Lookup(feature.ID); !ok || result != feature {
			t.Fatal("capability lookup disagrees with current report", feature.ID)
		}
	}
	for _, id := range []string{"intersection.affine_polygon_arrangement", "modeling.native_prism_boolean", "modeling.convex_euclidean_offsets",
		"modeling.rational_station_loft", "modeling.anisotropic_station_loft", "healing.exact_trimmed_topology",
		"exchange.step_planar_nurbs_assembly", "exchange.step_occurrence_appearance", "exchange.step_weighted_station_assembly"} {
		feature, ok := Lookup(id)
		if !ok || feature.State != SupportedSubset || feature.Supported == "" {
			t.Fatal("missing qualified subset", id)
		}
	}
	report.Features[0] = Feature{}
	if Current().Features[0].ID == "" {
		t.Fatal("caller mutated the shared report")
	}
}
