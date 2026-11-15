package trimmed

import (
	"context"
	"errors"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/csg/poly"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// SplineShellCombineOptions groups the certified Boolean paths. OpenSections
// supplies optional edge-crossing proposals. Without them, disjoint, nested,
// planar-interface and selected transverse cases are tried using face bounds.
type SplineShellCombineOptions struct {
	// AffineCap opts into exhaustive positive-area inclined-cap discovery.
	// It does not change default dispatch or accept ambiguous interfaces.
	AffineCap *AffineShellCapNativeOptions
	// AffinePaired explicitly selects complete exact convex affine authorization
	// and original-fragment material assembly. Nil preserves legacy dispatch.
	AffinePaired *ConvexAffineSplineShellOptions
	Transverse   SplineShellTransverseOptions
	Join         SplineShellJoinOptions
	Open         SplineShellOpenOptions
	OpenSections []SplineShellOpenSection
	// Sections supplies proposals for closed or mixed arrangements. It
	// is mutually exclusive with OpenSections.
	Sections []SplineShellSection
	// Planar controls the exact fallback for entirely affine spline shells.
	Planar poly.Options
	// DisableAffineDiscovery skips automatic affine tube proposals and the
	// exact planar fallback.
	DisableAffineDiscovery bool
	// DisablePlaneDiscovery skips automatic open plane/spline proposals.
	DisablePlaneDiscovery bool
	// EnableGeneralTubeDiscovery tries bounded automatic pair tracing when
	// the specialized plane path cannot propose the full wire.
	EnableGeneralTubeDiscovery bool
	// Algebraic optionally selects exact original section discovery, imprint,
	// and reconstruction when all resulting vertices/ranges are representable.
	// A nonrepresentable result can instead be consumed through SectionSplineShellsAlgebraic.
	Algebraic *SplineShellAlgebraicNativeOptions
	// OrderedRoofCommon optionally tries the original-chart roof-box Common
	// before general section discovery. A proof outside its supported domain
	// continues through the other certified paths.
	OrderedRoofCommon *SplineShellRoofCommonOptions
	// OrderedRoofFuse optionally tries the ordered roof-box Fuse
	// before general section discovery. Unsupported geometry retains fallback.
	OrderedRoofFuse *SplineShellRoofOptions
	// OrderedRoofCut optionally tries the exact band or cavity Cut before
	// general section discovery. Unsupported geometry retains fallback.
	OrderedRoofCut *SplineShellRoofOptions
	// ClosedEllipseTangency tries exact closed double-ellipse discovery when
	// just one original face pair remains after separation. Its work limit
	// is shared across projections. Exterior Fuse can return both contacting
	// material shells. Unsupported contacts continue dispatch.
	ClosedEllipseTangency *geom.GraphPointContactOptions
	// VerticalQuadraticWarp optionally proposes a common vertical warp
	// z <- z + k*(x-y)^2 of affine spline charts. Exact unwarping and
	// independent original-support tube proofs are required before use.
	VerticalQuadraticWarp float64
	// DiscoverVerticalQuadraticWarp derives and tests a common warp
	// coefficient when no explicit VerticalQuadraticWarp is supplied.
	DiscoverVerticalQuadraticWarp bool
}

// SplineShellRoofCommonOptions configures the ordered roof-box Common dispatch.
type SplineShellRoofCommonOptions struct {
	Source       MixedRoofQueryOptions
	Construction geom.SurfacePatchEvaluationOptions
}

// SplineShellRoofOptions bounds the source and construction proofs for an
// optional ordered roof-box Boolean dispatch.
type SplineShellRoofOptions = SplineShellRoofCommonOptions

// SplineShellAlgebraicNativeOptions configures the opt-in algebraic path to
// the ordinary native Solid. A zero tolerance uses the larger operand tolerance.
type SplineShellAlgebraicNativeOptions struct {
	Section   AlgebraicShellSectionOptions
	Boolean   AlgebraicShellBooleanOptions
	Native    NativeAlgebraicBooleanOptions
	Tolerance float64
}

// SplineShellCombineHistory provides retained faces for either operand and
// records which certified Boolean path established the result. EdgesA and
// EdgesB are populated by the section and planar arrangement paths.
type SplineShellCombineHistory struct {
	AffineCap      *AffineShellCapDiscovery
	AffineCapUnion *AffineShellCapUnion
	// AffinePaired retains exact fragment occupancy and full source ancestry for
	// the explicit paired-affine path; other dispatch paths leave it nil.
	AffinePaired   *AffineShellMaterial
	ClosedEllipse  *SplineShellRepeatedEllipse
	FacesA, FacesB [][]int
	EdgesA, EdgesB [][]int
	SectionEdges   []int
	// GeneratedEdges records edges with a new portion in the planar fallback,
	// including internal subdivisions of a retained planar face.
	GeneratedEdges []int
	// PlanarEdges is exhaustive edge provenance for the planar fallback.
	// It records exact input/output overlap intervals and owning faces.
	PlanarEdges []poly.EdgeAncestry
	// ReversedFaces indexes retained output faces whose orientation is
	// reversed from their single original face. The ordered roof Cut fills
	// this ledger; other paths may leave it nil.
	ReversedFaces []int
	Method        string
}

// CombineSplineShells selects a certified Boolean path for rectangular
// spline shells. It never substitutes a faceted result. Arbitrary input
// trims and contacts outside the certified paths return an error.
func CombineSplineShells(ctx context.Context, a, b Solid, op parametric.BooleanOp, o SplineShellCombineOptions) (Solid, SplineShellCombineHistory, error) {
	var r SplineShellCombineHistory
	if ctx == nil || op > parametric.Difference || !finite(o.VerticalQuadraticWarp) {
		return Solid{}, r, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, r, e
	}
	if o.AffineCap != nil {
		if o.AffinePaired != nil || o.Algebraic != nil || len(o.Sections) != 0 || len(o.OpenSections) != 0 || o.DisableAffineDiscovery || o.VerticalQuadraticWarp != 0 || o.DiscoverVerticalQuadraticWarp {
			return Solid{}, r, geom.ErrInvalid
		}
		return combineSplineShellsAffineCap(ctx, a, b, op, *o.AffineCap)
	}
	if o.AffinePaired != nil {
		if o.Algebraic != nil || len(o.Sections) != 0 || len(o.OpenSections) != 0 || o.DisableAffineDiscovery || o.VerticalQuadraticWarp != 0 || o.DiscoverVerticalQuadraticWarp {
			return Solid{}, r, geom.ErrInvalid
		}
		return combineSplineShellsAffinePaired(ctx, a, b, op, *o.AffinePaired)
	}
	if o.Algebraic != nil {
		if len(o.Sections) != 0 || len(o.OpenSections) != 0 {
			return Solid{}, r, geom.ErrInvalid
		}
		return combineSplineShellsAlgebraicNative(ctx, a, b, op, *o.Algebraic)
	}
	if len(o.Sections) != 0 {
		if len(o.OpenSections) != 0 {
			return Solid{}, r, geom.ErrInvalid
		}
		s, h, e := CombineSectionedSplineShells(ctx, a, b, op, o.Sections, o.Open)
		if e == nil {
			r = splineSectionCombineHistory(h, "section-arrangement")
		}
		return s, r, e
	}
	if len(o.OpenSections) != 0 {
		s, h, e := CombineOpenSplineShells(ctx, a, b, op, o.OpenSections, o.Open)
		if e == nil {
			r = splineSectionCombineHistory(h, "open-wire")
		}
		return s, r, e
	}
	if o.OrderedRoofCommon != nil && op == parametric.Intersection {
		proof := o.OrderedRoofCommon
		s, h, e := IntersectOrderedRoofBoxes(ctx, a, b, proof.Source, proof.Construction)
		if e == nil {
			r = SplineShellCombineHistory{Method: "ordered-roof-common", FacesA: make([][]int, len(a.faces)), FacesB: make([][]int, len(b.faces))}
			for resultFace, source := range h.Faces {
				row := [2][][]int{r.FacesA, r.FacesB}[source.Operand]
				row[source.Face] = append(row[source.Face], resultFace)
			}
			return s, r, nil
		}
		if !errors.Is(e, geom.ErrUnrepresentable) {
			return Solid{}, r, e
		}
	}
	if o.OrderedRoofFuse != nil && op == parametric.Union {
		proof := o.OrderedRoofFuse
		s, h, e := FuseOrderedRoofBoxes(ctx, a, b, proof.Source, proof.Construction)
		if e == nil {
			r = SplineShellCombineHistory{Method: "ordered-roof-fuse", FacesA: make([][]int, len(a.faces)), FacesB: make([][]int, len(b.faces))}
			for resultFace, sources := range h.Faces {
				for _, source := range sources {
					row := [2][][]int{r.FacesA, r.FacesB}[source.Operand]
					row[source.Face] = append(row[source.Face], resultFace)
				}
			}
			return s, r, nil
		}
		if !errors.Is(e, geom.ErrUnrepresentable) {
			return Solid{}, r, e
		}
	}
	if o.OrderedRoofCut != nil && op == parametric.Difference {
		proof := o.OrderedRoofCut
		s, h, e := SubtractOrderedRoofBoxes(ctx, a, b, proof.Source, proof.Construction)
		if e == nil {
			r = SplineShellCombineHistory{Method: "ordered-roof-cut", FacesA: make([][]int, len(a.faces)), FacesB: make([][]int, len(b.faces))}
			for resultFace, source := range h.Faces {
				row := [2][][]int{r.FacesA, r.FacesB}[source.Operand]
				row[source.Face] = append(row[source.Face], resultFace)
				if h.Reversed[resultFace] {
					r.ReversedFaces = append(r.ReversedFaces, resultFace)
				}
			}
			return s, r, nil
		}
		if !errors.Is(e, geom.ErrUnrepresentable) {
			return Solid{}, r, e
		}
	}
	if hasCanonicalWrappedCylinderShape(a) && hasCanonicalWrappedCylinderShape(b) {
		s, h, e := CombineAxialWrappedCylinders(ctx, a, b, op, o.Transverse.Shells.Embedding.Patches)
		if e == nil {
			return s, h, nil
		}
		if !errors.Is(e, geom.ErrConvergence) {
			return Solid{}, r, e
		}
	}
	if hasWrappedStationLoftShape(a) && hasWrappedStationLoftShape(b) {
		s, h, e := CombineCoaxialWrappedStationLofts(ctx, a, b, op, o.Transverse.Shells.Embedding.Patches)
		if e == nil {
			out := SplineShellCombineHistory{FacesA: make([][]int, len(a.faces)), FacesB: make([][]int, len(b.faces)), Method: "coaxial-station-loft"}
			for _, bands := range [][]WrappedStationBooleanBand{h.Bands, h.InnerBands} {
				for _, band := range bands {
					for k, rows := range [][][]int{out.FacesA, out.FacesB} {
						if source := band.SourceWalls[k]; source >= 0 {
							rows[source] = append(rows[source], band.ResultWall)
						}
					}
				}
			}
			if len(s.faces) != 0 {
				for i, cap := range h.ResultCaps {
					for k, rows := range [][][]int{out.FacesA, out.FacesB} {
						if h.CapSources[i][k] {
							rows[h.SourceCaps[k][i]] = append(rows[h.SourceCaps[k][i]], cap)
						}
					}
				}
			}
			return s, out, nil
		}
		if !errors.Is(e, geom.ErrConvergence) {
			return Solid{}, r, e
		}
	}
	affineOptions := splineDispatchAffineOptions(o)
	var ap, bp []splineEmbeddingFace
	for _, x := range []struct {
		s Solid
		p *[]splineEmbeddingFace
	}{{a, &ap}, {b, &bp}} {
		if len(x.s.faces) == 0 {
			continue
		}
		if _, e := x.s.validateSplineRectangleEmbedding(ctx, o.Transverse.Shells.Embedding, x.p); e != nil {
			if errors.Is(e, geom.ErrConvergence) && !o.DisableAffineDiscovery {
				s, h, affineErr := CombineAffineSplineShells(ctx, a, b, op, affineOptions)
				if affineErr == nil {
					return s, h, nil
				}
				if !isUnsupportedAffineDispatch(affineErr) {
					return Solid{}, r, affineErr
				}
				e = errors.Join(e, affineErr)
			}
			if errors.Is(e, geom.ErrConvergence) && (len(a.Topology().Shells) > 1 || len(b.Topology().Shells) > 1 ||
				len(a.faces) == 1 || len(b.faces) == 1 || hasCanonicalWrappedCylinderShape(a) || hasCanonicalWrappedCylinderShape(b)) {
				s, h, disjointErr := CombineDisjointSplineComponents(ctx, a, b, op, o.Transverse.Shells.Embedding)
				if disjointErr == nil {
					return s, h, nil
				}
				if !errors.Is(disjointErr, geom.ErrConvergence) {
					if !errors.Is(disjointErr, geom.ErrDegenerate) {
						return Solid{}, r, disjointErr
					}
				}
				e = errors.Join(e, disjointErr)
				s, h, nestedErr := CombineNestedSplineComponents(ctx, a, b, op, o.Transverse.Shells)
				if nestedErr == nil {
					return s, h, nil
				}
				if !errors.Is(nestedErr, geom.ErrConvergence) {
					return Solid{}, r, nestedErr
				}
				e = errors.Join(e, nestedErr)
			}
			return Solid{}, r, e
		}
	}
	if len(ap) != 0 && len(bp) != 0 && !o.Transverse.Trace.PlaneFallback.Enabled {
		o.Transverse.Trace.PlaneFallback.Enabled = true
		o.Transverse.Trace.PlaneFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	}
	var unresolved [][2]int
	for i, x := range ap {
		for j, y := range bp {
			if e := ctx.Err(); e != nil {
				return Solid{}, r, e
			}
			e := proveSplinePatchPairDisjoint(ctx, x.patch, y.patch, o.Transverse.Shells.Separation)
			if errors.Is(e, geom.ErrConvergence) {
				unresolved = append(unresolved, [2]int{i, j})
			} else if e != nil {
				return Solid{}, r, e
			}
		}
	}
	if len(unresolved) == 0 {
		s, h, e := CombineSeparatedSplineShells(ctx, a, b, op, o.Transverse.Shells)
		if e == nil {
			r = SplineShellCombineHistory{FacesA: faceRows(h.FacesA), FacesB: faceRows(h.FacesB), Method: "separated"}
		}
		return s, r, e
	}
	if h, same, e := coincidentSplineShells(ctx, ap, bp, o.Transverse.Trace.Coincidence); e != nil {
		return Solid{}, r, e
	} else if same {
		if op == parametric.Difference {
			return Solid{}, SplineShellCombineHistory{FacesA: make([][]int, len(ap)), FacesB: make([][]int, len(bp)), Method: "coincident"}, nil
		}
		m := make([][]int, len(ap))
		for i := range m {
			m[i] = []int{i}
		}
		return a, SplineShellCombineHistory{FacesA: m, FacesB: h, Method: "coincident"}, nil
	}
	if len(unresolved) >= 2 && len(unresolved) <= 8 && op != parametric.Union {
		pairs := make([]SplineShellTangencyPair, len(unresolved))
		for i, f := range unresolved {
			pairs[i].Faces = f
		}
		s, h, _, e := combineExternalSplineTangencySet(ctx, [2]Solid{a, b}, [2][]splineEmbeddingFace{ap, bp}, op, pairs,
			SplineShellTangencyOptions{Shells: o.Transverse.Shells}, true)
		if e == nil {
			return s, h, nil
		}
		if !errors.Is(e, geom.ErrConvergence) && !errors.Is(e, geom.ErrDegenerate) &&
			!errors.Is(e, geom.ErrDomain) && !errors.Is(e, geom.ErrUnrepresentable) {
			return Solid{}, r, e
		}
	}
	if len(unresolved) == 1 {
		f := unresolved[0]
		if o.ClosedEllipseTangency != nil {
			s, h, e := discoverSplineShellClosedEllipse(ctx, [2]Solid{a, b}, [2][]splineEmbeddingFace{ap, bp}, op,
				f, o.Transverse.Shells, *o.ClosedEllipseTangency)
			if e == nil {
				return s, h, nil
			}
			if !errors.Is(e, geom.ErrConvergence) {
				return Solid{}, r, e
			}
		}
		if op != parametric.Union {
			for _, axes := range [3][2]int{{0, 1}, {1, 2}, {0, 2}} {
				s, h, _, e := combineTangentSplineShellPair(ctx, [2]Solid{a, b}, [2][]splineEmbeddingFace{ap, bp}, op,
					SplineShellTangencyOptions{Faces: f, Projection: axes, Shells: o.Transverse.Shells}, true)
				if e == nil {
					return s, h, nil
				}
				if !errors.Is(e, geom.ErrConvergence) && !errors.Is(e, geom.ErrDegenerate) &&
					!errors.Is(e, geom.ErrDomain) && !errors.Is(e, geom.ErrUnrepresentable) {
					return Solid{}, r, e
				}
			}
		}
		s, h, e := CombineTransverseSplineShells(ctx, a, b, f[0], f[1], op, o.Transverse)
		if e == nil {
			return s, SplineShellCombineHistory{FacesA: h.FacesA, FacesB: h.FacesB, SectionEdges: h.SectionEdges, Method: "closed-contours"}, nil
		}
		if !errors.Is(e, geom.ErrConvergence) && !errors.Is(e, intersect.ErrUnresolved) && !errors.Is(e, geom.ErrInvalid) {
			return Solid{}, r, e
		}
	}
	if o.Join.Embedding == (SplineRectangleEmbeddingOptions{}) {
		o.Join.Embedding = o.Transverse.Shells.Embedding
	}
	for _, attempt := range []struct {
		name string
		join func(context.Context, Solid, Solid, parametric.BooleanOp, SplineShellJoinOptions) (Solid, SplineShellJoinHistory, error)
	}{{"shared-face", CombineFaceJoinedSplineShells}, {"inset-face", CombineInsetJoinedSplineShells}} {
		s, h, e := attempt.join(ctx, a, b, op, o.Join)
		if e == nil {
			return s, SplineShellCombineHistory{FacesA: faceRows(h.FacesA), FacesB: faceRows(h.FacesB), Method: attempt.name}, nil
		}
		if !errors.Is(e, geom.ErrConvergence) {
			return Solid{}, r, e
		}
	}
	s, h, e := CombineOverlappingInterfaceSplineShells(ctx, a, b, op, o.Join)
	if e == nil {
		return s, SplineShellCombineHistory{FacesA: h.FacesA, FacesB: h.FacesB, Method: "overlapping-interface"}, nil
	}
	if !errors.Is(e, geom.ErrConvergence) {
		return Solid{}, r, e
	}
	var unresolvedDiscovery error
	if !o.DisableAffineDiscovery || !o.DisablePlaneDiscovery || o.EnableGeneralTubeDiscovery {
		if o.Open.Shells.Embedding == (SplineRectangleEmbeddingOptions{}) {
			o.Open.Shells.Embedding = o.Transverse.Shells.Embedding
		}
		if o.Open.Shells.Separation == (geom.SurfaceSurfaceIsolationOptions{}) {
			o.Open.Shells.Separation = o.Transverse.Shells.Separation
		}
		o.Open.verticalQuadraticWarp = o.VerticalQuadraticWarp
		if !o.DisableAffineDiscovery && o.Open.verticalQuadraticWarp == 0 && o.DiscoverVerticalQuadraticWarp {
			k, e := discoverSplineShellVerticalQuadratic(ctx, ap, bp, o.Open.Trace.Seeds.Patches)
			if e != nil && !isUnresolvedOpenSpline(e) {
				return Solid{}, r, e
			}
			o.Open.verticalQuadraticWarp = k
		}
		var sections []SplineShellOpenSection
		method := "affine-open-wire"
		var e error
		if o.EnableGeneralTubeDiscovery {
			var curved bool
			sections, curved, e = discoverSplineShellSections(ctx, a, b, ap, bp, unresolved, o.Open,
				!o.DisableAffineDiscovery, !o.DisablePlaneDiscovery, true)
			if curved {
				method = "automatic-tube-arrangement"
			}
		} else if o.DisablePlaneDiscovery {
			sections, _, e = discoverSplineShellSections(ctx, a, b, ap, bp, unresolved, o.Open, true, false, false)
		} else {
			var curved bool
			sections, curved, e = discoverSplineShellSections(ctx, a, b, ap, bp, unresolved, o.Open, !o.DisableAffineDiscovery, true, false)
			if curved {
				method = "plane-open-wire"
			}
		}
		if len(sections) != 0 && o.Open.verticalQuadraticWarp != 0 {
			method = "vertical-quadratic-warp-arrangement"
		}
		if e != nil && !isUnresolvedOpenSpline(e) {
			return Solid{}, r, e
		}
		unresolvedDiscovery = e
		if len(sections) != 0 && e == nil {
			for _, s := range sections {
				if s.Closed {
					if method != "automatic-tube-arrangement" {
						method = "plane-section-arrangement"
					}
					break
				}
			}
			s, h, e := CombineSectionedSplineShells(ctx, a, b, op, sections, o.Open)
			if e == nil {
				return s, splineSectionCombineHistory(h, method), nil
			}
			if !isUnresolvedOpenSpline(e) {
				return Solid{}, r, e
			}
			unresolvedDiscovery = e
		}
	}
	if op != parametric.Union {
		if touch, e := proveSplineShellsTouchOnly(ctx, ap, bp, o.Transverse.Shells.Embedding.Plane); e != nil {
			return Solid{}, r, e
		} else if touch {
			s, h := regularizedTouchResult(a, b, op)
			return s, h, nil
		}
	}
	if !o.DisableAffineDiscovery && len(ap) != 0 && len(bp) != 0 {
		s, h, e := CombineAffineSplineShells(ctx, a, b, op, affineOptions)
		if e == nil {
			return s, h, nil
		}
		if !isUnsupportedAffineDispatch(e) {
			return Solid{}, r, e
		}
		if errors.Is(e, poly.ErrPrecision) {
			unresolvedDiscovery = errors.Join(unresolvedDiscovery, geom.ErrUnrepresentable, e)
		}
	}
	if unresolvedDiscovery != nil {
		return Solid{}, r, fmt.Errorf("trimmed: %d face pairs need further contact classification: %w", len(unresolved), errors.Join(geom.ErrConvergence, unresolvedDiscovery))
	}
	return Solid{}, r, fmt.Errorf("trimmed: %d face pairs need further contact classification: %w", len(unresolved), geom.ErrConvergence)
}

// Embeddings and every unselected original face pair are certified by the
// caller. Each candidate still has to prove the full contact and classify
// the original solid witnesses.
func discoverSplineShellClosedEllipse(ctx context.Context, solids [2]Solid, all [2][]splineEmbeddingFace, op parametric.BooleanOp,
	faces [2]int, shells SplineShellBooleanOptions, contact geom.GraphPointContactOptions) (Solid, SplineShellCombineHistory, error) {
	if contact.MaxOperations == 0 {
		contact.MaxOperations = 8000000
	}
	workLimit := contact.MaxOperations
	for _, axes := range [3][2]int{{0, 1}, {1, 2}, {0, 2}} {
		s, h, p, e := combineTangentSplineShellEllipse(ctx, solids, all, op, SplineShellRepeatedEllipseOptions{
			Faces: faces, Projection: axes, Shells: shells, Contact: contact,
		}, discoverTangentEllipseRelation)
		if e == nil {
			p.ExactOperations += workLimit - contact.MaxOperations
			h.ClosedEllipse = &p
			return s, h, nil
		}
		if !errors.Is(e, geom.ErrConvergence) && !errors.Is(e, geom.ErrDegenerate) &&
			!errors.Is(e, geom.ErrDomain) && !errors.Is(e, geom.ErrUnrepresentable) {
			return Solid{}, SplineShellCombineHistory{}, e
		}
		contact.MaxOperations -= p.ExactOperations
		if contact.MaxOperations < 1 {
			break
		}
	}
	return Solid{}, SplineShellCombineHistory{}, geom.ErrConvergence
}

func splineDispatchAffineOptions(o SplineShellCombineOptions) AffineSplineShellBooleanOptions {
	p := o.Transverse.Trace.Coincidence
	return AffineSplineShellBooleanOptions{
		Patches: o.Transverse.Shells.Embedding.Patches,
		Charts:  geom.AffineSurfaceChartOptions{MaxOperations: p.MaxOperations, MaxCoefficientBits: p.MaxCoefficientBits},
		Edges:   o.Transverse.Shells.Embedding.EdgeAgreement,
		Planar:  o.Planar,
	}
}

func isUnsupportedAffineDispatch(e error) bool {
	return errors.Is(e, geom.ErrConvergence) || errors.Is(e, geom.ErrUnrepresentable) || errors.Is(e, poly.ErrPrecision) || errors.Is(e, poly.ErrInvalid)
}

func splineSectionCombineHistory(h SplineShellOpenHistory, method string) SplineShellCombineHistory {
	return SplineShellCombineHistory{FacesA: h.FacesA, FacesB: h.FacesB, EdgesA: h.EdgesA, EdgesB: h.EdgesB, SectionEdges: h.SectionEdges, Method: method}
}

func coincidentSplineShells(ctx context.Context, a, b []splineEmbeddingFace, o intersect.SplineCoincidenceOptions) ([][]int, bool, error) {
	if len(a) != len(b) || len(a) == 0 {
		return nil, false, nil
	}
	// Each outward embedded single shell is a closed connected boundary.
	// A bijection of its complete chart images therefore proves the same
	// material domain, even with different face ordering and parameter bases.
	used := make([]bool, len(a))
	m := make([][]int, len(b))
	for i, x := range b {
		match := -1
		for j, y := range a {
			if used[j] {
				continue
			}
			// Control hulls can differ for equivalent spline bases, so
			// only strict box separation is a valid broad-phase reject.
			p, _ := y.patch.Bounds()
			q, _ := x.patch.Bounds()
			if _, overlap := p.Intersect(q); !overlap {
				continue
			}
			proof, e := intersect.FindExactSplineCoincidence(ctx, y.patch, x.patch, o)
			if e != nil && !errors.Is(e, geom.ErrConvergence) {
				return nil, false, e
			}
			if proof.Found {
				match = j
				break
			}
		}
		if match < 0 {
			return nil, false, nil
		}
		used[match] = true
		m[i] = []int{match}
	}
	return m, true, nil
}

func faceRows(in []int) [][]int {
	r := make([][]int, len(in))
	for i, x := range in {
		if x >= 0 {
			r[i] = []int{x}
		}
	}
	return r
}
