//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// BuildProjectiveSplineRoofBandSolid closes a Common or Fuse with two
// through sixteen complete parallel sections over the same original
// rectangular footprint. Every section and exterior subcurve must
// compose within one original support span. RoofFragments identifies all
// retained fragments of each original surface in increasing band order.
func BuildProjectiveSplineRoofBandSolid(ctx context.Context, a, b Face, axes [2]int, baseHeight float64,
	op parametric.BooleanOp, o ProjectiveRoofSolidOptions) (r ProjectiveRoofSolid, err error) {
	if op != parametric.Intersection && op != parametric.Union || o.Envelope.MaxClassifyOperations < 0 ||
		o.Envelope.Imprint.MaxEdgeOperations < 0 {
		return r, geom.ErrInvalid
	}
	p, err := proveProjectiveRoofFootprints(ctx, a, b, axes, baseHeight, o)
	if err != nil {
		return r, err
	}
	if !p.covers(0) || !p.covers(1) {
		return r, geom.ErrConvergence
	}
	graph, err := TraceProjectiveSplineFaceContacts(ctx, a, b, axes, o.Envelope.Imprint.Contacts)
	if err != nil {
		return r, err
	}
	if len(graph.Graph.Contours) < 2 || len(graph.Graph.Contours) > 16 {
		return r, geom.ErrConvergence
	}
	composition := o.Build
	if x := o.Envelope.Imprint.MaxEdgeOperations; x > 0 && (composition.MaxOperations == 0 || x < composition.MaxOperations) {
		composition.MaxOperations = x
	}
	bands, err := graph.Graph.BuildUnsplitParallelContactBands(ctx, composition)
	if err != nil {
		return r, err
	}
	bySection := make([][2]ExactQuadraticUV, len(bands.Sections))
	for i, s := range bands.Sections {
		for axis, uv := range s.Whole.UV {
			q, _, err := projectiveSectionUV(ctx, uv, o.Build)
			if err != nil {
				return r, err
			}
			bySection[i][axis] = q
		}
	}
	maxClassify := o.Envelope.MaxClassifyOperations
	if maxClassify == 0 {
		maxClassify = 2_000_000
	}
	classify := func(axis int, station, transverse *big.Rat) (geom.ProjectiveContactHeight, error) {
		var uv [2]*big.Rat
		uv[axis], uv[1-axis] = station, transverse
		if maxClassify < 1 {
			return geom.ProjectiveContactHeight{}, geom.ErrConvergence
		}
		h, err := graph.Graph.ClassifyCommonHeightRational(ctx, uv,
			geom.SurfacePatchEvaluationOptions{MaxOperations: maxClassify, MaxCoefficientBits: o.Build.MaxCoefficientBits})
		maxClassify -= h.ExactOperations
		return h, err
	}
	var parts []SewFace
	var rim []projectiveRoofRim
	var refs [2][]int
	first := [2]int{-1, -1}
	seams := make([][]projectiveRoofEdgeID, len(bands.Sections))
	for i, band := range bands.Bands {
		owner := 0
		if op == parametric.Intersection && band.Sign > 0 || op == parametric.Union && band.Sign < 0 {
			owner = 1
		}
		domain := p.supports[owner].Domain()
		span := [2]geom.ExactParameterInterval{band.Common.U, band.Common.V}[bands.Axis]
		lo, err := classify(bands.Axis, span.Lower, new(big.Rat))
		if err != nil {
			return r, err
		}
		hi, err := classify(bands.Axis, span.Upper, big.NewRat(1, 1))
		if err != nil {
			return r, err
		}
		var intervals [2]geom.ExactParameterInterval
		for axis, x := range lo.OriginalUV[owner] {
			y := hi.OriginalUV[owner][axis]
			if x.Cmp(y) > 0 {
				x, y = y, x
			}
			intervals[axis] = geom.ExactParameterInterval{Lower: x, Upper: y}
		}
		clipped, ancestry, err := projectiveClipGraphFace(p.owners[owner], domain, geom.ExactSurfaceRectangle{U: intervals[0], V: intervals[1]})
		if err != nil {
			return r, err
		}
		original, boundary, err := buildOriginalProjectiveRoofRim(ctx, clipped, p.supports[owner], owner, o.Build)
		if err != nil {
			return r, err
		}
		for _, e := range boundary {
			index := e.use.OriginalUse
			old := ancestry[index]
			if old.OriginalUse >= 0 {
				e.use = old
				rim = append(rim, e)
				continue
			}
			u := &original.Face.Outer[index]
			match := -1
			for _, j := range [2]int{i - 1, i} {
				if j < 0 || j >= len(bySection) {
					continue
				}
				q := bySection[j][owner]
				dir := u.UVStart == q.ends[0] && u.UVEnd == q.ends[1]
				rev := u.UVStart == q.ends[1] && u.UVEnd == q.ends[0]
				if !dir && !rev {
					continue
				}
				if match >= 0 {
					return r, geom.ErrConvergence
				}
				match = j
				model := bands.Sections[j].Whole.Model
				edge := &original.Edges[u.Edge]
				if rev {
					edge.Start, edge.End = edge.End, edge.Start
					q = q.reversed()
				}
				for k, id := range [2]int{edge.Start, edge.End} {
					pos, err := model.ExactPoint(float64(k))
					if err != nil {
						return r, err
					}
					if pos != original.Vertices[id] {
						return r, geom.ErrConvergence
					}
				}
				edge.Curve, u.Curve, u.Reversed = model, q, rev
			}
			if match < 0 {
				return r, geom.ErrConvergence
			}
			seams[match] = append(seams[match], projectiveRoofEdgeID{len(parts), u.Edge})
		}
		if first[owner] < 0 {
			first[owner] = len(parts)
		}
		refs[owner] = append(refs[owner], len(parts))
		parts = append(parts, original)
	}
	if !joinProjectiveRoofRim(rim) {
		return r, geom.ErrConvergence
	}
	var mins, maxs [2]float64
	for i, x := range p.overlap.Projection {
		mins[i], err = geom.RepresentedExactScalar(x.Lower)
		if err != nil {
			return r, err
		}
		maxs[i], err = geom.RepresentedExactScalar(x.Upper)
		if err != nil {
			return r, err
		}
	}
	height := 3 - axes[0] - axes[1]
	parity := 1
	if (axes[0]+1)%3 != axes[1] {
		parity = -1
	}
	var walls []ProjectiveRoofWall
	var bottoms []projectiveRoofBottom
	for _, edge := range rim {
		wall, bottom, err := makeProjectiveRoofWallWithBaseChart(ctx, edge, axes, height, baseHeight, mins, maxs, true, o.Build)
		if err != nil {
			return r, err
		}
		walls = append(walls, ProjectiveRoofWall{Owner: edge.owner, Face: len(parts), Source: edge.use})
		bottoms = append(bottoms, bottom)
		parts = append(parts, wall)
	}
	base, err := makeProjectiveRoofBaseWithChart(ctx, bottoms, axes, height, baseHeight, mins, maxs, parity, true, o.Build)
	if err != nil {
		return r, err
	}
	baseFace := len(parts)
	parts = append(parts, base)
	pairs := make([][2]projectiveRoofEdgeID, len(seams))
	for i, ids := range seams {
		if len(ids) != 2 || ids[0].face != i || ids[1].face != i+1 {
			return r, geom.ErrConvergence
		}
		pairs[i] = [2]projectiveRoofEdgeID{ids[0], ids[1]}
	}
	solid, edges, err := sewProjectiveRoofPairedEdges(ctx, parts, pairs, o.Sew)
	if err != nil {
		return r, err
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	return ProjectiveRoofSolid{Solid: solid, Roofs: first, Walls: walls, RoofFragments: refs,
		BaseFace: baseFace, Section: edges[0], SectionEdges: edges, Complete: true}, nil
}

func joinProjectiveRoofRim(rim []projectiveRoofRim) bool {
	if len(rim) < 4 {
		return false
	}
	for i := 1; i < len(rim); i++ {
		found := -1
		for j := i; j < len(rim); j++ {
			if rim[i-1].ends[1] == rim[j].ends[0] {
				if found >= 0 {
					return false
				}
				found = j
			}
		}
		if found < 0 {
			return false
		}
		rim[i], rim[found] = rim[found], rim[i]
	}
	return rim[len(rim)-1].ends[1] == rim[0].ends[0]
}
