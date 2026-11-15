package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/intersect"
)

type CoincidentConvexOverlapOptions struct {
	Coincidence                          intersect.SplineCoincidenceOptions
	Injectivity                          geom.SurfaceInjectivityOptions
	Trims                                trimvalidate.Options
	MaxUses, MaxWork, MaxCoefficientBits int
}

// CoincidentConvexOverlap retains the complete closed chart intersection.
// Dimension is -1(empty),0(point),1(segment),2(area). Only Dimension2 has
// nonempty regularized 2D material. Boundary is a CCW A-chart polygon for
// area, one directed contact segment for dimension1, otherwise empty. Native
// curve descriptors and original coedge ranges carry no rounded cut identity.
type CoincidentConvexOverlap struct {
	Faces           [2]int
	Map             intersect.SplineChartMap
	Injectivity     geom.SurfaceInjectivity
	Vertices        []CoincidentOverlapPoint
	Boundary        []CoincidentChartEdge
	UVArea          [2]*big.Rat
	Dimension, Work int
	Complete        bool
}

// IntersectCoincidentConvexTrims accepts strictly convex straight outer loops
// without holes on an independently proved injective shared support. All
// original halfplanes are checked, including after dimensional collapse.
// MaxUses(default64,max128) counts both loops. MaxWork(default100000,max1M)
// counts checked arrangement rationals; MaxCoefficientBits(default16384,max65536)
// bounds each. Support/trim/native-curve proof limits apply separately.
func (a Solid) IntersectCoincidentConvexTrims(ctx context.Context, faceA int, b Solid, faceB int, o CoincidentConvexOverlapOptions) (CoincidentConvexOverlap, error) {
	fail := func(e error) (CoincidentConvexOverlap, error) { return CoincidentConvexOverlap{}, e }
	if ctx == nil || faceA < 0 || faceA >= len(a.faces) || faceB < 0 || faceB >= len(b.faces) {
		return fail(geom.ErrInvalid)
	}
	if o.MaxUses == 0 {
		o.MaxUses = 64
	}
	if o.MaxWork == 0 {
		o.MaxWork = 100000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxUses < 6 || o.MaxUses > 128 || o.MaxWork < 1 || o.MaxWork > 1000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 65536 {
		return fail(geom.ErrInvalid)
	}
	work := 0
	check := func(x *big.Rat) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		work++
		if work > o.MaxWork || x.Num().BitLen() > o.MaxCoefficientBits || x.Denom().BitLen() > o.MaxCoefficientBits {
			return parametric.ErrBudget
		}
		return nil
	}
	faces := [2]Face{a.faces[faceA], b.faces[faceB]}
	ids := [2]int{faceA, faceB}
	if len(faces[0].Outer)+len(faces[1].Outer) > o.MaxUses {
		return fail(parametric.ErrBudget)
	}
	var patches [2]*geom.SurfacePatch
	for i, f := range faces {
		if len(f.Holes) != 0 || len(f.Outer) < 3 {
			return fail(geom.ErrConvergence)
		}
		proof, e := ValidateFaceTrimLoops(ctx, f, o.Trims)
		if e != nil {
			return fail(e)
		}
		if proof.Status != trimvalidate.Certified {
			return fail(geom.ErrConvergence)
		}
		for j, u := range f.Outer {
			if u.Curve != nil {
				return fail(geom.ErrConvergence)
			}
			next := f.Outer[(j+1)%len(f.Outer)].UVEnd
			cross := new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).Sub(ratInset(u.UVEnd.X), ratInset(u.UVStart.X)), new(big.Rat).Sub(ratInset(next.Y), ratInset(u.UVEnd.Y))), new(big.Rat).Mul(new(big.Rat).Sub(ratInset(u.UVEnd.Y), ratInset(u.UVStart.Y)), new(big.Rat).Sub(ratInset(next.X), ratInset(u.UVEnd.X))))
			if e := check(cross); e != nil {
				return fail(e)
			}
			if cross.Sign() <= 0 {
				return fail(geom.ErrConvergence)
			}
		}
		p, e := exactFaceSupportPatch(f, o.Coincidence.Patches)
		if e != nil {
			return fail(e)
		}
		patches[i] = p
		d := p.Domain()
		bounds, e := faceUVBounds(f)
		if e != nil {
			return fail(e)
		}
		if bounds.UMin < d.UMin || bounds.UMax > d.UMax || bounds.VMin < d.VMin || bounds.VMax > d.VMax {
			return fail(geom.ErrDomain)
		}
	}
	same, e := intersect.FindExactSplineCoincidence(ctx, patches[0], patches[1], o.Coincidence)
	if e != nil {
		return fail(e)
	}
	if !same.Found {
		return fail(geom.ErrConvergence)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	injective, e := geom.CertifySurfaceInjectivity(patches[0], o.Injectivity)
	if e != nil {
		return fail(e)
	}
	if !injective.ProvenInjective {
		return fail(geom.ErrConvergence)
	}
	matrix := splineCoincidenceUVMatrix(same.Map)
	polygon := make(lensUVPolygon, len(faces[0].Outer))
	for i, u := range faces[0].Outer {
		polygon[i] = [2]*big.Rat{ratInset(u.UVStart.X), ratInset(u.UVStart.Y)}
	}
	equal := func(a, b [2]*big.Rat) bool { return a[0].Cmp(b[0]) == 0 && a[1].Cmp(b[1]) == 0 }
	for _, u := range faces[1].Outer {
		x, y := ratInset(u.UVStart.X), ratInset(u.UVStart.Y)
		dx, dy := new(big.Rat).Sub(ratInset(u.UVEnd.X), x), new(big.Rat).Sub(ratInset(u.UVEnd.Y), y)
		var row [3]*big.Rat
		for j, col := range []int{0, 1, 3} {
			row[j] = new(big.Rat).Sub(new(big.Rat).Mul(dx, matrix[1][col]), new(big.Rat).Mul(dy, matrix[0][col]))
		}
		row[2].Add(row[2], new(big.Rat).Sub(new(big.Rat).Mul(dy, x), new(big.Rat).Mul(dx, y)))
		for _, q := range row {
			if e := check(q); e != nil {
				return fail(e)
			}
		}
		polygon, e = polygon.clip(row, check)
		if e != nil {
			return fail(e)
		}
		clean := polygon[:0]
		for _, p := range polygon {
			if len(clean) == 0 || !equal(clean[len(clean)-1], p) {
				clean = append(clean, p)
			}
		}
		if len(clean) > 1 && equal(clean[0], clean[len(clean)-1]) {
			clean = clean[:len(clean)-1]
		}
		polygon = clean
		// Do not stop on point/segment: later planes can remove that contact.
	}
	r := CoincidentConvexOverlap{Faces: ids, Map: same.Map, Injectivity: injective, UVArea: [2]*big.Rat{new(big.Rat), new(big.Rat)}, Dimension: -1}
	if len(polygon) == 0 {
		r.Complete = true
		r.Work = work
		return r, nil
	}
	area, e := polygon.area(check)
	if e != nil {
		return fail(e)
	}
	r.UVArea[0] = area
	determinant := new(big.Rat).Sub(new(big.Rat).Mul(matrix[0][0], matrix[1][1]), new(big.Rat).Mul(matrix[0][1], matrix[1][0]))
	determinant.Abs(determinant)
	r.UVArea[1].Mul(area, determinant)
	if e := check(r.UVArea[1]); e != nil {
		return fail(e)
	}
	if area.Sign() > 0 {
		r.Dimension = 2
	} else {
		lo, hi := polygon[0], polygon[0]
		less := func(a, b [2]*big.Rat) bool { return a[0].Cmp(b[0]) < 0 || a[0].Cmp(b[0]) == 0 && a[1].Cmp(b[1]) < 0 }
		for _, p := range polygon {
			if less(p, lo) {
				lo = p
			}
			if less(hi, p) {
				hi = p
			}
		}
		polygon = lensUVPolygon{lo}
		r.Dimension = 0
		if !equal(lo, hi) {
			polygon = append(polygon, hi)
			r.Dimension = 1
		}
	}
	// Exact line membership supplies original owner ranges, never proximity.
	owners := func(p [2]*big.Rat) ([]CoincidentChartUse, error) {
		var out []CoincidentChartUse
		for source, f := range faces {
			point := p
			if source == 1 {
				point = coincidentMapPoint(matrix, p)
			}
			for i, u := range f.Outer {
				start, end := [2]*big.Rat{ratInset(u.UVStart.X), ratInset(u.UVStart.Y)}, [2]*big.Rat{ratInset(u.UVEnd.X), ratInset(u.UVEnd.Y)}
				axis := 0
				if start[0].Cmp(end[0]) == 0 {
					axis = 1
				}
				t := new(big.Rat).Quo(new(big.Rat).Sub(point[axis], start[axis]), new(big.Rat).Sub(end[axis], start[axis]))
				if e := check(t); e != nil {
					return nil, e
				}
				if t.Sign() < 0 || t.Cmp(big.NewRat(1, 1)) > 0 {
					continue
				}
				on := true
				for k := 0; k < 2; k++ {
					v := new(big.Rat).Add(start[k], new(big.Rat).Mul(t, new(big.Rat).Sub(end[k], start[k])))
					if e := check(v); e != nil {
						return nil, e
					}
					on = on && v.Cmp(point[k]) == 0
				}
				if on {
					out = append(out, CoincidentChartUse{CoincidentTrimOwner{source, ids[source], 0, i, u.Edge}, [2]*big.Rat{new(big.Rat).Set(t), new(big.Rat).Set(t)}})
				}
			}
		}
		return out, nil
	}
	for _, p := range polygon {
		source, e := owners(p)
		if e != nil {
			return fail(e)
		}
		v := CoincidentOverlapPoint{A: coincidentCopyPoint(p), B: coincidentMapPoint(matrix, p)}
		for _, x := range source {
			v.Owners = append(v.Owners, x.Owner)
		}
		r.Vertices = append(r.Vertices, v)
	}
	edges := len(polygon)
	if r.Dimension == 1 {
		edges = 1
	}
	if r.Dimension == 0 {
		edges = 0
	}
	for i := 0; i < edges; i++ {
		p, q := polygon[i], polygon[(i+1)%len(polygon)]
		start, e := owners(p)
		if e != nil {
			return fail(e)
		}
		end, e := owners(q)
		if e != nil {
			return fail(e)
		}
		edge := CoincidentChartEdge{A: [2][2]*big.Rat{coincidentCopyPoint(p), coincidentCopyPoint(q)}, B: [2][2]*big.Rat{coincidentMapPoint(matrix, p), coincidentMapPoint(matrix, q)}}
		for _, x := range start {
			for _, y := range end {
				if x.Owner == y.Owner {
					edge.Sources = append(edge.Sources, CoincidentChartUse{x.Owner, [2]*big.Rat{x.Range[0], y.Range[0]}})
				}
			}
		}
		if len(edge.Sources) == 0 {
			return fail(geom.ErrConvergence)
		}
		for side, points := range [][2][2]*big.Rat{edge.A, edge.B} {
			h := make([][4]*big.Rat, 2)
			for k, p := range points {
				h[k] = [4]*big.Rat{p[0], p[1], new(big.Rat), big.NewRat(1, 1)}
			}
			curve, e := geom.NewExactHomogeneousBezier(ctx, h, o.Coincidence.Patches)
			if e != nil {
				return fail(e)
			}
			if side == 0 {
				edge.CurveA = curve
			} else {
				edge.CurveB = curve
			}
		}
		r.Boundary = append(r.Boundary, edge)
	}
	r.Complete = true
	r.Work = work
	return r, nil
}
