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
	{"intersection.spline_pair", "Spline intersections", "Certified spline-pair tubes on selected coordinate-plane charts, exact dependent anchors at C0 knots, original rational trim clipping through degree eight, selected double contacts, and coincident coedge imprints", "General exhaustive branch discovery, arbitrary coincidence and singular contact classification remain incomplete", SupportedSubset},
	{"intersection.projected_cubic_graphs", "Projected cubic graph sections", "Exact real crossing nodes, true cusps, isolated double points and separated regular arcs, with signed regions and original owner jets", "Alignable affine projections; every original knot cell must prove the same cubic law, with unambiguous required derivatives and rational admitted exits", SupportedSubset},
	{"intersection.coincident_spline_trims", "Coincident spline trim arrangements", "Exact chart correspondence, bounded one-to-many coedge refinement, hole permutation, and regularized chart regions on original orthogonal trims", "Requires proved chart identity and supported original trim geometry. General curved partial overlaps remain open", SupportedSubset},
	{"intersection.affine_polygon_arrangement", "Exact affine polygon arrangements", "Transverse trim contacts and coplanar Boolean cycles for polygon and hole loops, with retained source intervals and lower-dimensional witnesses", "Branching area boundaries and unsupported exact contact configurations remain unresolved", SupportedSubset},
	{"intersection.wrapped_rectangles", "Periodic face arrangements", "Exact wrapped chart correspondence across compatible knot bases; lifted rectangular overlaps with original UV ownership and optional projected embedding", "Requires a proved affine chart relation; bounded embedding does not establish arbitrary overlapping surfaces or material topology", SupportedSubset},
	{"modeling.planar_boolean", "Planar Booleans", "Regularized union, intersection, subtraction and face/edge ancestry for embedded planar boundaries", "Complexity and numerical conversion are explicitly bounded", SupportedSubset},
	{"modeling.native_prism_boolean", "Native prism Booleans", "Authenticated straight prisms with matching, oblique and selected unequal axial intervals, exact cap imprints, crossing cuts and unions, cavities, side openings and original side history", "The exact proof requires supported extrusion and profile relationships and lossless output vertices", SupportedSubset},
	{"modeling.spline_graph_boolean", "Spline graph Booleans", "Shared-base graph Booleans with regular, knot-aligned coincident, straight, saddle and selected elliptical contacts, and selected exact tube regions", "Requires qualified projected charts and nondegenerate boundary topology; pinched differences are rejected", SupportedSubset},
	{"modeling.cubic_graph_boolean", "Cubic graph Booleans", "Source-backed Common, Fuse and admitted Cut for rectangular stock with nodal, cuspidal or isolated cubic contacts, including native replay and selected closed previews", "Only the documented section and footprint families; pinched material and unsupported source knots or displays may be refused", SupportedSubset},
	{"modeling.spline_shell_boolean", "Spline shell Booleans", "Boolean operations for separated, nested, equal and planar-interface joined spline shells, selected closed tangencies, convex affine edge crossings and independently certified material components", "The paths require complete original pair coverage and exact junctions. General curved edge arrangements and arbitrary trims remain unresolved", SupportedSubset},
	{"modeling.spline_plane_clipping", "Spline plane clipping", "Native one-plane common and cut with certified corner, polynomial and rational section branches through degree four, including selected oblique planes and free-axis crease spans; selected two-plane intersections retain shared-cap ancestry", "Crossed faces must satisfy bounded exact chart contracts; general tangency, coincidence and plane arrangements remain incomplete", SupportedSubset},
	{"modeling.sphere_cylinder", "Sphere and cylinder Booleans", "Native finite coaxial and selected off-axis intersections, subtractive bores and cavities, and protruding unions; exact material and section queries; finite one- and two-lens intersections", "General cap arrangements, pinches, arbitrary axes and cross-family trimming are not fully assembled", SupportedSubset},
	{"queries.sphere_pair_material", "Two-sphere analytic queries", "Exact regularized point, finite-line and first boundary hit queries with exposed sphere ownership and outward normals", "Touching sets can be singular; the analytic query does not assert manifold native topology", SupportedSubset},
	{"modeling.cylinder_pair", "Cylinder pair Booleans", "Native selected perpendicular, oblique and parallel sets, including transverse blind and interior cuts, common, staggered, nested and abutting axial unions, and exact regularized point, finite-line and first boundary hit queries", "General skew placement, singular radial tangency and other cap arrangements remain incomplete", SupportedSubset},
	{"modeling.finite_termination", "Finite feature termination", "Polygon sweeps to selected finite planar faces, and axial circle or polygon sweeps to selected recognized curved-profile caps", "Exact represented geometry and documented cap families required; not a first-contact solver", SupportedSubset},
	{"modeling.mixed_extrusion", "Mixed analytic extrusion", "Signed polygon, circular and rounded rectangular stock with selected through bores, blind pockets, rectangular through-pockets and cap bosses", "Strict feature separation and prescribed profile families; arbitrary line/arc sketches remain incomplete", SupportedSubset},
	{"modeling.side_open_dual_disk", "Paired side-opening cylinders", "Blind and through cuts on signed rectangular stock, with original wall ownership, exact line queries, certified display and named STEP codecs", "Requires common axial depth, qualified exact side intersections, representable model vertices and the supported two-circle arc arrangement", SupportedSubset},
	{"modeling.revolution", "Revolutions", "Selected signed partial and full polygon and circular meridians; coordinate-axis model-space placement; exact quarter/half sector queries", "Supported meridians and axis placement are bounded; general revolved-solid Booleans are incomplete", SupportedSubset},
	{"modeling.helical", "Helical modeling", "Finite external and internal polygon grooves, separated starts, disjoint end reliefs and paired flank/runout proofs", "Overlapping lead-ins, arbitrary thread profiles and broader tool/stock Booleans remain incomplete", SupportedSubset},
	{"modeling.cap_chain", "Composed cap features", "The selected horizontal fillet, opposite chamfer, through bore, through-pocket and top-boss chain retains original analytic edges and surfaces", "A bounded authored construction; arbitrary edge treatments and stable history are incomplete", SupportedSubset},
	{"modeling.general_curved_boolean", "General curved Booleans", "", "Arbitrary trimmed curved solids cannot be combined", NotImplemented},
	{"modeling.planar_features", "Planar modeling features", "Selected support and independently hinged drafts, constant and tapered edge chamfers, planar pockets, protrusions and exact multi-port convex hollowing", "Each operation has restricted planar support and sketch domains", SupportedSubset},
	{"modeling.planar_loft", "Planar polygon lofts", "Exact two-ring and multi-station lofts, convex bands, separated miter cells and source intervals for unequal collinear subdivisions", "Requires supported corresponding planar rings and exact side planarity; no general curved, self-intersecting or inferred correspondence loft", SupportedSubset},
	{"modeling.convex_euclidean_offsets", "Convex Euclidean offsets", "Convex planar-solid erosion and independently shifted mitered supports, plus rounded convex prism footprint dilation, exact line queries and certified shared-rim meshes", "Planar support shifts require a supported radical field. Footprint dilation requires exact local tangent endpoints", SupportedSubset},
	{"modeling.wrapped_toroid_offset", "Wrapped toroid offsets", "Exact Euclidean normal offsets on the canonical circular wrapped toroid", "Rejects noncircular profiles and topology transitions", SupportedSubset},
	{"modeling.wrapped_station_loft", "Circular station lofts", "Exact multi-band lofts and nested through-bores with point, line, distance, section and horizontal split queries", "Stations share a Z axis and positive circular radii vary affinely between stations", SupportedSubset},
	{"modeling.rational_station_loft", "Weighted rational station lofts", "Exact weighted homothetic filled lofts and through-bores with sections, classification, certified distance, area, volume and inertia", "Profiles have the supported four-quadrant basis and piecewise-affine station scales", SupportedSubset},
	{"modeling.anisotropic_station_loft", "Anisotropic weighted station lofts", "Native weighted bands with independent X and Y scales and through-bores, exact rigid occurrences, source-bound sections, point and line queries, certified mesh and mass bounds", "Requires positive affine scales and exact shared weighted rims. Arbitrary profiles and path sweeps remain open", SupportedSubset},
	{"modeling.wrapped_cylinder_rim", "Circular cylinder rim modeling", "Exact selected toroidal rim blends, complete Euclidean offset in a bounded profile family, and exact material queries", "Requires canonical circular stock; offsets and splits reject unsupported blend transitions", SupportedSubset},
	{"modeling.general_curved_fillet", "General rolling-ball fillets", "", "Only specialized fillet constructions are available", NotImplemented},
	{"healing.exact_trimmed_topology", "Exact trimmed topology sewing", "Continuous rational edge matching, selected nonconforming straight and curved seam splits, original closed vertex fans, and bounded planar tolerance repair", "General shell embedding, cavity nesting, ambiguous gap repair and curved toleranced healing remain open", SupportedSubset},
	{"queries", "Queries", "Exact planar distances, analytic point and ray queries, and certified trimmed spline distance bounds with exact original coedge witnesses across algebraic knot events", "General trimmed curved solids can return unresolved query results", SupportedSubset},
	{"tessellation", "Tessellation", "Certified selected multi-span rational curved trims, exact shared seam cuts, closed shell incidence and opt-in display embedding", "Arbitrary trims, general periodic seams and smooth source-shell embedding remain open", SupportedSubset},
	{"tessellation.wrapped_circular_normals", "Circular wrapped display normals", "Exact continuous normal deviation bounds for canonical circular wrapped cylinders, tapered frustums and both walls of supported bores", "Lossless signed-axis placement only; arbitrary transformed or noncircular normal proofs remain incomplete", SupportedSubset},
	{"tessellation.mapped_spline_normals", "Mapped spline display normals", "Closed source-cell tessellation with exact shared knot stations, distance and one-sided facet-to-source normal bounds", "Selected mapped spline graphs; facet angle and within-facet normal turning are distinct controls", SupportedSubset},
	{"exchange.step", "STEP", "Faceted and planar bodies, selected analytic and spline B-reps including circular and rounded stock with cavities, torus elbows, closed spline lenses and connected curved oval sheets, twelve certified wrapped material families, named styles and rigid shared product occurrences", "General curved STEP face graphs and full AP203/AP214/AP242 qualification remain open", SupportedSubset},
	{"exchange.step_planar_nurbs_assembly", "Planar NURBS STEP assemblies", "Independently authored unit, scaled and supported oblique affine NURBS product assemblies with STEP and checked native persistence", "Requires the supported finite affine charts and source ownership graph", SupportedSubset},
	{"exchange.step_occurrence_appearance", "STEP occurrence appearance", "Original ordinary and SHUO contextual whole-body colors on qualified shared paths through three assembly levels", "Arbitrary assembly graphs and face-local occurrence styles remain open", SupportedSubset},
	{"exchange.step_weighted_station_assembly", "Weighted station STEP assemblies", "Original homothetic and anisotropic filled or through-bore station products, signed and exact rational poses, and checked native reload for flat and selected nested shared assemblies", "Requires a supported weighted station profile and occurrence graph. General assembly nesting, placement and local styles remain open", SupportedSubset},
	{"exchange.step_composite_curves", "STEP composite curves", "Ordered full-domain spline segments with explicit direction, exact positional joins, and verified open or closed geometry", "Nested or shared leaves, unsupported continuation claims, and implied face or solid topology are rejected", SupportedSubset},
	{"exchange.step_swept_surfaces", "STEP swept surfaces", "Standalone bounded linear-extrusion and revolution surfaces with exact spline profile data and explicit parameter ranges", "Endpoint translations or axis and meridian proofs must succeed; general swept topology is outside these codecs", SupportedSubset},
	{"exchange.cylindrical_supports", "Standalone cylindrical supports", "Exact signed-coordinate cylindrical support exchange and strict conversion between STEP and IGES 192 form 1", "Unbounded supports with explicit placement; nonportable metadata, trims, faces, and implied material are rejected", SupportedSubset},
	{"exchange.planar_supports", "Standalone planar supports", "Exact signed-coordinate plane support exchange and strict conversion between STEP and IGES 190 form 1", "Unbounded supports with explicit placement; no trim or topology inference", SupportedSubset},
	{"exchange.spherical_supports", "Standalone spherical supports", "Exact signed-coordinate sphere support exchange and strict conversion between STEP and IGES 196 form 1", "Natural parameter period and poles; no trimmed face, seam topology, or material inference", SupportedSubset},
	{"exchange.step_polynomial_evidence", "STEP polynomial trim evidence", "Exact rational residual and winding bounds, full-domain rim correspondence, admissible cylinder seam intervals, and conditional chart coverage after explicit spherical longitude correction", "Bounded source evidence only; full authored trim selection, repaired cross-face closure and import material equivalence are not certified", SupportedSubset},
	{"exchange.iges", "IGES", "Datum points, ordered 3D polylines, exact direct composite curves, bounded revolution and tabulated surfaces, selected spline trims, shells and closed lenses", "General IGES trimmed topology and assemblies are incomplete", SupportedSubset},
	{"exchange.eccentric_bore_rim_fillets", "Eccentric bore rim fillet exchange", "Analytic STEP and IGES output and bounded independent model-only input with original units and rim source provenance", "Strict independent STEP covers certified owner incidence and support coordinates; rounded off-support curves and conflicting uses are refused", SupportedSubset},
	{"exchange.cubic_graph_envelopes", "Cubic graph envelope exchange", "STEP and IGES transfer of admitted nodal, cuspidal and isolated-contact bodies, with exact original support and certified rational knot splits where supported", "Source-backed writers and canonical verification with external readback; general independent reconstruction remains incomplete", SupportedSubset},
	{"exchange.native", "Native persistence", "Versioned exact planar and curved geometry, multispan homogeneous tensor supports, algebraic seam subdivision, and source-bound exchange assemblies", "User-defined evaluators outside the native codecs are rejected", SupportedSubset},
	{"exchange.mesh", "Mesh exchange", "OBJ, STL, PLY and embedded triangle glTF/GLB scenes", "Freeform OBJ, broad glTF materials and extensions are unsupported", SupportedSubset},
	{"document", "Documents", "Mixed assets, typed attributes, shared occurrences, transactions and bounded feature regeneration", "General persistent topological naming and feature dependencies are incomplete", SupportedSubset},
	{"presentation", "Presentation", "Headless camera, mesh picking, exact picking for selected wrapped bodies, SVG and mesh-occluded edge spans", "Exact arbitrary curved hidden-line removal and an interactive renderer are unavailable", SupportedSubset},
}
