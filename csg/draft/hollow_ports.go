package draft

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/csg/poly"
)

// PortHollowOptions selects two to 32 distinct complete supporting faces.
// All openings must be pairwise strictly separated over their entire original
// face polygons. Thickness is zero at ports and positive elsewhere.
type PortHollowOptions struct {
	OpenFaces           []int
	CoordinateThickness bool
	Reconstruction      Options
	Boolean             poly.Options
}

// PortHollowResult retains original port IDs and corresponding cavity faces.
// All returned slices and rational values are caller owned.
type PortHollowResult struct {
	HollowResult
	ExactVolume                     *big.Rat
	OpenFaces, InteriorOpeningFaces []int
}

// HollowPorts constructs a closed multi-port wall by subtracting a convex
// support-offset cavity. Pairwise separation work shares Reconstruction.MaxWork;
// reconstruction and Boolean passes have separate budgets. Conversion must
// preserve every opening plane exactly. No partial result is returned.
func HollowPorts(ctx context.Context, s brep.Solid, thickness []float64, o PortHollowOptions) (PortHollowResult, error) {
	empty := PortHollowResult{}
	if ctx == nil {
		return empty, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	faces := s.Faces()
	if len(faces) == 0 || len(thickness) != len(faces) || len(o.OpenFaces) < 2 {
		return empty, ErrInvalid
	}
	if len(o.OpenFaces) > 32 {
		return empty, ErrLimit
	}
	selected := map[int]bool{}
	for _, id := range o.OpenFaces {
		if id < 0 || id >= len(faces) || selected[id] {
			return empty, ErrInvalid
		}
		selected[id] = true
	}
	ro, e := options(o.Reconstruction)
	if e != nil {
		return empty, e
	}
	if len(faces) > ro.MaxFaces {
		return empty, ErrLimit
	}
	budget := &budget{ctx: ctx, o: ro}
	base, e := Apply(ctx, s, nil, ro)
	if e != nil {
		return empty, e
	}
	open := func(i int) bool { return selected[i] }
	for _, id := range o.OpenFaces {
		out := base.OldToNew[id]
		if out < 0 || len(base.FaceSources[out]) != 1 {
			return empty, ErrUnsupported
		}
	}
	for i, a := range o.OpenFaces {
		for _, b := range o.OpenFaces[i+1:] {
			if e := proveSeparatedHollowOpenings(s, [2]int{a, b}, base, budget); e != nil {
				return empty, e
			}
		}
	}
	transforms := map[int]matrix{}
	for i, t := range thickness {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		if math.IsNaN(t) || math.IsInf(t, 0) || t < 0 || (open(i) && t != 0) || (!open(i) && t == 0) {
			return empty, ErrInvalid
		}
		index := base.OldToNew[i]
		if index < 0 {
			return empty, ErrUnsupported
		}
		h := base.FacePlanes[index]
		n := point(h.Normal)
		d := num(t)
		if t > 0 && !o.CoordinateThickness {
			length, ok := rationalSqrt(dot(n, n))
			if !ok {
				return empty, ErrUnsupported
			}
			d = mul(d, length)
		}
		translation := scale(n, new(big.Rat).Neg(div(d, dot(n, n))))
		var m matrix
		for j := range m {
			for k := range m[j] {
				m[j][k] = new(big.Rat)
			}
			m[j][j].SetInt64(1)
			m[j][3] = translation[j]
		}
		transforms[i] = m
	}
	interior, e := apply(ctx, s, transforms, ro)
	if e != nil {
		return empty, e
	}
	if len(interior.Solid.Faces()) == 0 {
		return empty, ErrInvalid
	}
	// Rounded cavity points must remain within all exact original supports. An
	// opening must remain exactly coincident; otherwise a microscopic cap could
	// change the intended open/closed topology under exact Boolean classification.
	for _, v := range interior.Solid.Vertices() {
		p := exact(v)
		for index, h := range base.FacePlanes {
			if !budget.tick() {
				return empty, budget.err
			}
			comparison := dot(h.Normal, p).Cmp(h.Offset)
			opening := false
			for _, id := range o.OpenFaces {
				if index == base.OldToNew[id] {
					opening = true
					break
				}
			}
			if comparison > 0 || (!opening && comparison == 0) {
				return empty, ErrPrecision
			}
		}
	}
	for _, openFace := range o.OpenFaces {
		mapped := interior.OldToNew[openFace]
		if mapped < 0 {
			return empty, ErrInvalid
		}
		original := base.FacePlanes[base.OldToNew[openFace]]
		vs, es := interior.Solid.Vertices(), interior.Solid.Edges()
		for _, u := range interior.Solid.Faces()[mapped].Boundary {
			if dot(original.Normal, exact(vs[es[u.Edge].Start])).Cmp(original.Offset) != 0 {
				return empty, ErrPrecision
			}
		}
	}
	solid, history, e := poly.CombineWithTopologyHistory(ctx, s, interior.Solid, poly.Difference, o.Boolean)
	if e != nil {
		return empty, e
	}
	volume := new(big.Rat).Sub(base.ExactVolume, interior.ExactVolume)
	if volume.Sign() <= 0 {
		return empty, ErrInvalid
	}
	mapped := make([]int, len(o.OpenFaces))
	for i, id := range o.OpenFaces {
		mapped[i] = interior.OldToNew[id]
	}
	return PortHollowResult{HollowResult: HollowResult{solid, interior, history}, ExactVolume: volume, OpenFaces: append([]int(nil), o.OpenFaces...), InteriorOpeningFaces: mapped}, nil
}
