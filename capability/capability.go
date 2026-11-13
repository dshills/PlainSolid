// Package capability exposes PlainSolid's supported workflows and their
// current limits to callers deciding whether a model can be migrated.
package capability

import "strings"

// State distinguishes a tested subset from a capability not yet implemented.
type State string

const (
	SupportedSubset State = "supported_subset"
	NotImplemented  State = "not_implemented"
)

// Feature describes one advertised domain. The limitations are part of
// the contract; these entries do not claim blanket OCCT compatibility.
type Feature struct {
	ID, Domain, Supported, Limits string
	State                         State
}

// Report describes the catalog format, the differential reference and the
// repository's full replacement qualification status.
type Report struct {
	Schema            int
	ReferenceRevision string
	OCCTReplacement   bool
	Features          []Feature
}

// Current returns a caller-owned catalog. ReferenceRevision is the pinned
// version used by the optional native test harness, not a runtime dependency.
func Current() Report {
	return Report{Schema: 1, ReferenceRevision: "4dbfdcc448c0b3e565488afcd626c84483172db9", Features: append([]Feature(nil), catalog[:]...)}
}

// Lookup returns the feature with a stable, case-sensitive identifier.
func Lookup(id string) (Feature, bool) {
	for _, f := range catalog {
		if f.ID == strings.TrimSpace(id) {
			return f, true
		}
	}
	return Feature{}, false
}

var catalog = [...]Feature{
	{"geometry.curves", "Curves", "Analytic conics and rational spline evaluation, editing, intersection with a plane, distance and length bounds", "Continuous clamped splines required by several exact bounds; general curve intersection remains incomplete", SupportedSubset},
	{"geometry.surfaces", "Surfaces", "Analytic and rational spline evaluation, derivatives, exact subpatches, projection, area and distance bounds", "General surface fitting and full periodic editing remain incomplete", SupportedSubset},
	{"geometry.wrapped_toroid", "Wrapped rational toroids", "Exact canonical tensor quotient embedding, outward orientation, paired seam topology and point classification", "Arbitrary wrapped support embedding and material queries remain incomplete", SupportedSubset},
	{"intersection.spline_pair", "Spline intersections", "Certified spline-pair tubes on selected affine coordinate-plane graph charts, one-sided C0 knots, exact trim clipping, and open and closed face partitions", "General exhaustive branch discovery, partial coincidence and tangency classification remain incomplete", SupportedSubset},
	{"modeling.planar_boolean", "Planar Booleans", "Regularized union, intersection, subtraction and face/edge ancestry for embedded planar boundaries", "Complexity and numerical conversion are explicitly bounded", SupportedSubset},
	{"modeling.spline_graph_boolean", "Spline graph Booleans", "Shared-base graph Booleans with regular, knot-aligned coincident, straight, saddle and selected elliptical contacts, and selected exact tube regions", "Requires qualified projected charts and nondegenerate boundary topology; pinched differences are rejected", SupportedSubset},
	{"modeling.spline_shell_boolean", "Spline shell Booleans", "Boolean operations for certified separated, nested and planar-interface joined rectangular spline shells; transverse arrangements with multiple closed contours, edge-crossing wires and selected simultaneous edge crossings", "The transverse paths require complete contact coverage and exact junctions. Singular contacts, general edge-edge arrangements and arbitrary input trims remain unresolved", SupportedSubset},
	{"modeling.finite_termination", "Finite feature termination", "Polygon sweeps to selected finite planar faces, and axial circle or polygon sweeps to selected recognized curved-profile caps", "Exact represented geometry and documented cap families required; not a first-contact solver", SupportedSubset},
	{"modeling.mixed_extrusion", "Mixed analytic extrusion", "Signed polygon, circular and rounded rectangular stock with selected through bores, blind pockets, rectangular through-pockets and cap bosses", "Strict feature separation and prescribed profile families; arbitrary line/arc sketches remain incomplete", SupportedSubset},
	{"modeling.revolution", "Revolutions", "Selected signed partial and full polygon and circular meridians; coordinate-axis model-space placement; exact quarter/half sector queries", "Supported meridians and axis placement are bounded; general revolved-solid Booleans are incomplete", SupportedSubset},
	{"modeling.helical", "Helical modeling", "Finite external and internal polygon grooves, separated starts, disjoint end reliefs and paired flank/runout proofs", "Overlapping lead-ins, arbitrary thread profiles and broader tool/stock Booleans remain incomplete", SupportedSubset},
	{"modeling.cap_chain", "Composed cap features", "The selected horizontal fillet, opposite chamfer, through bore, through-pocket and top-boss chain retains original analytic edges and surfaces", "A bounded authored construction; arbitrary edge treatments and stable history are incomplete", SupportedSubset},
	{"modeling.general_curved_boolean", "General curved Booleans", "", "Arbitrary trimmed curved solids cannot be combined", NotImplemented},
	{"modeling.planar_features", "Planar modeling features", "Selected support drafts, two-distance chamfers, planar pockets, protrusions and convex hollowing", "Each operation has restricted planar support and sketch domains", SupportedSubset},
	{"modeling.wrapped_toroid_offset", "Wrapped toroid offsets", "Exact Euclidean normal offsets on the canonical circular wrapped toroid", "Rejects noncircular profiles and topology transitions", SupportedSubset},
	{"modeling.wrapped_station_loft", "Circular station lofts", "Exact multi-band lofts and nested through-bores with point, line, distance, section and horizontal split queries", "Stations share a Z axis and positive circular radii vary affinely between stations", SupportedSubset},
	{"modeling.wrapped_cylinder_rim", "Circular cylinder rim modeling", "Exact selected toroidal rim blends, complete Euclidean offset in a bounded profile family, and exact material queries", "Requires canonical circular stock; offsets and splits reject unsupported blend transitions", SupportedSubset},
	{"modeling.general_curved_fillet", "General rolling-ball fillets", "", "Only specialized fillet constructions are available", NotImplemented},
	{"queries", "Queries", "Exact planar boundary distances, analytic point/ray queries and certified contact and distance bounds for selected trimmed spline faces", "General trimmed curved solids can return unresolved query results", SupportedSubset},
	{"tessellation", "Tessellation", "Certified selected multi-span rational curved trims, exact shared seam cuts, closed shell incidence and opt-in display embedding", "Arbitrary trims, general periodic seams and smooth source-shell embedding remain open", SupportedSubset},
	{"exchange.step", "STEP", "Faceted and planar bodies, selected analytic and spline B-reps including circular and rounded stock with cavities, torus elbows, closed spline lenses and connected curved oval sheets, twelve certified wrapped material families, named styles and rigid shared product occurrences", "General curved STEP face graphs and full AP203/AP214/AP242 qualification remain open", SupportedSubset},
	{"exchange.iges", "IGES", "Selected analytic and spline entities, certified planar trims, rational oval spline faces with holes, selected open shells and closed spline lenses with exact shared curves", "General IGES trimmed topology and assemblies are incomplete", SupportedSubset},
	{"exchange.native", "Native persistence", "Versioned exact planar and supported curved geometry, including reconstructed rational vertices and a bounded algebraic seam subdivision format", "User-defined evaluators outside the native codec are rejected", SupportedSubset},
	{"exchange.mesh", "Mesh exchange", "OBJ, STL, PLY and embedded triangle glTF/GLB scenes", "Freeform OBJ, broad glTF materials and extensions are unsupported", SupportedSubset},
	{"document", "Documents", "Mixed assets, typed attributes, shared occurrences, transactions and bounded feature regeneration", "General persistent topological naming and feature dependencies are incomplete", SupportedSubset},
	{"presentation", "Presentation", "Headless camera, mesh picking, exact picking for selected wrapped bodies, SVG and mesh-occluded edge spans", "Exact arbitrary curved hidden-line removal and an interactive renderer are unavailable", SupportedSubset},
}
