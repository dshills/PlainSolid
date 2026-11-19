//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// ProjectiveRoofBandDifference contains one closed body per strictly
// positive original A-minus-B band, in the first chart's station order.
type ProjectiveRoofBandDifference struct {
	Components []ProjectiveRoofSolid
	Bands      []int
	Complete   bool
}

// BuildProjectiveSplineRoofBandDifference subtracts two original graphs
// with the same positive-base footprint and at least two full parallel
// sections. A bounded interior positive band is split once at its
// exact midpoint, so its walls meet at a positive-height station.
func BuildProjectiveSplineRoofBandDifference(ctx context.Context, a, b Face, axes [2]int, baseHeight float64,
	o ProjectiveRoofSolidOptions) (r ProjectiveRoofBandDifference, err error) {
	if o.Envelope.MaxClassifyOperations < 0 || o.Envelope.Imprint.MaxEdgeOperations < 0 {
		return r, geom.ErrInvalid
	}
	p, err := proveProjectiveRoofFootprints(ctx, a, b, axes, baseHeight, o)
	if err != nil {
		return r, err
	}
	if !p.covers(0) || !p.covers(1) {
		return r, geom.ErrConvergence
	}
	g, err := TraceProjectiveSplineFaceContacts(ctx, a, b, axes, o.Envelope.Imprint.Contacts)
	if err != nil {
		return r, err
	}
	if len(g.Graph.Contours) < 2 || len(g.Graph.Contours) > 16 {
		return r, geom.ErrConvergence
	}
	composition := o.Build
	if n := o.Envelope.Imprint.MaxEdgeOperations; n > 0 && (composition.MaxOperations == 0 || n < composition.MaxOperations) {
		composition.MaxOperations = n
	}
	bands, err := g.Graph.BuildUnsplitParallelContactBands(ctx, composition)
	if err != nil {
		return r, err
	}
	maxClassify := o.Envelope.MaxClassifyOperations
	if maxClassify == 0 {
		maxClassify = 2_000_000
	}
	work := projectiveBandDifferenceWork{ctx: ctx, footprints: p, graph: g.Graph, bands: bands, axes: axes, o: o, maxClassify: maxClassify}
	var solids []ProjectiveRoofSolid
	var indices []int
	for i, band := range bands.Bands {
		if band.Sign < 0 {
			continue
		}
		q, err := work.build(i)
		if err != nil {
			return r, err
		}
		solids, indices = append(solids, q), append(indices, i)
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	return ProjectiveRoofBandDifference{Components: solids, Bands: indices, Complete: true}, nil
}

type projectiveBandDifferenceWork struct {
	ctx         context.Context
	footprints  projectiveRoofFootprintProof
	graph       geom.ProjectiveSurfaceContactGraph
	bands       geom.ProjectiveContactBands
	axes        [2]int
	o           ProjectiveRoofSolidOptions
	maxClassify int
}

func (w *projectiveBandDifferenceWork) corner(station *big.Rat, upper bool) (q geom.ProjectiveContactHeight, err error) {
	uv := [2]*big.Rat{new(big.Rat), new(big.Rat)}
	uv[w.bands.Axis] = station
	if upper {
		uv[1-w.bands.Axis].SetInt64(1)
	}
	if w.maxClassify < 1 {
		return q, geom.ErrConvergence
	}
	q, err = w.graph.ClassifyCommonHeightRational(w.ctx, uv,
		geom.SurfacePatchEvaluationOptions{MaxOperations: w.maxClassify, MaxCoefficientBits: w.o.Build.MaxCoefficientBits})
	w.maxClassify -= q.ExactOperations
	return q, err
}

func (w *projectiveBandDifferenceWork) build(band int) (r ProjectiveRoofSolid, err error) {
	d := w.bands.Bands[band].Common
	span := [2]geom.ExactParameterInterval{d.U, d.V}[w.bands.Axis]
	left, right := band > 0, band < len(w.bands.Sections)
	stops := []*big.Rat{span.Lower, span.Upper}
	if left && right {
		m := new(big.Rat).Quo(new(big.Rat).Add(span.Lower, span.Upper), big.NewRat(2, 1))
		stops = []*big.Rat{span.Lower, m, span.Upper}
	}
	var parts []SewFace
	var walls []ProjectiveRoofWall
	var floors []ProjectiveSplineImprintUse
	var roofFaces [2][]int
	var sourcePairs, splitPairs [][2]projectiveRoofEdgeID
	var mid [2][]projectiveRoofEdgeID
	for chunk := 0; chunk+1 < len(stops); chunk++ {
		lo, err := w.corner(stops[chunk], false)
		if err != nil {
			return r, err
		}
		hi, err := w.corner(stops[chunk+1], true)
		if err != nil {
			return r, err
		}
		section := [2]int{-1, -1}
		split := [2]bool{chunk > 0, chunk+2 < len(stops)}
		if chunk == 0 && left {
			section[0] = band - 1
		}
		if chunk+2 == len(stops) && right {
			section[1] = band
		}
		var sourceRefs [2][2]projectiveRoofEdgeID
		var outside [2][]projectiveRoofRim
		for owner := range 2 {
			face, edges, err := w.roofChunk(owner, lo, hi, section, split)
			if err != nil {
				return r, err
			}
			if owner == 1 {
				face.Face.Reversed = !face.Face.Reversed
			}
			fid := len(parts)
			parts = append(parts, face)
			roofFaces[owner] = append(roofFaces[owner], fid)
			outside[owner] = edges.rim
			for side, edge := range edges.sides {
				if section[side] >= 0 {
					sourceRefs[side][owner] = projectiveRoofEdgeID{fid, edge}
				} else if split[side] {
					mid[owner] = append(mid[owner], projectiveRoofEdgeID{fid, edge})
				}
			}
		}
		for side, s := range section {
			if s >= 0 {
				sourcePairs = append(sourcePairs, sourceRefs[side])
			}
		}
		if len(outside[0]) != len(outside[1]) {
			return r, geom.ErrConvergence
		}
		used := make([]bool, len(outside[1]))
		for _, top := range outside[0] {
			j := -1
			for i, bot := range outside[1] {
				if !used[i] && projectiveSameBorderProjection(top, bot, w.axes) {
					if j >= 0 {
						return r, geom.ErrConvergence
					}
					j = i
				}
			}
			if j < 0 {
				return r, geom.ErrConvergence
			}
			used[j] = true
			floor := outside[1][j]
			wall, err := makeProjectiveDifferenceWall(w.ctx, top, floor, w.axes, 3-w.axes[0]-w.axes[1], w.o.Build)
			if err != nil {
				return r, err
			}
			walls = append(walls, ProjectiveRoofWall{Owner: 0, Face: len(parts), Source: top.use})
			floors = append(floors, floor.use)
			parts = append(parts, wall)
		}
	}
	if len(stops) == 3 {
		for _, ids := range mid {
			if len(ids) != 2 {
				return r, geom.ErrConvergence
			}
			splitPairs = append(splitPairs, [2]projectiveRoofEdgeID{ids[0], ids[1]})
		}
	}
	s, ids, err := sewProjectiveRoofPairedEdges(w.ctx, parts, append(sourcePairs, splitPairs...), w.o.Sew)
	if err != nil {
		return r, err
	}
	if len(sourcePairs) == 0 {
		return r, geom.ErrConvergence
	}
	return ProjectiveRoofSolid{Solid: s, BaseFace: -1, Section: ids[0], SectionEdges: ids[:len(sourcePairs)],
		Roofs: [2]int{roofFaces[0][0], roofFaces[1][0]}, RoofFragments: roofFaces, Walls: walls,
		FloorSources: floors, Complete: true}, nil
}

type projectiveBandRoofBoundary struct {
	rim   []projectiveRoofRim
	sides [2]int
}

func (w *projectiveBandDifferenceWork) roofChunk(owner int, lo, hi geom.ProjectiveContactHeight,
	section [2]int, split [2]bool) (out SewFace, boundary projectiveBandRoofBoundary, err error) {
	ownerAxis := w.bands.Axis
	if owner == 1 && w.footprints.overlap.SwapB {
		ownerAxis = 1 - ownerAxis
	}
	var box [2]geom.ExactParameterInterval
	for i, x := range lo.OriginalUV[owner] {
		y := hi.OriginalUV[owner][i]
		if x.Cmp(y) > 0 {
			x, y = y, x
		}
		box[i] = geom.ExactParameterInterval{Lower: x, Upper: y}
	}
	f, ledger, err := projectiveClipGraphFace(w.footprints.owners[owner], w.footprints.supports[owner].Domain(),
		geom.ExactSurfaceRectangle{U: box[0], V: box[1]})
	if err != nil {
		return out, boundary, err
	}
	fixedLo, err := geom.RepresentedExactScalar(lo.OriginalUV[owner][ownerAxis])
	if err != nil {
		return out, boundary, err
	}
	fixedHi, err := geom.RepresentedExactScalar(hi.OriginalUV[owner][ownerAxis])
	if err != nil {
		return out, boundary, err
	}
	native, rim, err := buildOriginalProjectiveRoofRim(w.ctx, f, w.footprints.supports[owner], owner, w.o.Build)
	if err != nil {
		return out, boundary, err
	}
	boundary.sides = [2]int{-1, -1}
	for _, e := range rim {
		idx := e.use.OriginalUse
		u := &native.Face.Outer[idx]
		first, last := [2]float64{u.UVStart.X, u.UVStart.Y}[ownerAxis], [2]float64{u.UVEnd.X, u.UVEnd.Y}[ownerAxis]
		side := -1
		if first == last && first == fixedLo {
			side = 0
		} else if first == last && first == fixedHi {
			side = 1
		}
		if side < 0 || section[side] < 0 && !split[side] {
			e.use = ledger[idx]
			if e.use.OriginalUse < 0 {
				return out, boundary, geom.ErrConvergence
			}
			boundary.rim = append(boundary.rim, e)
			continue
		}
		if boundary.sides[side] >= 0 {
			return out, boundary, geom.ErrConvergence
		}
		boundary.sides[side] = u.Edge
		if section[side] >= 0 {
			s := w.bands.Sections[section[side]].Whole
			q, _, err := projectiveSectionUV(w.ctx, s.UV[owner], w.o.Build)
			if err != nil {
				return out, boundary, err
			}
			forward, reverse := u.UVStart == q.ends[0] && u.UVEnd == q.ends[1], u.UVStart == q.ends[1] && u.UVEnd == q.ends[0]
			if !forward && !reverse {
				return out, boundary, geom.ErrConvergence
			}
			edge := &native.Edges[u.Edge]
			if reverse {
				edge.Start, edge.End = edge.End, edge.Start
				q = q.reversed()
			}
			u.Curve, u.Reversed, edge.Curve = q, reverse, s.Model
		}
	}
	for i, s := range section {
		if (s >= 0 || split[i]) && boundary.sides[i] < 0 {
			return out, boundary, geom.ErrConvergence
		}
	}
	return native, boundary, nil
}
