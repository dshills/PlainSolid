package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// BuildOrderedProjectiveSplineRoofSolid closes a Common, Fuse or Cut of two
// same-projection original spline graphs above their shared planar base.
// The height relation must be strict everywhere or exactly coincident.
// Inputs whose common projection or relative height has to be split
// require a different builder. The selected original face charts are
// retained. Empty regularized differences explicitly return Empty.
func BuildOrderedProjectiveSplineRoofSolid(ctx context.Context, a, b Face, axes [2]int, baseHeight float64,
	op parametric.BooleanOp, o ProjectiveRoofSolidOptions) (r ProjectiveRoofSolid, err error) {
	if ctx == nil || !finite(baseHeight) || axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] ||
		o.Sew.RepairOrientation || op != parametric.Intersection && op != parametric.Union && op != parametric.Difference {
		return r, geom.ErrInvalid
	}
	height := 3 - axes[0] - axes[1]
	original := [2]Face{a, b}
	var patches [2]*geom.SurfacePatch
	for owner, f := range original {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		if len(f.Holes) != 0 {
			return r, geom.ErrConvergence
		}
		d, err := projectiveSplineFaceRectangle(f.Outer, false)
		if err != nil {
			return r, err
		}
		raw, pose := parametric.SurfaceMapping(f.Surface)
		p, err := exactSplineSupportPatch(raw, o.Build)
		if err != nil {
			return r, err
		}
		if p.Domain() != d {
			p, err = p.Trim(d)
			if err != nil {
				return r, err
			}
		}
		if pose != geom.Identity() {
			p, err = p.Transformed(pose)
			if err != nil {
				return r, err
			}
		}
		box, err := p.Bounds()
		if err != nil {
			return r, err
		}
		if projectiveVectorAxis(box.Min, height) <= baseHeight {
			return r, geom.ErrConvergence
		}
		patches[owner] = p
	}
	proof, err := geom.CertifyProjectiveSurfaceHeightOrder(ctx, patches[0], patches[1], axes, o.Envelope.Imprint.Contacts.Graph.Cover.Atlas)
	if err != nil {
		return r, err
	}
	parity := 1
	if (axes[0]+1)%3 != axes[1] {
		parity = -1
	}
	for owner, p := range patches {
		d := p.Domain()
		s := proof.Overlap.Areas[owner]
		if s.U.Lower.Cmp(new(big.Rat).SetFloat64(d.UMin)) != 0 || s.U.Upper.Cmp(new(big.Rat).SetFloat64(d.UMax)) != 0 ||
			s.V.Lower.Cmp(new(big.Rat).SetFloat64(d.VMin)) != 0 || s.V.Upper.Cmp(new(big.Rat).SetFloat64(d.VMax)) != 0 {
			return r, geom.ErrConvergence
		}
		sign := proof.Overlap.ProjectionOrientation[owner]
		if original[owner].Reversed {
			sign = -sign
		}
		if sign != parity {
			return r, geom.ErrConvergence
		}
	}
	const absent = -1
	if op == parametric.Difference && proof.Sign <= 0 {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		return ProjectiveRoofSolid{Roofs: [2]int{absent, absent}, BaseFace: absent, Section: absent, Complete: true, Empty: true}, nil
	}
	active := []int{0}
	if op == parametric.Difference {
		active = append(active, 1)
	} else if op == parametric.Intersection && proof.Sign > 0 || op == parametric.Union && proof.Sign < 0 {
		active[0] = 1
	}
	var parts []SewFace
	roofs := [2]int{absent, absent}
	var rims [2][]projectiveRoofRim
	for _, owner := range active {
		f, rim, err := buildOriginalProjectiveRoofRim(ctx, original[owner], patches[owner], owner, o.Build)
		if err != nil {
			return r, err
		}
		roofs[owner], rims[owner] = len(parts), rim
		if op == parametric.Difference && owner == 1 {
			f.Face.Reversed = !f.Face.Reversed
		}
		parts = append(parts, f)
	}
	baseFace := absent
	var walls []ProjectiveRoofWall
	var floor []ProjectiveSplineImprintUse
	if op == parametric.Difference {
		used := [4]bool{}
		for _, top := range rims[0] {
			id := -1
			for i, bottom := range rims[1] {
				if !used[i] && projectiveSameBorderProjection(top, bottom, axes) {
					if id >= 0 {
						return r, geom.ErrConvergence
					}
					id = i
				}
			}
			if id < 0 {
				return r, geom.ErrConvergence
			}
			used[id] = true
			wall, err := makeProjectiveDifferenceWall(ctx, top, rims[1][id], axes, height, o.Build)
			if err != nil {
				return r, err
			}
			walls = append(walls, ProjectiveRoofWall{Owner: 0, Face: len(parts), Source: top.use})
			floor = append(floor, rims[1][id].use)
			parts = append(parts, wall)
		}
	} else {
		var mins, maxs [2]float64
		for i, s := range proof.Overlap.Projection {
			mins[i], err = geom.RepresentedExactScalar(s.Lower)
			if err != nil {
				return r, err
			}
			maxs[i], err = geom.RepresentedExactScalar(s.Upper)
			if err != nil {
				return r, err
			}
		}
		var bottoms []projectiveRoofBottom
		for _, top := range rims[active[0]] {
			wall, bottom, err := makeProjectiveRoofWall(ctx, top, axes, height, baseHeight, mins, maxs, o.Build)
			if err != nil {
				return r, err
			}
			bottoms = append(bottoms, bottom)
			walls = append(walls, ProjectiveRoofWall{Owner: active[0], Face: len(parts), Source: top.use})
			parts = append(parts, wall)
		}
		base, err := makeProjectiveRoofBase(ctx, bottoms, axes, height, baseHeight, mins, maxs, parity, o.Build)
		if err != nil {
			return r, err
		}
		baseFace = len(parts)
		parts = append(parts, base)
	}
	solid, section, err := sewProjectiveRoofParts(ctx, parts, [2]int{absent, absent}, o.Sew)
	if err != nil {
		return r, err
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	return ProjectiveRoofSolid{Solid: solid, Roofs: roofs, Walls: walls, FloorSources: floor, BaseFace: baseFace, Section: section, Complete: true}, nil
}

func buildOriginalProjectiveRoofRim(ctx context.Context, f Face, patch *geom.SurfacePatch, owner int,
	o geom.SurfacePatchOptions) (out SewFace, rim []projectiveRoofRim, err error) {
	if o.MaxOperations == 0 {
		o.MaxOperations = 4_000_000
	}
	result := SewFace{Face: Face{Surface: f.Surface, Reversed: f.Reversed}}
	stations := [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}
	var oriented []projectiveRoofRim
	for i, u := range f.Outer {
		curve, _, err := composeProjectiveImprintSide(ctx, patch, u.UVStart, u.UVEnd, o)
		if err != nil {
			return out, nil, err
		}
		p0, err := curve.ExactPoint(0)
		if err != nil {
			return out, nil, err
		}
		result.Vertices = append(result.Vertices, p0)
		result.Edges = append(result.Edges, parametric.Edge{Start: i, End: (i + 1) % 4, Curve: curve, Range: parametric.Interval{Max: 1}})
		result.Face.Outer = append(result.Face.Outer, Use{Edge: i, UVStart: u.UVStart, UVEnd: u.UVEnd})
		net, _, err := curve.BezierControlNet(ctx, o)
		if err != nil {
			return out, nil, err
		}
		if len(net.Control) < 2 || len(net.Control) > 3 || net.Start != 0 || net.End != 1 {
			return out, nil, geom.ErrConvergence
		}
		if f.Reversed {
			for a, b := 0, len(net.Control)-1; a < b; a, b = a+1, b-1 {
				net.Control[a], net.Control[b] = net.Control[b], net.Control[a]
			}
			curve, err = geom.NewExactHomogeneousBezier(ctx, net.Control, o)
			if err != nil {
				return out, nil, err
			}
		}
		p1, err := curve.ExactPoint(1)
		if err != nil {
			return out, nil, err
		}
		if f.Reversed {
			p0, err = curve.ExactPoint(0)
			if err != nil {
				return out, nil, err
			}
		}
		oriented = append(oriented, projectiveRoofRim{model: curve, ends: [2]geom.Vec3{p0, p1}, owner: owner,
			use: ProjectiveSplineImprintUse{OriginalUse: i, OriginalEdge: u.Edge, OriginalRange: [2]*big.Rat{
				new(big.Rat).Set(stations[0]), new(big.Rat).Set(stations[1]),
			}}})
	}
	if f.Reversed {
		for a, b := 0, len(oriented)-1; a < b; a, b = a+1, b-1 {
			oriented[a], oriented[b] = oriented[b], oriented[a]
		}
	}
	return result, oriented, nil
}

func projectiveSameBorderProjection(a, b projectiveRoofRim, axes [2]int) bool {
	for i := range a.ends {
		for _, axis := range axes {
			if projectiveVectorAxis(a.ends[i], axis) != projectiveVectorAxis(b.ends[i], axis) {
				return false
			}
		}
	}
	return true
}
