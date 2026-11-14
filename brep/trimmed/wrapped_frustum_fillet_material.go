package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/geom"
)

// FilletedFrustumMaterialPoint reports exact material membership and all native
// boundary face IDs at the point. Outside/inside results have no boundary IDs.
type FilletedFrustumMaterialPoint struct {
	Location geom.BorePointLocation
	Faces    []int
}

// FilletedFrustumMaterial is immutable exact original-support material. It is
// prepared only by re-proving the native fillet and finite-bore construction.
type FilletedFrustumMaterial struct {
	solid   Solid
	recipe  *filletedFrustumMaterialRecipe
	center  [2]*big.Rat
	z, r    [2]*big.Rat
	slope   *big.Rat
	fillets WrappedFrustumDoubleFilletHistory
	cuts    []filletedFrustumMaterialCut
}
type filletedFrustumMaterialCut struct {
	center      [2]*big.Rat
	radius      *big.Rat
	z           [2]*big.Rat
	open        [2]bool
	wall, floor int
}

// PrepareFilletedFrustumMaterial accepts the same bounded 1..16 mixed cutters
// as FilletAndBoreWrappedFrustumSet. It does not accept mutable history as proof.
func PrepareFilletedFrustumMaterial(ctx context.Context, source Solid, lower, upper float64, cuts []FilletedFrustumBoreCut, o geom.SurfacePatchOptions) (FilletedFrustumMaterial, error) {
	result, e := FilletAndBoreWrappedFrustumSet(ctx, source, lower, upper, cuts, o)
	if e != nil {
		return FilletedFrustumMaterial{}, e
	}
	_, p, e := source.ValidateCanonicalWrappedFrustum(ctx, o)
	if e != nil {
		return FilletedFrustumMaterial{}, e
	}
	q := FilletedFrustumMaterial{solid: result.Solid, center: [2]*big.Rat{new(big.Rat).Set(p.CenterX), new(big.Rat).Set(p.CenterY)}, z: [2]*big.Rat{new(big.Rat).Set(p.LowerZ), new(big.Rat).Set(p.UpperZ)}, r: [2]*big.Rat{new(big.Rat).Set(p.LowerRadius), new(big.Rat).Set(p.UpperRadius)}, fillets: result.Fillets}
	q.recipe = &filletedFrustumMaterialRecipe{source: source, lower: lower, upper: upper, cuts: append([]FilletedFrustumBoreCut(nil), cuts...)}
	q.slope = new(big.Rat).Quo(new(big.Rat).Sub(q.r[1], q.r[0]), new(big.Rat).Sub(q.z[1], q.z[0]))
	for i, c := range cuts {
		v := filletedFrustumMaterialCut{center: [2]*big.Rat{new(big.Rat).SetFloat64(c.Center.X), new(big.Rat).SetFloat64(c.Center.Y)}, radius: new(big.Rat).SetFloat64(c.Radius), z: q.z, open: [2]bool{true, true}, wall: result.Cuts[i].BoreFace, floor: result.Cuts[i].FloorFace}
		if c.Depth != 0 {
			if c.FromUpper {
				v.z[0] = new(big.Rat).Set(result.Cuts[i].FloorZ)
				v.open[0] = false
			} else {
				v.z[1] = new(big.Rat).Set(result.Cuts[i].FloorZ)
				v.open[1] = false
			}
		}
		q.cuts = append(q.cuts, v)
	}
	return q, nil
}

// Solid returns the re-proved native solid whose face indices the query reports.
func (q FilletedFrustumMaterial) Solid() Solid { return q.solid }

// ClassifyRational uses exact rational signs, including on the toroidal bands.
// In a band the radius is M+sqrt(r²-(z-Cz)²), with M>r>0.
// Writing t=r²-(z-Cz)² and A=rho²-M²-t: A<0 is strictly
// inside; otherwise sign(A²-4M²t) is the exact radial relation.
// The A sign gate excludes the extraneous branch introduced by squaring.
// All points with input numerator/denominator above 4096 bits fail atomically;
// this fixed-degree expression and at most 16 cutters bound all query work.
// This is not a display, nearest-distance or arbitrary torus certificate.
func (q FilletedFrustumMaterial) ClassifyRational(ctx context.Context, p [3]*big.Rat) (FilletedFrustumMaterialPoint, error) {
	fail := func(e error) (FilletedFrustumMaterialPoint, error) { return FilletedFrustumMaterialPoint{}, e }
	if ctx == nil || q.slope == nil {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	for _, v := range p {
		if v == nil {
			return fail(geom.ErrInvalid)
		}
		if v.Num().BitLen() > 4096 || v.Denom().BitLen() > 4096 {
			return fail(geom.ErrConvergence)
		}
	}
	outside := func() (FilletedFrustumMaterialPoint, error) {
		return FilletedFrustumMaterialPoint{Location: geom.BoreOutside}, nil
	}
	if p[2].Cmp(q.z[0]) < 0 || p[2].Cmp(q.z[1]) > 0 {
		return outside()
	}
	square := func(v *big.Rat) *big.Rat { return new(big.Rat).Mul(v, v) }
	rho := new(big.Rat).Add(square(new(big.Rat).Sub(p[0], q.center[0])), square(new(big.Rat).Sub(p[1], q.center[1])))
	faces := []int{}
	relation := 0
	band := -1
	if p[2].Cmp(q.fillets.Circles[0].Tangency[1]) <= 0 {
		band = 0
	} else if p[2].Cmp(q.fillets.Circles[1].Tangency[1]) >= 0 {
		band = 1
	}
	if band >= 0 {
		c := q.fillets.Circles[band]
		t := new(big.Rat).Sub(square(c.Radius), square(new(big.Rat).Sub(p[2], c.Center[1])))
		if t.Sign() < 0 {
			return fail(geom.ErrConvergence)
		}
		a := new(big.Rat).Sub(new(big.Rat).Sub(rho, square(c.Center[0])), t)
		relation = -1
		if a.Sign() >= 0 {
			rhs := new(big.Rat).Mul(new(big.Rat).SetInt64(4), new(big.Rat).Mul(square(c.Center[0]), t))
			relation = square(a).Cmp(rhs)
		}
		if relation == 0 {
			faces = append(faces, q.fillets.Blends[band])
			if p[2].Cmp(c.Tangency[1]) == 0 {
				faces = append(faces, q.fillets.Lateral)
			}
		}
	} else {
		radius := new(big.Rat).Add(q.r[0], new(big.Rat).Mul(q.slope, new(big.Rat).Sub(p[2], q.z[0])))
		relation = rho.Cmp(square(radius))
		if relation == 0 {
			faces = append(faces, q.fillets.Lateral)
		}
	}
	if relation > 0 {
		return outside()
	}
	for end, z := range q.z {
		if p[2].Cmp(z) == 0 {
			faces = append(faces, q.fillets.Caps[end])
		}
	}
	for _, c := range q.cuts {
		if e := ctx.Err(); e != nil {
			return fail(e)
		}
		lo, hi := p[2].Cmp(c.z[0]), p[2].Cmp(c.z[1])
		if lo < 0 || hi > 0 {
			continue
		}
		d := new(big.Rat).Add(square(new(big.Rat).Sub(p[0], c.center[0])), square(new(big.Rat).Sub(p[1], c.center[1])))
		radial := d.Cmp(square(c.radius))
		if radial > 0 {
			continue
		}
		floor := (lo == 0 && !c.open[0]) || (hi == 0 && !c.open[1])
		if radial < 0 && !floor {
			return outside()
		}
		if radial == 0 {
			faces = append(faces, c.wall)
		}
		if floor {
			faces = append(faces, c.floor)
		}
	}
	if len(faces) == 0 {
		return FilletedFrustumMaterialPoint{Location: geom.BoreInside}, nil
	}
	sort.Ints(faces)
	unique := faces[:0]
	for _, id := range faces {
		if len(unique) == 0 || unique[len(unique)-1] != id {
			unique = append(unique, id)
		}
	}
	return FilletedFrustumMaterialPoint{Location: geom.BoreBoundary, Faces: unique}, nil
}
