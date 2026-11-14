package trimmed

import (
	"context"
	"fmt"
	"math/big"

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

// ClipCrossingRationalGraphSplineShell retains the intersection with two
// nonparallel halfspaces. Each complete section has one simple outer wire;
// the other plane crosses it twice at strictly interior, exactly represented
// edge points. Each original stock face is either unchanged, wholly omitted,
// singly split, affine, or bounded by two graph sections in the same chart
// direction. A crossing between two graphs on one face must be unique. The
// kept sides can bound a band or select the upper or lower graph envelope.
// Each must leave one simple region. The two caps and paired stock trims
// are authored independently on the original supports.
func (s Solid) ClipCrossingRationalGraphSplineShell(ctx context.Context, cuts [2]RationalGraphHalfspace, o RationalGraphSeparatedBandOptions) (RationalGraphCrossingBand, error) {
	var zero RationalGraphCrossingBand
	if o.Caps.RationalEndpoints {
		return zero, geom.ErrInvalid
	}
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
	var out RationalGraphCrossingBand
	var signs [2][]int
	var clipped [2]*RationalGraphShellRegions
	var junctions [2][2]capBoundaryCut
	var capParts [2]RationalGraphSectionCap
	var arcs [2]map[int][]PCurve
	for i := range cuts {
		sec, sides, e := s.discoverRationalGraphShellSection(ctx, cuts[i].Points, planes[i], faces, so)
		if e != nil {
			return zero, fmt.Errorf("trimmed: crossing band section %d: %w", i, e)
		}
		out.Sections[i], signs[i] = sec, sides
		clipped[i], junctions[i], e = sec.Regions.intersectCapHalfspace(ctx, planes[1-i], keep[1-i], *so.Regions)
		if e != nil {
			return zero, fmt.Errorf("trimmed: crossing band cap %d: %w", i, e)
		}
		caps, e := clipped[i].BuildCaps(ctx, cuts[i].Cap, cuts[i].CapDomain, o.Caps)
		if e != nil {
			return zero, e
		}
		if len(caps) != 1 {
			return zero, geom.ErrConvergence
		}
		capParts[i] = caps[0]
		arcs[i], e = retainedRationalGraphCapArcs(sec, clipped[i], junctions[i], clipped[i].maxBits)
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
	if !(paired(junctions[0][0], junctions[1][0]) && paired(junctions[0][1], junctions[1][1]) ||
		paired(junctions[0][0], junctions[1][1]) && paired(junctions[0][1], junctions[1][0])) {
		return zero, geom.ErrConvergence
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
		switch {
		case !c0 && !c1:
			part, old = s.splineSewFaceMapped(face, nil)
		case !c1:
			part, err = splitRationalGraphFaceFragment(ctx, face, f.patch, b0, planes[0], keep[0], so.Spans.Faces, false)
		case !c0:
			part, err = splitRationalGraphFaceFragment(ctx, face, f.patch, b1, planes[1], keep[1], so.Spans.Faces, false)
		case b0.freeAxis < 0 && b1.freeAxis < 0:
			var uv [][2]*big.Rat
			uv, err = rationalGraphAffineBandPolygon(ctx, face, f.patch, planes, keep, so.Spans.Faces)
			if err == nil && len(uv) < 3 {
				continue
			}
			if err == nil {
				part, err = slabSewFace(ctx, face, f.patch, uv, so.Spans.Faces)
			}
			if err == nil {
				pieces := append(append([]PCurve(nil), arcs[0][id]...), arcs[1][id]...)
				err = installRationalGraphFragmentTrims(ctx, &part, pieces, false)
			}
		case b0.freeAxis >= 0 && b0.freeAxis == b1.freeAxis:
			part, err = rationalGraphCrossingFace(ctx, face, f.patch, [2]RationalGraphShellBranch{b0, b1},
				[2][]PCurve{arcs[0][id], arcs[1][id]}, keep, junctions[0], id, so.Spans.Faces)
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
	for i, capa := range capParts {
		part := capa.Boundary
		if keep[i] > 0 {
			part.Face.Reversed = !part.Face.Reversed
		}
		out.CapFaces[i] = []int{len(parts)}
		parts = append(parts, part)
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
	common := -1
	for i, p := range capParts {
		face := out.CapFaces[i][0]
		for j, source := range p.Edges {
			id := h.Edges[face][j]
			capEd[i] = append(capEd[i], id)
			if source.Branch >= 0 {
				out.CapEdges[i] = append(out.CapEdges[i], id)
				continue
			}
			if i == 0 {
				if common >= 0 {
					return zero, geom.ErrConvergence
				}
				common = id
			} else if common != id {
				return zero, geom.ErrConvergence
			}
			u := p.Boundary.Face.Outer[j]
			a, b := u.UVStart, u.UVEnd
			if h.EdgeReversed[face][j] {
				a, b = b, a
			}
			out.CapIntersection.UV[i] = [2][2]*big.Rat{{ratInset(a.X), ratInset(a.Y)}, {ratInset(b.X), ratInset(b.Y)}}
		}
	}
	if common < 0 || out.CapIntersection.UV[1][0][0] == nil {
		return zero, geom.ErrConvergence
	}
	for i, id := range [2]int{result.edges[common].Start, result.edges[common].End} {
		p := result.vertices[id]
		point := [3]*big.Rat{ratInset(p.X), ratInset(p.Y), ratInset(p.Z)}
		if !(rationalGraphSamePoint(point, junctions[0][0].Point) || rationalGraphSamePoint(point, junctions[0][1].Point)) {
			return zero, geom.ErrConvergence
		}
		out.CapIntersection.Points[i] = point
	}
	out.CapIntersection.Edge = common
	out.CapIntersection.Faces = [2]int{out.CapFaces[0][0], out.CapFaces[1][0]}
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
	pieces [2][]PCurve, keep [2]int, cuts [2]capBoundaryCut, faceID int, o SplineRectangleEmbeddingOptions) (SewFace, error) {
	free := branches[0].freeAxis
	above := [2]bool{branches[0].positive == keep[0], branches[1].positive == keep[1]}
	var junctions int
	for _, cut := range cuts {
		if cut.Source.Face == faceID {
			junctions++
		}
	}
	if junctions > 1 {
		return SewFace{}, geom.ErrConvergence
	}
	var chains [2][][2]*big.Rat
	for i, row := range pieces {
		if len(row) == 0 || len(row) > len(branches[i].Pieces) {
			return SewFace{}, geom.ErrConvergence
		}
		for j, p := range row {
			x, _, e := graphSectionCapEndpoints(p, false)
			if e != nil {
				return SewFace{}, e
			}
			if j == 0 {
				chains[i] = append(chains[i], x[0])
			}
			if !rationalGraphSameUV(chains[i][j], x[0]) {
				return SewFace{}, geom.ErrConvergence
			}
			chains[i] = append(chains[i], x[1])
		}
		if branches[i].reversed {
			reverseGraphUVChain(chains[i])
		}
		for j := 1; j < len(chains[i]); j++ {
			if chains[i][j-1][free].Cmp(chains[i][j][free]) >= 0 {
				return SewFace{}, geom.ErrConvergence
			}
		}
	}
	if above[0] == above[1] {
		if junctions != 1 {
			return SewFace{}, geom.ErrConvergence
		}
		return rationalGraphCrossingEnvelope(ctx, face, patch, free, above[0], chains, pieces, o)
	}
	a, b := chains[0], chains[1]
	if !above[0] {
		a, b = b, a
	}
	var shared int
	for _, ends := range [][2][2]*big.Rat{{a[0], b[0]}, {a[len(a)-1], b[len(b)-1]}} {
		x, y := ends[0], ends[1]
		if x[free].Cmp(y[free]) != 0 || x[1-free].Cmp(y[1-free]) > 0 {
			return SewFace{}, geom.ErrConvergence
		}
		if rationalGraphSameUV(x, y) {
			shared++
		}
	}
	if shared != junctions {
		return SewFace{}, geom.ErrConvergence
	}
	// The cap root counts prove the graph trims cannot meet again. Their
	// relative order is therefore the proved endpoint order on the run.
	if free == 1 {
		a, b = b, a
	}
	reverseGraphUVChain(b)
	if rationalGraphSameUV(a[len(a)-1], b[0]) {
		b = b[1:]
	}
	if rationalGraphSameUV(a[0], b[len(b)-1]) {
		b = b[:len(b)-1]
	}
	part, e := slabSewFace(ctx, face, patch, append(a, b...), o)
	if e != nil {
		return SewFace{}, e
	}
	if e = installRationalGraphFragmentTrims(ctx, &part, append(append([]PCurve(nil), pieces[0]...), pieces[1]...), false); e != nil {
		return SewFace{}, e
	}
	return part, nil
}

func rationalGraphCrossingEnvelope(ctx context.Context, face Face, patch *geom.SurfacePatch, free int, above bool,
	chains [2][][2]*big.Rat, pieces [2][]PCurve, o SplineRectangleEmbeddingOptions) (SewFace, error) {
	// Both retained intervals together must run from one original chart
	// boundary to the other, with exactly the independently proved join.
	a, b := chains[0], chains[1]
	if a[0][free].Cmp(b[0][free]) > 0 {
		a, b = b, a
	}
	d := patch.Domain()
	lo := [2]*big.Rat{ratInset(d.UMin), ratInset(d.VMin)}
	hi := [2]*big.Rat{ratInset(d.UMax), ratInset(d.VMax)}
	if a[0][free].Cmp(lo[free]) != 0 || b[len(b)-1][free].Cmp(hi[free]) != 0 || !rationalGraphSameUV(a[len(a)-1], b[0]) {
		return SewFace{}, geom.ErrConvergence
	}
	chain := append(a, b[1:]...)
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
	part, e := slabSewFace(ctx, face, patch, uv, o)
	if e != nil {
		return SewFace{}, e
	}
	if e = installRationalGraphFragmentTrims(ctx, &part, append(append([]PCurve(nil), pieces[0]...), pieces[1]...), false); e != nil {
		return SewFace{}, e
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

func retainedRationalGraphCapArcs(section RationalGraphShellSection, clipped *RationalGraphShellRegions, cuts [2]capBoundaryCut, bits int) (map[int][]PCurve, error) {
	retained := map[int][]PCurve{}
	for _, z := range clipped.wires[0] {
		s := z.source
		if s.Branch < 0 {
			continue
		}
		p := section.Branches[s.Branch].Pieces[s.Piece]
		for _, c := range cuts {
			if c.Source != s {
				continue
			}
			h, e := polynomialPlaneControls(p)
			if e != nil {
				return nil, e
			}
			a, b, e := splitCapBezier(h, c.Parameter, bits)
			if e != nil {
				return nil, e
			}
			if !c.KeepBefore {
				a = b
			}
			if len(a) == 3 {
				p, e = NewExactQuadraticUV([3][3]*big.Rat{a[0], a[1], a[2]})
			} else {
				p, e = NewExactBezierUV(a)
			}
			if e != nil {
				return nil, e
			}
		}
		retained[s.Face] = append(retained[s.Face], p)
	}
	return retained, nil
}
