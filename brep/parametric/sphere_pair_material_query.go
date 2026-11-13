package parametric

import (
	"context"
	"math"
	"plainsolid/geom"
)

// SpherePairSet binds exact represented ball material to a checked native
// spherical boundary. Exact queries refer to the original balls; the native
// seam/latitude approximation has CombineSpheres' independent sewing tolerance.
// Only coordinate-aligned center displacement and exact signed-permutation
// support frames are accepted here. Arbitrary reconstructed solids are not
// implicitly bound. The source recipe and face-owner lists are immutable.
type SpherePairSet struct {
	solid Solid
	query geom.SpherePairMaterial
	faces [2][]int
	valid bool
}

// NewSpherePairSet constructs and checks a native ball Boolean before exposing
// exact point/line/first-hit queries. Native tangent unions/cavities and unresolved
// near contacts remain rejected even though the geometric query can describe
// their singular sets. No partial binding is returned on failure.
func NewSpherePairSet(ctx context.Context, a, b Ball, op BooleanOp, tolerance float64) (SpherePairSet, error) {
	var zero SpherePairSet
	if ctx == nil || op > Difference {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	kind := geom.SphereSetUnion
	switch op {
	case Intersection:
		kind = geom.SphereSetIntersection
	case Difference:
		kind = geom.SphereSetDifference
	}
	query, e := geom.NewSpherePairMaterial(a.Center, a.Radius, b.Center, b.Radius, kind)
	if e != nil {
		return zero, e
	}
	differences := 0
	for i, v := range []float64{a.Center.X, a.Center.Y, a.Center.Z} {
		if v != []float64{b.Center.X, b.Center.Y, b.Center.Z}[i] {
			differences++
		}
	}
	if differences > 1 {
		return zero, geom.ErrDomain
	}
	expected, e := query.ExposedSphereSupports()
	if e != nil {
		return zero, e
	}
	transverse, e := query.HasTransverseBoundaryIntersection()
	if e != nil {
		return zero, e
	}
	solid, e := CombineSpheres(a, b, op, tolerance)
	if e != nil {
		return zero, e
	}
	out := SpherePairSet{solid: solid, query: query}
	var observed geom.BoreBoundaryMask
	for id, face := range solid.Faces() {
		full := face.V.Min == -math.Pi/2 && face.V.Max == math.Pi/2
		if transverse == full {
			return zero, geom.ErrDomain
		}

		surface, ok := face.Surface.(geom.SphereSurface)
		if !ok {
			return zero, geom.ErrDomain
		}
		f, r := surface.Parameters()
		axes := [3]geom.Vec3{f.XAxis(), f.YAxis(), f.ZAxis()}
		for _, v := range axes {
			count := 0
			for _, c := range []float64{v.X, v.Y, v.Z} {
				if c == 1 || c == -1 {
					count++
				} else if c != 0 {
					return zero, geom.ErrDomain
				}
			}
			if count != 1 {
				return zero, geom.ErrDomain
			}
		}
		if axes[0].Cross(axes[1]) != axes[2] {
			return zero, geom.ErrDomain
		}
		owners := geom.BoreBoundaryMask(0)
		for owner, ball := range []Ball{a, b} {
			if f.Origin() == ball.Center && r == ball.Radius {
				mask := geom.BoreBoundaryMask(1 << uint(owner))
				if expected&mask == 0 {
					return zero, geom.ErrDomain
				}
				if face.Reversed != (op == Difference && owner == 1) {
					return zero, geom.ErrDomain
				}
				out.faces[owner] = append(out.faces[owner], id)
				owners |= mask
			}
		}
		if owners == 0 {
			return zero, geom.ErrDomain
		}
		observed |= owners
	}
	if observed != expected {
		return zero, geom.ErrDomain
	}
	for owner := 0; owner < 2; owner++ {
		want := 0
		if expected&geom.BoreBoundaryMask(1<<uint(owner)) != 0 {
			want = 1
		}
		if len(out.faces[owner]) != want {
			return zero, geom.ErrDomain
		}
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	out.valid = true
	return out, nil
}

// Solid returns the checked immutable native spherical boundary.
func (s SpherePairSet) Solid() Solid { return s.solid }

// SourceFaces returns caller-owned face IDs in original input order. Coincident
// union/intersection has the same native face in both source lists.
func (s SpherePairSet) SourceFaces() [2][]int {
	return [2][]int{append([]int(nil), s.faces[0]...), append([]int(nil), s.faces[1]...)}
}

// Classify applies exact regularized material predicates to represented XYZ.
func (s SpherePairSet) Classify(p geom.Vec3) (geom.BorePointLocation, geom.BoreBoundaryMask, error) {
	if !s.valid {
		return geom.BoreOutside, 0, geom.ErrInvalid
	}
	return s.query.Classify(p)
}

// ClipLine preserves algebraic parameter endpoints and original source masks.
func (s SpherePairSet) ClipLine(ctx context.Context, origin, direction geom.Vec3, minT, maxT float64, o geom.ToroidLineOptions) (geom.SpherePairMaterialLine, error) {
	if !s.valid {
		return geom.SpherePairMaterialLine{}, geom.ErrInvalid
	}
	return s.query.ClipLine(ctx, origin, direction, minT, maxT, o)
}

// FirstBoundaryHit returns the first exposed event with exact normal evidence.
func (s SpherePairSet) FirstBoundaryHit(ctx context.Context, origin, direction geom.Vec3, maxT float64, o geom.ToroidLineOptions) (geom.SpherePairFirstHit, error) {
	if !s.valid {
		return geom.SpherePairFirstHit{}, geom.ErrInvalid
	}
	return s.query.FirstBoundaryHit(ctx, origin, direction, maxT, o)
}
