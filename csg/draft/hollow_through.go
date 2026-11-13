package draft

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/csg/poly"
)

// ThroughHollowOptions selects two complete unique antiparallel supporting
// faces as openings. OpenFaces refers to original face indices; thickness is
// zero at those faces and strictly positive elsewhere. CoordinateThickness
// and independent reconstruction/Boolean budgets have HollowOptions semantics.
type ThroughHollowOptions struct {
	OpenFaces           [2]int
	CoordinateThickness bool
	Reconstruction      Options
	Boolean             poly.Options
}

// ThroughHollowResult adds ideal exact material volume and explicit original
// opening-to-interior face identities. ExactVolume is independent of the
// checked rounded display; its rational is caller-owned.
type ThroughHollowResult struct {
	HollowResult
	ExactVolume          *big.Rat
	OpenFaces            [2]int
	InteriorOpeningFaces [2]int
}

// HollowThrough subtracts an exact support-offset cavity joining two opposite
// parallel end faces of a convex planar solid. All other supports remain
// strictly outside the cavity, including after display conversion. Both end
// openings must survive with exact original-plane coincidence. The returned
// material has closed walls/rims; this is a through duct, not an open mesh.
// Each reconstruction, support check and Boolean pass has its own configured
// budget. No partial result is returned on failure. Input arrays remain owned
// by the caller. Arbitrary nonparallel openings are unsupported.
func HollowThrough(ctx context.Context, s brep.Solid, thickness []float64, o ThroughHollowOptions) (ThroughHollowResult, error) {
	empty := ThroughHollowResult{}
	if ctx == nil {
		return empty, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	faces := s.Faces()
	if len(faces) == 0 || len(thickness) != len(faces) || o.OpenFaces[0] < 0 || o.OpenFaces[1] < 0 || o.OpenFaces[0] >= len(faces) || o.OpenFaces[1] >= len(faces) || o.OpenFaces[0] == o.OpenFaces[1] {
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
	open := func(i int) bool { return i == o.OpenFaces[0] || i == o.OpenFaces[1] }
	for _, id := range o.OpenFaces {
		out := base.OldToNew[id]
		if out < 0 || len(base.FaceSources[out]) != 1 {
			return empty, ErrUnsupported
		}
	}
	n0, n1 := base.FacePlanes[base.OldToNew[o.OpenFaces[0]]].Normal, base.FacePlanes[base.OldToNew[o.OpenFaces[1]]].Normal
	c := cross(n0, n1)
	if c[0].Sign() != 0 || c[1].Sign() != 0 || c[2].Sign() != 0 || dot(n0, n1).Sign() >= 0 {
		return empty, ErrUnsupported
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
			opening := index == base.OldToNew[o.OpenFaces[0]] || index == base.OldToNew[o.OpenFaces[1]]
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
	return ThroughHollowResult{HollowResult: HollowResult{solid, interior, history}, ExactVolume: volume, OpenFaces: o.OpenFaces, InteriorOpeningFaces: [2]int{interior.OldToNew[o.OpenFaces[0]], interior.OldToNew[o.OpenFaces[1]]}}, nil
}
