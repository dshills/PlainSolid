//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// BuildProjectiveSplineRoofAdjacentFuse joins two original stocks whose
// projected rectangles share one entire side. The two roof lifts must
// coincide along the complete analytic edge, with its exact original
// projective clocks. The shared wall cancels and the planar base closes
// the union of both footprints.
func BuildProjectiveSplineRoofAdjacentFuse(ctx context.Context, a, b Face, axes [2]int, baseHeight float64,
	o ProjectiveRoofSolidOptions) (r ProjectiveRoofSolid, err error) {
	p, err := proveProjectiveRoofFootprints(ctx, a, b, axes, baseHeight, o)
	if err != nil {
		return r, err
	}
	return buildProjectiveRoofAdjacentFuse(ctx, p, axes, baseHeight, o)
}

func buildProjectiveRoofAdjacentFuse(ctx context.Context, p projectiveRoofFootprintProof, axes [2]int, baseHeight float64,
	o ProjectiveRoofSolidOptions) (r ProjectiveRoofSolid, err error) {
	if !p.overlap.NoArea {
		return r, geom.ErrDomain
	}
	source := o.Envelope.Imprint.Contacts.Patches
	edge, err := geom.CertifyProjectiveGraphBoundaryContact(ctx, p.supports[0], p.supports[1], axes,
		geom.SurfaceAffineCurveOptions{TubeAffineLineOptions: geom.TubeAffineLineOptions{
			MaxPoles: source.MaxPoles, MaxOperations: source.MaxOperations, MaxCoefficientBits: source.MaxCoefficientBits,
		}})
	if err != nil {
		return r, err
	}
	if !edge.Complete || !edge.Edge || len(edge.Owners[0].Pieces) != 1 || len(edge.Owners[1].Pieces) != 1 {
		return r, geom.ErrConvergence
	}
	model := edge.Owners[0].Pieces[0].Curve
	var parts []SewFace
	var external []projectiveRoofRim
	var seams [2]int
	one := big.NewRat(1, 1)
	for owner := range 2 {
		f, rims, err := buildOriginalProjectiveRoofRim(ctx, p.owners[owner], p.supports[owner], owner, o.Build)
		if err != nil {
			return r, err
		}
		ratio := one
		if owner == 1 {
			ratio = edge.BClockRatio
		}
		from, to := edge.Endpoints[0].OriginalUV[owner], edge.Endpoints[1].OriginalUV[owner]
		h := [3][3]*big.Rat{
			{from[0], from[1], one},
			{},
			{new(big.Rat).Mul(to[0], ratio), new(big.Rat).Mul(to[1], ratio), ratio},
		}
		for i := range 3 {
			h[1][i] = new(big.Rat).Quo(new(big.Rat).Add(h[0][i], h[2][i]), big.NewRat(2, 1))
		}
		q, err := NewExactQuadraticUV(h)
		if err != nil {
			return r, err
		}
		found := -1
		for _, side := range rims {
			u := &f.Face.Outer[side.use.OriginalUse]
			forward := u.UVStart == q.ends[0] && u.UVEnd == q.ends[1]
			reverse := u.UVStart == q.ends[1] && u.UVEnd == q.ends[0]
			if !forward && !reverse {
				external = append(external, side)
				continue
			}
			if found >= 0 {
				return r, geom.ErrConvergence
			}
			found = u.Edge
			e := &f.Edges[u.Edge]
			local := q
			if reverse {
				e.Start, e.End = e.End, e.Start
				local = local.reversed()
			}
			for i, vertex := range [2]int{e.Start, e.End} {
				x, err := model.ExactPoint(float64(i))
				if err != nil {
					return r, err
				}
				if x != f.Vertices[vertex] {
					return r, geom.ErrConvergence
				}
			}
			e.Curve, u.Curve, u.Reversed = model, local, reverse
		}
		if found < 0 {
			return r, geom.ErrConvergence // Partial coedges do not prove a full wall join.
		}
		seams[owner] = found
		parts = append(parts, f)
	}
	if len(external) != 6 || !joinProjectiveRoofRim(external) {
		return r, geom.ErrConvergence
	}
	var mins, maxs [2]float64
	for i, axis := range axes {
		mins[i] = projectiveVectorAxis(external[0].ends[0], axis)
		maxs[i] = mins[i]
		for _, side := range external {
			for _, v := range side.ends {
				x := projectiveVectorAxis(v, axis)
				mins[i], maxs[i] = min(mins[i], x), max(maxs[i], x)
			}
		}
	}
	height := 3 - axes[0] - axes[1]
	parity := 1
	if (axes[0]+1)%3 != axes[1] {
		parity = -1
	}
	var walls []ProjectiveRoofWall
	var bottoms []projectiveRoofBottom
	for _, side := range external {
		wall, bottom, err := makeProjectiveRoofWallWithBaseChart(ctx, side, axes, height, baseHeight, mins, maxs, true, o.Build)
		if err != nil {
			return r, err
		}
		walls = append(walls, ProjectiveRoofWall{Owner: side.owner, Face: len(parts), Source: side.use})
		bottoms = append(bottoms, bottom)
		parts = append(parts, wall)
	}
	base, err := makeProjectiveRoofBaseWithChart(ctx, bottoms, axes, height, baseHeight, mins, maxs, parity, true, o.Build)
	if err != nil {
		return r, err
	}
	baseID := len(parts)
	parts = append(parts, base)
	solid, seam, err := sewProjectiveRoofParts(ctx, parts, seams, o.Sew)
	if err != nil {
		return r, err
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	return ProjectiveRoofSolid{Solid: solid, Roofs: [2]int{0, 1}, Walls: walls, BaseFace: baseID,
		Section: seam, SectionEdges: []int{seam}, Complete: true}, nil
}
