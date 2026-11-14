package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"sort"

	"plainsolid/geom"
)

// RationalGraphCrossingBand keeps the two original full section proofs and
// the native chord built independently in the two cap charts. CapEdges
// excludes the chord. CapIntersection.Faces indexes the two returned cap
// faces; its UVs use the input cap charts in canonical model-edge direction.
type RationalGraphCrossingBand struct {
	RationalGraphSeparatedBand
	CapIntersection SplineCapIntersection
}

type graphCapArrangementMode uint8

const (
	graphCapSingleSeam graphCapArrangementMode = iota
	graphCapPolygonal
	graphCapRational
)

// ClipCrossingRationalGraphSplineShell retains the intersection with two
// nonparallel halfspaces. Each complete section has one simple outer wire;
// the other plane crosses it twice at strictly interior edge points. Any
// inner wires must stay strictly to one side. Each stock face is unchanged, omitted,
// singly split, affine, or bounded by two graph sections in the same chart
// direction. A crossing between two graphs on one face must be unique. The
// kept sides can bound a band or select the upper or lower graph envelope.
// Each must leave one simple region. The two caps and paired stock trims
// are authored independently on the original supports. Caps.RationalEndpoints
// admits exact nonbinary junctions when all retained trim pieces are quadratic.
func (s Solid) ClipCrossingRationalGraphSplineShell(ctx context.Context, cuts [2]RationalGraphHalfspace, o RationalGraphSeparatedBandOptions) (RationalGraphCrossingBand, error) {
	result, e := s.clipIntersectingRationalGraphCaps(ctx, cuts, o, graphCapSingleSeam)
	if e != nil {
		return RationalGraphCrossingBand{}, e
	}
	if len(result.CapIntersections) != 1 {
		return RationalGraphCrossingBand{}, geom.ErrConvergence
	}
	return RationalGraphCrossingBand{result.RationalGraphSeparatedBand, result.CapIntersections[0]}, nil
}

// RationalGraphMultiSeamBand reports all generated cap edges separately from
// stock/cap edges. CapIntersections is ordered by result model edge ID.
type RationalGraphMultiSeamBand struct {
	RationalGraphSeparatedBand
	CapIntersections []SplineCapIntersection
}

// ClipIntersectingPolygonalGraphCaps intersects stock with two halfspaces
// whose complete original section boundaries are exactly polygonal. Inner
// loop crossings and multiple cap components are admitted. The exact seams
// must be proved independently in both cap regions, then sewn to continuous
// original stock uses. Each original stock face retains at most one fragment.
func (s Solid) ClipIntersectingPolygonalGraphCaps(ctx context.Context, cuts [2]RationalGraphHalfspace, o RationalGraphSeparatedBandOptions) (RationalGraphMultiSeamBand, error) {
	return s.clipIntersectingRationalGraphCaps(ctx, cuts, o, graphCapPolygonal)
}

// ClipIntersectingRationalGraphCaps is the curved counterpart to
// ClipIntersectingPolygonalGraphCaps. All cap contacts must have distinct,
// regular rational parameters in their original boundary pieces. Each
// original stock chart can retain several separated intervals between two
// opposite-sided graph trims with the same free axis.
func (s Solid) ClipIntersectingRationalGraphCaps(ctx context.Context, cuts [2]RationalGraphHalfspace, o RationalGraphSeparatedBandOptions) (RationalGraphMultiSeamBand, error) {
	return s.clipIntersectingRationalGraphCaps(ctx, cuts, o, graphCapRational)
}

func (s Solid) clipIntersectingRationalGraphCaps(ctx context.Context, cuts [2]RationalGraphHalfspace, o RationalGraphSeparatedBandOptions, arrangementMode graphCapArrangementMode) (RationalGraphMultiSeamBand, error) {
	var zero RationalGraphMultiSeamBand
	rational := o.Caps.RationalEndpoints
	so := o.Sections
	if so.MaxPieces == 0 {
		so.MaxPieces = 128
	}
	if so.Regions == nil {
		so.Regions = &RationalGraphShellRegionOptions{}
	}
	if ctx == nil || so.MaxPieces < 1 || so.MaxPieces > 1024 || so.Spans.MaxSpans < 0 || so.Spans.MaxSpans > 128 {
		return zero, geom.ErrInvalid
	}
	var planes [2]polynomialPlaneEquation
	var keep [2]int
	for i, cut := range cuts {
		if cut.Cap == nil || !cut.CapDomain.Valid() {
			return zero, geom.ErrInvalid
		}
		for _, p := range cut.Points {
			if !p.IsFinite() {
				return zero, geom.ErrInvalid
			}
		}
		p, e := polynomialPlaneFromPoints(cut.Points)
		if e != nil {
			return zero, e
		}
		planes[i], keep[i] = p, -1
		if cut.KeepPositive {
			keep[i] = 1
		}
	}
	var faces []splineEmbeddingFace
	proof, e := s.validateSplineRectangleEmbedding(ctx, so.Spans.Faces, &faces)
	if e != nil {
		return zero, e
	}
	if !proof.Complete {
		return zero, geom.ErrConvergence
	}
	var out RationalGraphMultiSeamBand
	var signs [2][]int
	var clipped [2]*RationalGraphShellRegions
	var junctions [2][]capBoundaryCut
	var seamPairs [2][]RationalGraphCapSeam
	var capParts [2][]RationalGraphSectionCap
	var arcs [2]map[int][]PCurve
	for i := range cuts {
		sec, sides, e := s.discoverRationalGraphShellSection(ctx, cuts[i].Points, planes[i], faces, so)
		if e != nil {
			return zero, fmt.Errorf("trimmed: crossing band section %d: %w", i, e)
		}
		out.Sections[i], signs[i] = sec, sides
		if arrangementMode != graphCapSingleSeam {
			var arrangement RationalGraphCapArrangement
			arrangement, e = sec.Regions.intersectGeneralCap(ctx, planes[1-i], keep[1-i], *so.Regions, arrangementMode == graphCapPolygonal)
			clipped[i] = arrangement.Regions
			seamPairs[i] = arrangement.Seams
			for _, cut := range arrangement.Cuts {
				junctions[i] = append(junctions[i], capBoundaryCut(cut))
			}
		} else {
			var j [2]capBoundaryCut
			clipped[i], j, e = sec.Regions.intersectCapHalfspace(ctx, planes[1-i], keep[1-i], *so.Regions)
			junctions[i] = j[:]
			seamPairs[i] = []RationalGraphCapSeam{{0, 1}}
		}
		if e != nil {
			return zero, fmt.Errorf("trimmed: crossing band cap %d: %w", i, e)
		}
		caps, e := clipped[i].BuildCaps(ctx, cuts[i].Cap, cuts[i].CapDomain, o.Caps)
		if e != nil {
			return zero, e
		}
		if len(caps) == 0 {
			return zero, geom.ErrConvergence
		}
		capParts[i] = caps
		arcs[i], e = retainedRationalGraphCapArcs(sec, clipped[i], clipped[i].maxBits, rational)
		if e != nil {
			return zero, e
		}
	}
	paired := func(a, b capBoundaryCut) bool {
		if a.Source.Face != b.Source.Face {
			return false
		}
		for i, p := range a.Point {
			if p.Cmp(b.Point[i]) != 0 {
				return false
			}
		}
		return true
	}
	if len(seamPairs[0]) == 0 || len(seamPairs[0]) != len(seamPairs[1]) {
		return zero, geom.ErrConvergence
	}
	seen := make([]bool, len(seamPairs[1]))
	for _, a := range seamPairs[0] {
		match := -1
		for i, b := range seamPairs[1] {
			aa, ab := junctions[0][a.From], junctions[0][a.To]
			ba, bb := junctions[1][b.From], junctions[1][b.To]
			if paired(aa, ba) && paired(ab, bb) || paired(aa, bb) && paired(ab, ba) {
				if match >= 0 || seen[i] {
					return zero, geom.ErrConvergence
				}
				match = i
			}
		}
		if match < 0 {
			return zero, geom.ErrConvergence
		}
		seen[match] = true
	}
	lookup := [2]map[int]RationalGraphShellBranch{{}, {}}
	for i, sec := range out.Sections {
		for _, b := range sec.Branches {
			lookup[i][b.Face] = b
		}
	}
	var parts []SewFace
	var edges [][]int
	out.SourceFaces = make([][]int, len(faces))
	for id, f := range faces {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		if signs[0][id] == -keep[0] || signs[1][id] == -keep[1] {
			continue
		}
		b0, c0 := lookup[0][id]
		b1, c1 := lookup[1][id]
		face := s.faces[id]
		var part SewFace
		var old []int
		var err error
		a0, a1 := arcs[0][id], arcs[1][id]
		switch {
		case !c0 && !c1:
			part, old = s.splineSewFaceMapped(face, nil)
			if rational {
				part, err = rationalQuadraticSewFace(ctx, face, f.patch, slabFaceUV(face), so.Spans.Faces.Patches)
				old = nil
			}
		case !c1:
			part, err = splitRationalGraphFaceFragment(ctx, face, f.patch, b0, planes[0], keep[0], so.Spans.Faces, rational)
		case !c0:
			part, err = splitRationalGraphFaceFragment(ctx, face, f.patch, b1, planes[1], keep[1], so.Spans.Faces, rational)
		case len(a0) == 0 && len(a1) == 0:
			// Two transverse branches cross this rectangular face, but the
			// complete cap arrangement retains neither. Their intersection
			// cannot contain material in the face interior: otherwise the
			// boundary of that proper retained subset contains a branch or
			// (for an uncut whole face) contradicts both original crossings.
			continue
		case b0.freeAxis < 0 && b1.freeAxis < 0:
			var uv [][2]*big.Rat
			uv, err = rationalGraphAffineBandPolygon(ctx, face, f.patch, planes, keep, so.Spans.Faces)
			if err == nil && len(uv) < 3 {
				continue
			}
			if err == nil {
				pieces := append(append([]PCurve(nil), a0...), a1...)
				part, err = rationalGraphCrossingSewFace(ctx, face, f.patch, uv, pieces, so.Spans.Faces, rational)
			}
		case b0.freeAxis >= 0 && b0.freeAxis == b1.freeAxis:
			var fragments []SewFace
			fragments, err = rationalGraphCrossingFace(ctx, face, f.patch, [2]RationalGraphShellBranch{b0, b1},
				[2][]PCurve{a0, a1}, keep, junctions[0], id, so.Spans.Faces, rational)
			if err == nil {
				for _, part := range fragments {
					old, e := rationalGraphFragmentEdgeSources(face, part)
					if e != nil {
						return zero, e
					}
					out.SourceFaces[id] = append(out.SourceFaces[id], len(parts))
					parts, edges = append(parts, part), append(edges, old)
				}
				continue
			}
		default:
			err = geom.ErrConvergence
		}
		if err != nil {
			return zero, fmt.Errorf("trimmed: crossing band face %d: %w", id, err)
		}
		if old == nil {
			old, err = rationalGraphFragmentEdgeSources(face, part)
			if err != nil {
				return zero, err
			}
		}
		out.SourceFaces[id] = []int{len(parts)}
		parts, edges = append(parts, part), append(edges, old)
	}
	for i, row := range capParts {
		for _, cap := range row {
			part := cap.Boundary
			if keep[i] > 0 {
				part.Face.Reversed = !part.Face.Reversed
			}
			out.CapFaces[i] = append(out.CapFaces[i], len(parts))
			parts = append(parts, part)
		}
	}
	if rational {
		if e := proveRationalSplitVertexIdentity(ctx, parts); e != nil {
			return zero, e
		}
	}
	result, h, e := Sew(ctx, parts, SewOptions{Tolerance: s.tolerance, MaxFaces: len(parts), Proof: so.Spans.Faces.EdgeAgreement})
	if e != nil {
		return zero, e
	}
	for i, p := range parts {
		for j, v := range p.Vertices {
			if result.vertices[h.Vertices[i][j]] != v {
				return zero, geom.ErrConvergence
			}
		}
	}
	var capEd [2][]int
	common := make(map[int]*SplineCapIntersection)
	for i, row := range capParts {
		for k, p := range row {
			face := out.CapFaces[i][k]
			for j, source := range p.Edges {
				id := h.Edges[face][j]
				capEd[i] = append(capEd[i], id)
				if source.Branch >= 0 {
					out.CapEdges[i] = append(out.CapEdges[i], id)
					continue
				}
				seam := common[id]
				if i == 0 && seam == nil {
					seam = &SplineCapIntersection{Edge: id}
					common[id] = seam
				}
				if seam == nil || seam.UV[i][0][0] != nil || j >= len(p.Boundary.Face.Outer) {
					return zero, geom.ErrConvergence
				}
				u := p.Boundary.Face.Outer[j]
				uv, _, e := graphSectionCapEndpoints(u.Curve, rational)
				if e != nil {
					return zero, e
				}
				if h.EdgeReversed[face][j] {
					uv[0], uv[1] = uv[1], uv[0]
				}
				seam.UV[i], seam.Faces[i] = uv, face
			}
		}
	}
	if len(common) != len(seamPairs[0]) {
		return zero, geom.ErrConvergence
	}
	var exact map[int][3]*big.Rat
	if rational {
		v, e := result.RationalVertices(ctx)
		if e != nil {
			return zero, e
		}
		exact = make(map[int][3]*big.Rat, len(v))
		for _, p := range v {
			exact[p.Vertex] = p.Point
		}
	}
	matchedSeams := make([]bool, len(seamPairs[0]))
	for edge, model := range result.edges {
		seam := common[edge]
		if seam == nil {
			continue
		}
		if seam.UV[1][0][0] == nil {
			return zero, geom.ErrConvergence
		}
		for i, id := range [2]int{model.Start, model.End} {
			p := result.vertices[id]
			point := [3]*big.Rat{ratInset(p.X), ratInset(p.Y), ratInset(p.Z)}
			if rational {
				var ok bool
				if point, ok = exact[id]; !ok {
					return zero, geom.ErrConvergence
				}
			}
			seam.Points[i] = point
		}
		match := -1
		for j, pair := range seamPairs[0] {
			a, b := junctions[0][pair.From].Point, junctions[0][pair.To].Point
			if rationalGraphSamePoint(seam.Points[0], a) && rationalGraphSamePoint(seam.Points[1], b) ||
				rationalGraphSamePoint(seam.Points[1], a) && rationalGraphSamePoint(seam.Points[0], b) {
				if match >= 0 || matchedSeams[j] {
					return zero, geom.ErrConvergence
				}
				match = j
			}
		}
		if match < 0 {
			return zero, geom.ErrConvergence
		}
		matchedSeams[match] = true
		out.CapIntersections = append(out.CapIntersections, *seam)
	}
	out.SourceEdges, e = classifyRationalGraphSewnEdges(result, h, edges, len(s.edges), capEd[:])
	if e != nil {
		return zero, e
	}
	out.Solid = result
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}

func rationalGraphCrossingFace(ctx context.Context, face Face, patch *geom.SurfacePatch, branches [2]RationalGraphShellBranch,
	pieces [2][]PCurve, keep [2]int, cuts []capBoundaryCut, faceID int, o SplineRectangleEmbeddingOptions, rational bool) ([]SewFace, error) {
	free := branches[0].freeAxis
	above := [2]bool{branches[0].positive == keep[0], branches[1].positive == keep[1]}
	var junctions int
	for _, cut := range cuts {
		if cut.Source.Face == faceID {
			junctions++
		}
	}
	if above[0] != above[1] {
		if !above[0] {
			pieces[0], pieces[1] = pieces[1], pieces[0]
		}
		return rationalGraphCrossingStrip(ctx, face, patch, free, pieces, junctions, o, rational)
	}
	part, e := rationalGraphCrossingEnvelope(ctx, face, patch, free, above[0], junctions, pieces, o, rational)
	if e != nil {
		return nil, e
	}
	return []SewFace{part}, nil
}

type graphStripRun struct {
	points [][2]*big.Rat
	pieces []PCurve
}

func makeGraphStripRuns(free int, pieces []PCurve, rational bool) ([]graphStripRun, error) {
	type segment struct {
		ends [2][2]*big.Rat
		c    PCurve
	}
	row := make([]segment, 0, len(pieces))
	for _, p := range pieces {
		x, _, e := graphSectionCapEndpoints(p, rational)
		if e != nil {
			return nil, e
		}
		switch x[0][free].Cmp(x[1][free]) {
		case 0:
			return nil, geom.ErrConvergence
		case 1:
			x[0], x[1] = x[1], x[0]
		}
		row = append(row, segment{x, p})
	}
	sort.Slice(row, func(i, j int) bool { return row[i].ends[0][free].Cmp(row[j].ends[0][free]) < 0 })
	var runs []graphStripRun
	for _, p := range row {
		var prev *graphStripRun
		if len(runs) > 0 {
			prev = &runs[len(runs)-1]
			end := prev.points[len(prev.points)-1]
			switch end[free].Cmp(p.ends[0][free]) {
			case 1:
				return nil, geom.ErrConvergence
			case 0:
				if !rationalGraphSameUV(end, p.ends[0]) {
					return nil, geom.ErrConvergence
				}
				prev.points = append(prev.points, p.ends[1])
				prev.pieces = append(prev.pieces, p.c)
				continue
			}
		}
		runs = append(runs, graphStripRun{p.ends[:], []PCurve{p.c}})
	}
	return runs, nil
}

func rationalGraphCrossingStrip(ctx context.Context, face Face, patch *geom.SurfacePatch, free int,
	pieces [2][]PCurve, junctions int, o SplineRectangleEmbeddingOptions, rational bool) ([]SewFace, error) {
	var runs [2][]graphStripRun
	for i, row := range pieces {
		var e error
		runs[i], e = makeGraphStripRuns(free, row, rational)
		if e != nil {
			return nil, e
		}
	}
	if len(runs[0]) != len(runs[1]) || len(runs[0]) == 0 || len(runs[0]) > 8 {
		return nil, geom.ErrConvergence
	}
	var fragments []SewFace
	var shared int
	for i, lower := range runs[0] {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		higher := runs[1][i]
		part, joined, e := rationalGraphCrossingStripFace(ctx, face, patch, free, lower, higher, o, rational)
		if e != nil {
			return nil, fmt.Errorf("run %d (axis %d): %w", i, free, e)
		}
		shared += joined
		fragments = append(fragments, part)
	}
	if shared != junctions {
		return nil, geom.ErrConvergence
	}
	return fragments, nil
}

func rationalGraphCrossingStripFace(ctx context.Context, face Face, patch *geom.SurfacePatch, free int,
	lower, higher graphStripRun, o SplineRectangleEmbeddingOptions, rational bool) (SewFace, int, error) {
	a, b := lower.points, higher.points
	var shared int
	for _, ends := range [][2][2]*big.Rat{{a[0], b[0]}, {a[len(a)-1], b[len(b)-1]}} {
		x, y := ends[0], ends[1]
		if x[free].Cmp(y[free]) != 0 || x[1-free].Cmp(y[1-free]) > 0 {
			return SewFace{}, 0, fmt.Errorf("unordered graph run terminal %s versus %s: %w", x, y, geom.ErrConvergence)
		}
		if rationalGraphSameUV(x, y) {
			shared++
		}
	}
	// The cap root counts prove the graph trims cannot meet again. Their
	// relative order is therefore the proved endpoint order on the run.
	if free == 1 {
		a, b = b, a
	}
	reverseGraphUVChain(b)
	joinAtEnd := rationalGraphSameUV(a[len(a)-1], b[0])
	joinAtStart := rationalGraphSameUV(a[0], b[len(b)-1])
	if joinAtEnd {
		b = b[1:]
	}
	if joinAtStart && len(b) > 0 {
		b = b[:len(b)-1]
	}
	// A two-edge lens needs paired subdivisions on the cap curves as well;
	// the native sew operation keys open edges by both endpoint IDs.
	if len(a)+len(b) < 3 {
		return SewFace{}, 0, geom.ErrConvergence
	}
	p, e := rationalGraphCrossingSewFace(ctx, face, patch, append(a, b...), append(append([]PCurve(nil), lower.pieces...), higher.pieces...), o, rational)
	return p, shared, e
}

func rationalGraphCrossingEnvelope(ctx context.Context, face Face, patch *geom.SurfacePatch, free int, above bool, junctions int,
	pieces [2][]PCurve, o SplineRectangleEmbeddingOptions, rational bool) (SewFace, error) {
	var runs []graphStripRun
	for _, row := range pieces {
		p, e := makeGraphStripRuns(free, row, rational)
		if e != nil {
			return SewFace{}, e
		}
		runs = append(runs, p...)
	}
	if len(runs) != junctions+1 || len(runs) > 9 {
		return SewFace{}, geom.ErrConvergence
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].points[0][free].Cmp(runs[j].points[0][free]) < 0 })
	// All active graph intervals together must run from one original chart
	// boundary to the other. Every change of owner must be a proved join.
	d := patch.Domain()
	lo := [2]*big.Rat{ratInset(d.UMin), ratInset(d.VMin)}
	hi := [2]*big.Rat{ratInset(d.UMax), ratInset(d.VMax)}
	if runs[0].points[0][free].Cmp(lo[free]) != 0 {
		return SewFace{}, geom.ErrConvergence
	}
	chain := append([][2]*big.Rat(nil), runs[0].points...)
	for _, row := range runs[1:] {
		if !rationalGraphSameUV(chain[len(chain)-1], row.points[0]) {
			return SewFace{}, geom.ErrConvergence
		}
		chain = append(chain, row.points[1:]...)
	}
	if chain[len(chain)-1][free].Cmp(hi[free]) != 0 {
		return SewFace{}, geom.ErrConvergence
	}
	var uv [][2]*big.Rat
	switch {
	case free == 1 && !above:
		uv = append([][2]*big.Rat{lo}, chain...)
		uv = append(uv, [2]*big.Rat{lo[0], hi[1]})
	case free == 1:
		reverseGraphUVChain(chain)
		uv = append([][2]*big.Rat{{hi[0], lo[1]}, hi}, chain...)
	case !above:
		reverseGraphUVChain(chain)
		uv = append([][2]*big.Rat{lo, {hi[0], lo[1]}}, chain...)
	default:
		uv = append(chain, hi, [2]*big.Rat{lo[0], hi[1]})
	}
	return rationalGraphCrossingSewFace(ctx, face, patch, uv, append(append([]PCurve(nil), pieces[0]...), pieces[1]...), o, rational)
}

func rationalGraphCrossingSewFace(ctx context.Context, face Face, patch *geom.SurfacePatch, uv [][2]*big.Rat, pieces []PCurve, o SplineRectangleEmbeddingOptions, rational bool) (SewFace, error) {
	var part SewFace
	var e error
	if rational {
		part, e = rationalQuadraticSewFace(ctx, face, patch, uv, o.Patches)
	} else {
		part, e = slabSewFace(ctx, face, patch, uv, o)
	}
	if e != nil {
		return SewFace{}, fmt.Errorf("exact vertices: %w", e)
	}
	if e = installRationalGraphFragmentTrims(ctx, &part, pieces, rational); e != nil {
		return SewFace{}, fmt.Errorf("original trims: %w", e)
	}
	return part, nil
}

func rationalGraphSameUV(a, b [2]*big.Rat) bool {
	return a[0].Cmp(b[0]) == 0 && a[1].Cmp(b[1]) == 0
}

func rationalGraphSamePoint(a, b [3]*big.Rat) bool {
	for i, p := range a {
		if p.Cmp(b[i]) != 0 {
			return false
		}
	}
	return true
}

func retainedRationalGraphCapArcs(section RationalGraphShellSection, clipped *RationalGraphShellRegions, bits int, rational bool) (map[int][]PCurve, error) {
	retained := map[int][]PCurve{}
	for _, wire := range clipped.wires {
		for _, z := range wire {
			s := z.source
			if s.Branch < 0 {
				continue
			}
			p := section.Branches[s.Branch].Pieces[s.Piece]
			if z.span[0] != nil || z.span[1] != nil {
				h, e := polynomialPlaneControls(p)
				if e != nil {
					return nil, e
				}
				a, e := restrictCapBezier(h, z.span[0], z.span[1], bits)
				if e != nil {
					return nil, e
				}
				if len(a) == 3 {
					p, e = newExactQuadraticUV([3][3]*big.Rat{a[0], a[1], a[2]}, rational)
				} else {
					p, e = NewExactBezierUV(a)
				}
				if e != nil {
					return nil, e
				}
			}
			retained[s.Face] = append(retained[s.Face], p)
		}
	}
	return retained, nil
}
