package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func intersectSplineFace(f Face, d geom.ParameterRectangle) (intersect.Patch, error) {
	if _, ok := f.Surface.(*geom.BSplineSurface); !ok {
		return intersect.Patch{}, geom.ErrInvalid
	}
	return intersect.Patch{Surface: f.Surface, Min: geom.Vec2{X: d.UMin, Y: d.VMin}, Max: geom.Vec2{X: d.UMax, Y: d.VMax}}, nil
}

func proveSplinePatchPairDisjoint(ctx context.Context, a, b *geom.SurfacePatch, o geom.SurfaceSurfaceIsolationOptions) error {
	x, _ := a.Bounds()
	y, _ := b.Bounds()
	if _, overlap := x.Intersect(y); !overlap {
		return nil
	}
	o.SeparatingProjections = true
	r, e := geom.BoundSurfaceSurfaceContext(ctx, a, b, o)
	if e != nil && !errors.Is(e, geom.ErrConvergence) {
		return e
	}
	if !r.Complete || len(r.Candidates) != 0 || len(r.Coincident) != 0 {
		return geom.ErrConvergence
	}
	return nil
}

// SplineShellSection describes one complete transverse branch between one
// face of each shell. Branch is the zero-based index in the branch graph
// derived from TubeCells. When a pair has multiple branches, the first
// section for that pair supplies the cells for all branches; later sections
// for that pair leave TubeCells empty. The sections must cover every branch
// exactly once. Closed branches lie strictly inside both face charts and
// do not have endpoint hints. Each open endpoint names the shell whose
// original edge it meets, along with exact [uA,vA,uB,vB] coordinates. The
// other shell's chart must be interior unless OtherSide is provided and
// both original edge anchors and the local continuation are verified.
type SplineShellSection struct {
	FaceA, FaceB int
	Branch       int
	Closed       bool
	TubeCells    intersect.SplineSeedReport
	Start, End   SplineShellOpenEndpoint
}

// SplineShellOpenSection describes an edge-crossing branch. Closed must
// be false when passed to CombineOpenSplineShells.
type SplineShellOpenSection = SplineShellSection

type SplineShellOpenEndpoint struct {
	Operand int
	Edge    SplineTubeEndpoint
	// OtherSide identifies a simultaneous boundary event on the other
	// operand. The four exact parameters in Edge.Hint are shared.
	OtherSide *geom.SurfaceEdge
}

// SplineShellOpenOptions controls exact per-pair coverage, original shell
// embedding, endpoint reconciliation, and final sewing.
type SplineShellOpenOptions struct {
	Shells    SplineShellBooleanOptions
	Trace     intersect.SplinePairTraceOptions
	Endpoints geom.TubeEndpointEdgeOptions
	Sew       SewOptions
	// EnableBoundaryContactHints independently isolates rectangular source
	// edges against the other face when a regular tube's terminal recovery
	// cannot find a rational point. The final tube proof is still required.
	EnableBoundaryContactHints bool
	BoundaryContacts           geom.CurveSurfaceIntersectionOptions
	verticalQuadraticWarp      float64
}

// SplineShellOpenHistory records certifications in first face-pair occurrence
// order. Each face and edge row lists retained result IDs from that original
// entity. Edge rows follow increasing original curve parameter. Generated
// section edges have no original edge and appear in SectionEdges.
type SplineShellOpenHistory struct {
	Pairs          []intersect.SplinePairTrace
	FacePairs      [][2]int // same order as Pairs
	FacesA, FacesB [][]int
	EdgesA, EdgesB [][]int
	SectionEdges   []int
}

// CombineOpenSplineShells combines two embedded rectangular spline shells
// along transverse wires crossing original faces. It is intended
// for callers supplying candidate tube cells and exact endpoint hints, all
// independently verified. A face cut may include pieces from several
// opposing faces. It rejects unresolved pairs, closed face-pair branches,
// tangential boundary arrangements, and unrepresentable split data.
func CombineOpenSplineShells(ctx context.Context, a, b Solid, op parametric.BooleanOp, sections []SplineShellOpenSection,
	o SplineShellOpenOptions) (Solid, SplineShellOpenHistory, error) {
	if len(sections) < 2 {
		return Solid{}, SplineShellOpenHistory{}, geom.ErrInvalid
	}
	for _, s := range sections {
		if s.Closed {
			return Solid{}, SplineShellOpenHistory{}, geom.ErrInvalid
		}
	}
	return CombineSectionedSplineShells(ctx, a, b, op, sections, o)
}

// CombineSectionedSplineShells assembles complete transverse arrangements on
// two embedded rectangular spline shells. Closed loops may lie inside either
// face chart, and open paths can cross original shell edges. This operation
// verifies every proposed tube, terminal, and face pair before assembly.
func CombineSectionedSplineShells(ctx context.Context, a, b Solid, op parametric.BooleanOp, sections []SplineShellSection,
	o SplineShellOpenOptions) (Solid, SplineShellOpenHistory, error) {
	var h SplineShellOpenHistory
	if o.Shells.MaxRays == 0 {
		o.Shells.MaxRays = 12
	}
	if ctx == nil || op > parametric.Difference || len(sections) == 0 || len(sections) > 64 || o.Shells.MaxRays < 1 || o.Shells.MaxRays > 12 {
		return Solid{}, h, geom.ErrInvalid
	}
	ap, bp, chains, h, e := certifySectionedSplineShells(ctx, a, b, sections, o)
	if e != nil {
		return Solid{}, h, e
	}
	return assembleOpenSplineWire(ctx, a, b, ap, bp, chains, op, h, o)
}

func certifySectionedSplineShells(ctx context.Context, a, b Solid, sections []SplineShellSection,
	o SplineShellOpenOptions) (ap, bp []splineEmbeddingFace, w openSplineWire, h SplineShellOpenHistory, err error) {
	if _, e := a.validateSplineRectangleEmbedding(ctx, o.Shells.Embedding, &ap); e != nil {
		return nil, nil, w, h, e
	}
	if _, e := b.validateSplineRectangleEmbedding(ctx, o.Shells.Embedding, &bp); e != nil {
		return nil, nil, w, h, e
	}
	curves, e := certifyOpenSplinePairs(ctx, a, b, ap, bp, sections, o, &h)
	if e != nil {
		return nil, nil, w, h, e
	}
	w, err = prepareSplineShellWire(ctx, a, b, ap, bp, sections, curves, o.Endpoints)
	return ap, bp, w, h, err
}

func prepareSplineShellWire(ctx context.Context, a, b Solid, ap, bp []splineEmbeddingFace,
	sections []SplineShellSection, curves [][]intersect.TubeSegment, o geom.TubeEndpointEdgeOptions) (openSplineWire, error) {
	var opened []SplineShellOpenSection
	var openCurves [][]intersect.TubeSegment
	for i, s := range sections {
		if !s.Closed {
			opened, openCurves = append(opened, s), append(openCurves, curves[i])
		}
	}
	chains, e := certifyOpenSplineWire(ctx, a, b, opened, openCurves, o)
	if e != nil {
		return openSplineWire{}, e
	}
	if e := chains.addClosedRings(ctx, sections, curves, ap, bp); e != nil {
		return openSplineWire{}, e
	}
	return chains, nil
}

type openSplineBranch struct {
	faces    [2]int
	segments []intersect.TubeSegment
	ends     [2]openSplineEndpoint
	offset   int
}

type openSplineEndpoint struct {
	operand int
	anchor  SplineTubeAnchor
	other   *SplineTubeAnchor
	point   [3]*big.Rat
	uv      [2]geom.Vec2
}

func (p openSplineEndpoint) anchorOn(operand int) (EdgeAnchor, bool) {
	if p.operand == operand {
		return p.anchor.Anchor, true
	}
	if p.other != nil {
		return p.other.Anchor, true
	}
	return EdgeAnchor{}, false
}

type openSplineWire struct {
	branches []openSplineBranch
	joins    [][2]int // encoded as 2*branch+endpoint index
	rings    []closedSplineRing
}

func certifyOpenSplinePairs(ctx context.Context, a, b Solid, ap, bp []splineEmbeddingFace, sections []SplineShellOpenSection,
	o SplineShellOpenOptions, h *SplineShellOpenHistory) ([][]intersect.TubeSegment, error) {
	type pairProof struct {
		index int
		seen  []bool
	}
	proofs := map[[2]int]pairProof{}
	isolated := map[[2]int][]splineIsolatedEdgePair{}
	edgeIncidence := map[[2]EdgeAnchor]SplineEdgePairIncidence{}
	curves := make([][]intersect.TubeSegment, len(sections))
	for i, s := range sections {
		id := [2]int{s.FaceA, s.FaceB}
		if id[0] < 0 || id[0] >= len(ap) || id[1] < 0 || id[1] >= len(bp) || s.Branch < 0 {
			return nil, geom.ErrInvalid
		}
		proof, exists := proofs[id]
		if exists && len(s.TubeCells.Attempts) != 0 || !exists && len(s.TubeCells.Attempts) == 0 {
			return nil, geom.ErrInvalid
		}
		if !exists {
			p, e := intersectSplineFace(a.faces[id[0]], ap[id[0]].patch.Domain())
			if e != nil {
				return nil, e
			}
			q, e := intersectSplineFace(b.faces[id[1]], bp[id[1]].patch.Domain())
			if e != nil {
				return nil, e
			}
			r, e := intersect.CertifySplinePairBranches(ctx, p, q, s.TubeCells, o.Trace)
			proof.index = len(h.Pairs)
			h.Pairs = append(h.Pairs, r)
			h.FacePairs = append(h.FacePairs, id)
			if e != nil {
				return nil, fmt.Errorf("trimmed: open branch A:%d B:%d: %w", id[0], id[1], e)
			}
			if !r.Complete || len(r.Curves) == 0 || len(r.Curves) != len(r.BranchGraph.Branches) {
				return nil, geom.ErrConvergence
			}
			proof.seen = make([]bool, len(r.Curves))
			proofs[id] = proof
		}
		if s.Branch >= len(proof.seen) || proof.seen[s.Branch] {
			return nil, geom.ErrInvalid
		}
		r := h.Pairs[proof.index]
		c := r.Curves[s.Branch]
		if r.BranchGraph.Branches[s.Branch].Closed != s.Closed || len(c) == 0 || len(c) > 64 {
			return nil, fmt.Errorf("trimmed: unexpected branch kind or size on A:%d B:%d: %w", id[0], id[1], geom.ErrConvergence)
		}
		proof.seen[s.Branch], curves[i] = true, c
		for end, endpoint := range [2]SplineShellOpenEndpoint{s.Start, s.End} {
			if s.Closed || endpoint.OtherSide == nil {
				continue
			}
			k := 0
			if end == 1 {
				k = len(c) - 1
			}
			anchors, e := splineSimultaneousAnchors(ctx, a, b, id, c[k], end, endpoint, o.Endpoints)
			if e != nil {
				return nil, e
			}
			incidence, known := edgeIncidence[anchors]
			if !known {
				incidence, e = CertifySplineEdgePairIncidence(ctx, a, b, anchors, o.Endpoints)
				if e != nil {
					return nil, e
				}
				edgeIncidence[anchors] = incidence
			} else {
				continue
			}
			for _, germ := range incidence.Germs {
				if germ.Inward {
					continue
				}
				pair := [2]int{germ.Faces[0].Face, germ.Faces[1].Face}
				x := splineIsolatedEdgePair{}
				for j, f := range germ.Faces {
					x.parameters[2*j], x.parameters[2*j+1] = f.Parameters[0], f.Parameters[1]
					x.axes[j] = 2*j + int(f.Side)/2
				}
				isolated[pair] = append(isolated[pair], x)
			}
		}
	}
	for i, x := range ap {
		for j, y := range bp {
			if proof, ok := proofs[[2]int{i, j}]; ok {
				for _, ok := range proof.seen {
					if !ok {
						return nil, fmt.Errorf("trimmed: missing branch on A:%d B:%d: %w", i, j, geom.ErrConvergence)
					}
				}
				continue
			}
			covered := false
			for _, event := range isolated[[2]int{i, j}] {
				e := geom.CertifySurfacePairOnlyBoundaryPoint(ctx, x.patch, y.patch, geom.SurfacePairBoundaryPointOptions{
					Anchor: event.parameters, Axes: event.axes, MaxOperations: o.Endpoints.MaxOperations, MaxCoefficientBits: o.Endpoints.MaxCoefficientBits,
				})
				if e == nil {
					covered = true
					break
				}
				if !isUnresolvedOpenSpline(e) && !errors.Is(e, geom.ErrDomain) {
					return nil, e
				}
			}
			if !covered {
				if e := proveSplinePatchPairDisjoint(ctx, x.patch, y.patch, o.Shells.Separation); e != nil {
					return nil, fmt.Errorf("trimmed: unexpected contact pair A:%d B:%d: %w", i, j, e)
				}
			}
		}
	}
	return curves, nil
}

type splineIsolatedEdgePair struct {
	parameters [4]*big.Rat
	axes       [2]int
}

func splineSimultaneousAnchors(ctx context.Context, a, b Solid, faces [2]int, segment intersect.TubeSegment,
	end int, h SplineShellOpenEndpoint, o geom.TubeEndpointEdgeOptions) (anchors [2]EdgeAnchor, err error) {
	if h.OtherSide == nil || h.Operand < 0 || h.Operand > 1 {
		return anchors, geom.ErrInvalid
	}
	solids := [2]Solid{a, b}
	x, e := solids[h.Operand].CertifySplineTubeAnchor(ctx, faces[h.Operand], segment, end, h.Operand, h.Edge.Side, h.Edge.Hint, o)
	if e != nil {
		return anchors, e
	}
	j := h.Operand ^ 1
	y, e := solids[j].certifySplineTubeAnchor(ctx, faces[j], segment, end, j, *h.OtherSide, h.Edge.Hint, o, true)
	if e != nil {
		return anchors, e
	}
	anchors[h.Operand], anchors[j] = x.Anchor, y.Anchor
	return anchors, nil
}

func certifyOpenSplineWire(ctx context.Context, a, b Solid, sections []SplineShellOpenSection, curves [][]intersect.TubeSegment,
	o geom.TubeEndpointEdgeOptions) (openSplineWire, error) {
	w := openSplineWire{joins: make([][2]int, len(sections))}
	for i := range w.joins {
		w.joins[i] = [2]int{-1, -1}
	}
	type edgeKey struct {
		operand int
		anchor  EdgeAnchor
	}
	used := map[edgeKey][]int{}
	simultaneous := map[[2]EdgeAnchor][]int{}
	count := 0
	for i, s := range sections {
		branch := openSplineBranch{faces: [2]int{s.FaceA, s.FaceB}, segments: append([]intersect.TubeSegment(nil), curves[i]...), offset: count}
		count += len(branch.segments)
		if count > 256 {
			return openSplineWire{}, parametric.ErrBudget
		}
		for end, h := range [2]SplineShellOpenEndpoint{s.Start, s.End} {
			if e := ctx.Err(); e != nil {
				return openSplineWire{}, e
			}
			if h.Operand < 0 || h.Operand > 1 {
				return openSplineWire{}, geom.ErrInvalid
			}
			index := 0
			if end == 1 {
				index = len(branch.segments) - 1
			}
			segment := branch.segments[index]
			solid := [2]Solid{a, b}[h.Operand]
			q, e := solid.CertifySplineTubeAnchor(ctx, branch.faces[h.Operand], segment, end, h.Operand, h.Edge.Side, h.Edge.Hint, o)
			if e != nil {
				return openSplineWire{}, fmt.Errorf("trimmed: open wire branch %d endpoint %d: %w", i, end, e)
			}
			uv := [2]geom.Vec2{}
			for j := range uv {
				uv[j].X, _ = q.Proof.Parameters[2*j].Float64()
				uv[j].Y, _ = q.Proof.Parameters[2*j+1].Float64()
			}
			other := h.Operand ^ 1
			face := [2]Solid{a, b}[other].faces[branch.faces[other]]
			d, e := faceUVBounds(face)
			if e != nil {
				return openSplineWire{}, e
			}
			p := uv[other]
			var second *SplineTubeAnchor
			if h.OtherSide == nil {
				if p.X <= d.UMin || p.X >= d.UMax || p.Y <= d.VMin || p.Y >= d.VMax {
					return openSplineWire{}, geom.ErrConvergence
				}
			} else {
				x, e := [2]Solid{a, b}[other].certifySplineTubeAnchor(ctx, branch.faces[other], segment, end, other, *h.OtherSide, h.Edge.Hint, o, true)
				if e != nil {
					return openSplineWire{}, fmt.Errorf("trimmed: second edge of branch %d endpoint %d: %w", i, end, e)
				}
				second = &x
			}
			fixed, e := segment.WithExactEndpoint(ctx, end, q.Proof.Parameters, geom.TubeEndpointOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
			if e != nil {
				return openSplineWire{}, e
			}
			branch.segments[index] = fixed
			pair := openSplineEndpoint{h.Operand, q, second, q.Proof.Point, uv}
			branch.ends[end] = pair
			if second == nil {
				key := edgeKey{h.Operand, q.Anchor}
				used[key] = append(used[key], 2*i+end)
			} else {
				x, _ := pair.anchorOn(0)
				y, _ := pair.anchorOn(1)
				k := [2]EdgeAnchor{x, y}
				simultaneous[k] = append(simultaneous[k], 2*i+end)
			}
		}
		w.branches = append(w.branches, branch)
	}
	for _, pair := range used {
		if len(pair) != 2 {
			return openSplineWire{}, geom.ErrConvergence
		}
		x, y := w.branches[pair[0]/2], w.branches[pair[1]/2]
		p, q := x.ends[pair[0]%2], y.ends[pair[1]%2]
		if p.anchor.Anchor != q.anchor.Anchor || !exactPointCoordinatesEqual(p.point, q.point) ||
			x.faces[p.operand] == y.faces[p.operand] || x.faces[p.operand^1] != y.faces[p.operand^1] || p.uv[p.operand^1] != q.uv[p.operand^1] {
			return openSplineWire{}, geom.ErrConvergence
		}
		w.joins[pair[0]/2][pair[0]%2], w.joins[pair[1]/2][pair[1]%2] = pair[1], pair[0]
	}
	for anchors, pair := range simultaneous {
		if len(pair) != 2 || len(used[edgeKey{0, anchors[0]}]) != 0 || len(used[edgeKey{1, anchors[1]}]) != 0 {
			return openSplineWire{}, geom.ErrConvergence
		}
		incidence, e := CertifySplineEdgePairIncidence(ctx, a, b, anchors, o)
		if e != nil {
			return openSplineWire{}, e
		}
		expected := map[[2]int]bool{}
		for _, g := range incidence.Germs {
			if g.Inward {
				expected[[2]int{g.Faces[0].Face, g.Faces[1].Face}] = true
			}
		}
		if len(expected) != 2 {
			return openSplineWire{}, geom.ErrConvergence
		}
		for _, id := range pair {
			br := w.branches[id/2]
			if !expected[br.faces] || !exactPointCoordinatesEqual(incidence.Point, br.ends[id%2].point) {
				return openSplineWire{}, geom.ErrConvergence
			}
			delete(expected, br.faces)
		}
		x, y := w.branches[pair[0]/2], w.branches[pair[1]/2]
		if x.faces[0] == y.faces[0] || x.faces[1] == y.faces[1] {
			// One chart touches its boundary then returns into the
			// same face. The current planar arrangement needs an
			// explicit tangential-boundary vertex for that case.
			return openSplineWire{}, geom.ErrConvergence
		}
		w.joins[pair[0]/2][pair[0]%2], w.joins[pair[1]/2][pair[1]%2] = pair[1], pair[0]
	}
	seen := map[int]bool{}
	for start := range w.branches {
		if seen[start] {
			continue
		}
		i := 2 * start
		for !seen[i/2] {
			seen[i/2] = true
			i = w.joins[i/2][(i%2)^1]
			if i < 0 {
				return openSplineWire{}, geom.ErrConvergence
			}
		}
		if i != 2*start {
			return openSplineWire{}, geom.ErrConvergence
		}
	}
	return w, nil
}
