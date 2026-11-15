package step

import (
	"io"
	"sort"
	"strings"
)

// EntityTypeUse records a declared simple/complex entity component and its IDs.
// Feature is a catalog category, not an EXPRESS-schema or geometry certificate.
// An empty Feature means this catalog does not classify the component.
type EntityTypeUse struct {
	Type      string
	Feature   string
	EntityIDs []int64
}

// GraphPreflight is a syntax-level inventory for migration routing. All arrays
// are sorted deterministically. A recognized type does not establish supported
// argument forms, graph ownership, units, trim coverage or valid geometry.
// Strict readers must still validate the entire graph before constructing assets.
type GraphPreflight struct {
	Schemas                         []string
	EntityCount, ComplexEntityCount int
	Types                           []EntityTypeUse
	Features                        []string
	Unclassified                    []EntityTypeUse
	CandidateReaders                []string
}

var preflightCatalog = func() map[string]string {
	groups := map[string]string{
		"placement":         "AXIS1_PLACEMENT AXIS2_PLACEMENT_2D AXIS2_PLACEMENT_3D CARTESIAN_POINT DIRECTION VECTOR",
		"analytic-curves":   "LINE CIRCLE ELLIPSE PARABOLA HYPERBOLA TRIMMED_CURVE",
		"composite-curves":  "COMPOSITE_CURVE COMPOSITE_CURVE_SEGMENT",
		"spline-curves":     "B_SPLINE_CURVE B_SPLINE_CURVE_WITH_KNOTS RATIONAL_B_SPLINE_CURVE",
		"analytic-surfaces": "PLANE CYLINDRICAL_SURFACE CONICAL_SURFACE SPHERICAL_SURFACE TOROIDAL_SURFACE",
		"swept-surfaces":    "SURFACE_OF_LINEAR_EXTRUSION SURFACE_OF_REVOLUTION",
		"surface-trims":     "RECTANGULAR_TRIMMED_SURFACE",
		"spline-surfaces":   "B_SPLINE_SURFACE B_SPLINE_SURFACE_WITH_KNOTS RATIONAL_B_SPLINE_SURFACE",
		"pcurves":           "PCURVE SURFACE_CURVE SEAM_CURVE DEFINITIONAL_REPRESENTATION",
		"face-topology":     "FACE ADVANCED_FACE FACE_SURFACE FACE_BOUND FACE_OUTER_BOUND EDGE_CURVE EDGE_LOOP ORIENTED_EDGE VERTEX_POINT VERTEX_LOOP POLY_LOOP",
		"solid-topology":    "MANIFOLD_SOLID_BREP FACETED_BREP CLOSED_SHELL ADVANCED_BREP_SHAPE_REPRESENTATION FACETED_BREP_SHAPE_REPRESENTATION",
		"cavities":          "BREP_WITH_VOIDS ORIENTED_CLOSED_SHELL",
		"surface-models":    "OPEN_SHELL SHELL_BASED_SURFACE_MODEL MANIFOLD_SURFACE_SHAPE_REPRESENTATION GEOMETRICALLY_BOUNDED_SURFACE_SHAPE_REPRESENTATION GEOMETRIC_SET",
		"curve-models":      "GEOMETRIC_CURVE_SET GEOMETRICALLY_BOUNDED_WIREFRAME_SHAPE_REPRESENTATION",
		"mapped-material":   "MAPPED_ITEM REPRESENTATION_MAP CARTESIAN_TRANSFORMATION_OPERATOR_3D",
		"assembly":          "NEXT_ASSEMBLY_USAGE_OCCURRENCE SPECIFIED_HIGHER_USAGE_OCCURRENCE CONTEXT_DEPENDENT_SHAPE_REPRESENTATION REPRESENTATION_RELATIONSHIP REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION SHAPE_REPRESENTATION_RELATIONSHIP ITEM_DEFINED_TRANSFORMATION",
		"presentation":      "COLOUR_RGB DRAUGHTING_PRE_DEFINED_COLOUR STYLED_ITEM OVER_RIDING_STYLED_ITEM PRESENTATION_STYLE_ASSIGNMENT PRESENTATION_STYLE_BY_CONTEXT SURFACE_STYLE_USAGE SURFACE_SIDE_STYLE SURFACE_STYLE_FILL_AREA FILL_AREA_STYLE FILL_AREA_STYLE_COLOUR MECHANICAL_DESIGN_GEOMETRIC_PRESENTATION_REPRESENTATION",
		"units":             "SI_UNIT NAMED_UNIT LENGTH_UNIT PLANE_ANGLE_UNIT SOLID_ANGLE_UNIT CONVERSION_BASED_UNIT LENGTH_MEASURE_WITH_UNIT MEASURE_WITH_UNIT DIMENSIONAL_EXPONENTS GLOBAL_UNIT_ASSIGNED_CONTEXT GLOBAL_UNCERTAINTY_ASSIGNED_CONTEXT UNCERTAINTY_MEASURE_WITH_UNIT",
		"metadata":          "REPRESENTATION DESIGN_CONTEXT MECHANICAL_CONTEXT APPLICATION_CONTEXT APPLICATION_PROTOCOL_DEFINITION PRODUCT PRODUCT_CONTEXT PRODUCT_DEFINITION PRODUCT_DEFINITION_CONTEXT PRODUCT_DEFINITION_FORMATION PRODUCT_DEFINITION_FORMATION_WITH_SPECIFIED_SOURCE PRODUCT_DEFINITION_SHAPE PRODUCT_RELATED_PRODUCT_CATEGORY SHAPE_DEFINITION_REPRESENTATION SHAPE_REPRESENTATION REPRESENTATION_CONTEXT GEOMETRIC_REPRESENTATION_CONTEXT PARAMETRIC_REPRESENTATION_CONTEXT REPRESENTATION_ITEM GEOMETRIC_REPRESENTATION_ITEM CURVE BOUNDED_CURVE SURFACE BOUNDED_SURFACE",
	}
	catalog := map[string]string{}
	for group, names := range groups {
		for _, name := range strings.Fields(names) {
			catalog[name] = group
		}
	}
	return catalog
}()

// Preflight parses bounded Part21 input and inventories declared entity types
// without decoding or approximating any geometry. Unknown types are reported,
// not silently dropped. Malformed syntax/references and resource limits fail.
// CandidateReaders are hints only; even a graph with no Unclassified records
// can have unsupported trims, placements, combinations or invalid geometry.
func Preflight(r io.Reader, l Limits) (GraphPreflight, error) {
	f, err := Parse(r, l)
	if err != nil {
		return GraphPreflight{}, err
	}
	report := GraphPreflight{EntityCount: len(f.Entities)}
	schemas := map[string]bool{}
	for _, h := range f.Header {
		if h.Type == "FILE_SCHEMA" {
			for _, v := range h.Args {
				if v.Kind == Aggregate {
					for _, x := range v.Items {
						if x.Kind == String {
							schemas[x.Text] = true
						}
					}
				}
			}
		}
	}
	for schema := range schemas {
		report.Schemas = append(report.Schemas, schema)
	}
	sort.Strings(report.Schemas)
	types := map[string]map[int64]bool{}
	for _, id := range f.Order {
		entity := f.Entities[id]
		if len(entity.Records) > 1 {
			report.ComplexEntityCount++
		}
		for _, record := range entity.Records {
			if types[record.Type] == nil {
				types[record.Type] = map[int64]bool{}
			}
			types[record.Type][id] = true
		}
	}
	names := make([]string, 0, len(types))
	for name := range types {
		names = append(names, name)
	}
	sort.Strings(names)
	features := map[string]bool{}
	for _, name := range names {
		use := EntityTypeUse{Type: name, Feature: preflightCatalog[name]}
		for id := range types[name] {
			use.EntityIDs = append(use.EntityIDs, id)
		}
		sort.Slice(use.EntityIDs, func(i, j int) bool { return use.EntityIDs[i] < use.EntityIDs[j] })
		report.Types = append(report.Types, use)
		if use.Feature == "" {
			copyUse := use
			copyUse.EntityIDs = append([]int64(nil), use.EntityIDs...)
			report.Unclassified = append(report.Unclassified, copyUse)
		} else {
			features[use.Feature] = true
		}
	}
	for feature := range features {
		report.Features = append(report.Features, feature)
	}
	sort.Strings(report.Features)
	hints := map[string]bool{}
	if len(types["REPRESENTATION"]) > 0 && len(types["GEOMETRIC_SET"]) > 0 && features["analytic-surfaces"] && !features["face-topology"] && !features["solid-topology"] && !features["assembly"] {
		hints["ReadAnalyticSupports"] = true
	}
	if len(types["REPRESENTATION"]) > 0 && len(types["GEOMETRIC_SET"]) > 0 && len(types["TOROIDAL_SURFACE"]) > 0 && !features["face-topology"] && !features["solid-topology"] && !features["assembly"] {
		hints["ReadTorusSupports"] = true
	}
	if len(types["REPRESENTATION"]) > 0 && len(types["GEOMETRIC_SET"]) > 0 && len(types["SPHERICAL_SURFACE"]) > 0 && !features["face-topology"] && !features["solid-topology"] && !features["assembly"] {
		hints["ReadSphereSupports"] = true
	}
	if len(types["REPRESENTATION"]) > 0 && len(types["GEOMETRIC_SET"]) > 0 && len(types["PLANE"]) > 0 && !features["face-topology"] && !features["solid-topology"] && !features["assembly"] {
		hints["ReadPlaneSupports"] = true
	}
	if len(types["REPRESENTATION"]) > 0 && len(types["GEOMETRIC_SET"]) > 0 && len(types["CYLINDRICAL_SURFACE"]) > 0 && !features["face-topology"] && !features["solid-topology"] && !features["assembly"] {
		hints["ReadCylinderSupports"] = true
	}
	// These are declaration-level routes, not acceptance or ownership proofs.
	// OCCT fixture files may include a bounded support alongside unrelated face
	// topology: those receive only entity-decoder hints for the swept family.
	linear := len(types["SURFACE_OF_LINEAR_EXTRUSION"]) > 0
	revolution := len(types["SURFACE_OF_REVOLUTION"]) > 0
	trimmed := len(types["RECTANGULAR_TRIMMED_SURFACE"]) > 0
	if trimmed && linear {
		hints["DecodeLinearExtrusionSurface"] = true
		hints["DecodeRationalLinearExtrusionSurface"] = true
	}
	if trimmed && revolution {
		hints["DecodeRevolutionSurface"] = true
	}
	standalone := trimmed && len(types["GEOMETRIC_SET"]) > 0 && len(types["GEOMETRICALLY_BOUNDED_SURFACE_SHAPE_REPRESENTATION"]) > 0 && !features["face-topology"] && !features["solid-topology"] && !features["cavities"] && !features["assembly"]
	if standalone && linear && !revolution {
		hints["ReadLinearExtrusions"] = true
		hints["ReadRationalLinearExtrusions"] = true
	}
	if standalone && revolution && !linear {
		hints["ReadRevolutionSurfaces"] = true
	}

	if features["solid-topology"] {
		hints["Read"] = true
		hints["ReadAnalyticBodies"] = true
		hints["ReadTrimmedBodies"] = true
		if features["spline-surfaces"] {
			hints["ReadSplineShells"] = true
		}
	}
	if len(types["SHAPE_DEFINITION_REPRESENTATION"]) > 0 && len(types["ADVANCED_BREP_SHAPE_REPRESENTATION"]) > 0 &&
		len(types["SPHERICAL_SURFACE"]) > 0 && len(types["VERTEX_LOOP"]) > 0 && !features["assembly"] {
		// The pole-only boundary is a declaration-level hint. The
		// reader must still prove the exact pole and product ownership.
		hints["ReadIndependentSphereMaterials"] = true
	}
	if features["cavities"] {
		hints["ReadCavities"] = true
	}
	if features["surface-models"] && features["spline-surfaces"] {
		hints["ReadSurfaces"] = true
		hints["ReadBoundedSplineFaces"] = true
	}
	if features["composite-curves"] {
		hints["ReadCompositeCurves"] = true
		hints["ReadTangentCompositeCurves"] = true
	}
	if features["curve-models"] {
		hints["ReadCurves"] = true
		hints["ReadConicArcs"] = true
	}
	if features["mapped-material"] {
		hints["ReadMappedPlanarMaterial"] = true
		hints["ReadMappedPlanarMaterials"] = true
	}
	if features["assembly"] {
		hints["ReadAssembly"] = true
		if features["presentation"] {
			hints["ReadColoredAssembly"] = true
		}
	}
	if features["presentation"] && features["solid-topology"] {
		hints["ReadFaceColoredPlanar"] = true
	}
	for name := range hints {
		report.CandidateReaders = append(report.CandidateReaders, name)
	}
	sort.Strings(report.CandidateReaders)
	return report, nil
}
