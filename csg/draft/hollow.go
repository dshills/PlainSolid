package draft

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/csg/poly"
)

// HollowOptions controls support reconstruction and final Boolean separately.
// OpenFace=-1 creates a closed internal cavity. An open face must be a complete
// unique supporting face. CoordinateThickness uses the dominant-component
// normalized support coordinate; false requests Euclidean thickness, requiring
// an exactly rational normal length. Boolean ancestry operand A is the original
// solid, operand B the returned Interior.Solid.
type HollowOptions struct {
	OpenFace            int
	CoordinateThickness bool
	Reconstruction      Options
	Boolean             poly.Options
}

// HollowResult preserves the reconstructed interior and its original-face
// ancestry, as well as the final shell's full Boolean topology ancestry.
type HollowResult struct {
	Solid    brep.Solid
	Interior Result
	History  poly.TopologyHistory
}

// Hollow subtracts independently offset supporting halfspaces from a convex
// planar solid. Thickness aligns with original faces and must be positive,
// except the optional OpenFace which must have thickness zero. The interior
// must remain full-dimensional. Excessive offsets fail instead of returning an
// unchanged solid. Selected open-face coincidence creates a tray or open shell.
// Output is a closed material boundary (including rim walls), not an open mesh.
func Hollow(ctx context.Context, s brep.Solid, thickness []float64, o HollowOptions) (HollowResult, error) {
	empty := HollowResult{}
	if ctx == nil {
		return empty, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	faces := s.Faces()
	if len(faces) == 0 || len(thickness) != len(faces) || o.OpenFace < -1 || o.OpenFace >= len(faces) {
		return empty, ErrInvalid
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
	if o.OpenFace >= 0 {
		out := base.OldToNew[o.OpenFace]
		if out < 0 || len(base.FaceSources[out]) != 1 {
			return empty, ErrUnsupported
		}
	}
	transforms := map[int]matrix{}
	for i, t := range thickness {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		if math.IsNaN(t) || math.IsInf(t, 0) || t < 0 || (i == o.OpenFace && t != 0) || (i != o.OpenFace && t == 0) {
			return empty, ErrInvalid
		}
		index := base.OldToNew[i]
		if index < 0 {
			return empty, ErrUnsupported
		}
		h := base.FacePlanes[index]
		n := point(h.Normal)
		d := num(t)
		if !o.CoordinateThickness {
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
			open := o.OpenFace >= 0 && index == base.OldToNew[o.OpenFace]
			if comparison > 0 || (!open && comparison == 0) {
				return empty, ErrPrecision
			}
		}
	}
	if o.OpenFace >= 0 {
		mapped := interior.OldToNew[o.OpenFace]
		if mapped < 0 {
			return empty, ErrInvalid
		}
		original := base.FacePlanes[base.OldToNew[o.OpenFace]]
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
	return HollowResult{solid, interior, history}, nil
}
