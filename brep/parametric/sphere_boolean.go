package parametric

import (
	"fmt"
	"math"

	"plainsolid/geom"
)

// Ball is a sphere's center and strictly positive radius.
type Ball struct {
	Center geom.Vec3
	Radius float64
}

type BooleanOp uint8

const (
	Union BooleanOp = iota
	Intersection
	Difference
)

// CombineSpheres performs a regularized Boolean on two balls and returns
// exact spherical patches bounded by their common circle. Difference is A-B.
// Its absolute tolerance controls boundary consistency and detection of
// numerically unresolved near-tangency. The operation is specialized to balls.
// An exactly tangent union or tangent internal cavity is rejected if it would
// join otherwise disconnected shells at a single point.
func CombineSpheres(a, b Ball, op BooleanOp, tolerance float64) (Solid, error) {
	if op > Difference || !finite(tolerance) || tolerance <= 0 ||
		!a.Center.IsFinite() || !b.Center.IsFinite() || a.Radius <= 0 || b.Radius <= 0 ||
		!finite(a.Radius) || !finite(b.Radius) {
		return Solid{}, fmt.Errorf("parametric: invalid sphere Boolean parameters")
	}
	sphere := func(c Ball) (Solid, error) { return Sphere(c.Center, c.Radius) }
	pair := func(c, d Ball, reverseSecond bool) (Solid, error) {
		s, err := sphere(c)
		if err != nil {
			return Solid{}, err
		}
		t, err := sphere(d)
		if err != nil {
			return Solid{}, err
		}
		if reverseSecond {
			t.faces[0].Reversed = !t.faces[0].Reversed
		}
		nv, ne := len(s.vertices), len(s.edges)
		for i := range t.edges {
			t.edges[i].Start += nv
			t.edges[i].End += nv
		}
		for i := range t.faces {
			for j := range t.faces[i].Boundary {
				t.faces[i].Boundary[j].Edge += ne
			}
		}
		return NewSolid(append(s.vertices, t.vertices...), append(s.edges, t.edges...), append(s.faces, t.faces...), math.Max(s.tolerance, t.tolerance))
	}
	delta := b.Center.Sub(a.Center)
	d := delta.Length()
	if !finite(d) {
		return Solid{}, fmt.Errorf("parametric: sphere separation outside float64 range")
	}
	scale := math.Max(d, math.Max(a.Radius, b.Radius))
	x, y, z := a.Radius/scale, b.Radius/scale, d/scale
	eps := tolerance / scale
	if x == 0 || y == 0 || !finite(eps) {
		return Solid{}, fmt.Errorf("parametric: unsupported sphere size ratio")
	}
	// Handle exact coincident operands before the internal tangency test.
	if d == 0 && a.Radius == b.Radius {
		if op == Difference {
			return Solid{}, nil
		}
		return sphere(a)
	}
	sum, diff := x+y, math.Abs(x-y)
	if z > sum+eps || math.Abs(z-sum) <= eps && tangent(a.Center, b.Center, a.Radius, b.Radius, false) {
		switch op {
		case Union:
			if z <= sum+eps {
				return Solid{}, fmt.Errorf("parametric: tangent sphere union is nonmanifold")
			}
			return pair(a, b, false)
		case Intersection:
			return Solid{}, nil
		default:
			return sphere(a)
		}
	}
	if d == 0 || z < diff-eps || math.Abs(z-diff) <= eps && tangent(a.Center, b.Center, a.Radius, b.Radius, true) {
		outer, inner := a, b
		if a.Radius < b.Radius {
			outer, inner = b, a
		}
		switch op {
		case Union:
			return sphere(outer)
		case Intersection:
			return sphere(inner)
		default:
			if a.Radius < b.Radius {
				return Solid{}, nil
			}
			if d != 0 && z >= diff-eps {
				return Solid{}, fmt.Errorf("parametric: tangent cavity is nonmanifold")
			}
			return pair(a, b, true)
		}
	}
	if math.Abs(z-sum) <= eps || math.Abs(z-diff) <= eps || z == 0 {
		return Solid{}, fmt.Errorf("parametric: unresolved near-tangent sphere Boolean")
	}
	if !(z < sum && z > diff) {
		return Solid{}, fmt.Errorf("parametric: unresolved sphere relation")
	}
	n, err := delta.Unit()
	if err != nil {
		return Solid{}, err
	}
	fa, err := geom.FrameFromNormal(a.Center, n)
	if err != nil {
		return Solid{}, err
	}
	fb, err := geom.NewFrame(b.Center, fa.XAxis(), n)
	if err != nil {
		return Solid{}, err
	}
	sa, err := geom.NewSphereSurface(fa, a.Radius)
	if err != nil {
		return Solid{}, err
	}
	sb, err := geom.NewSphereSurface(fb, b.Radius)
	if err != nil {
		return Solid{}, err
	}

	// Signed offsets of the same section plane relative to each center.
	// Factoring the radius-square difference limits cancellation.
	mid := (x - y) * (x + y) / z
	va := math.Asin((z + mid) / (2 * x))
	vb := math.Asin((mid - z) / (2 * y))
	if !finite(va) || !finite(vb) || math.Abs(va) >= math.Pi/2 || math.Abs(vb) >= math.Pi/2 {
		return Solid{}, fmt.Errorf("parametric: unresolved spherical cap dimensions")
	}

	// Intersect: north A + south B. Union: south A + north B.
	// Difference: south A + south B with the second cap reversed.
	aNorth, bNorth := op == Intersection, op == Union
	cap := func(surface geom.SphereSurface, latitude float64, north, reversed bool) (Face, int) {
		f := Face{Surface: surface, U: circleDomain, Reversed: reversed}
		if north {
			f.V = Interval{latitude, math.Pi / 2}
			return f, 0
		}
		f.V = Interval{-math.Pi / 2, latitude}
		return f, 2
	}
	f0, side0 := cap(sa, va, aNorth, false)
	f1, side1 := cap(sb, vb, bNorth, op == Difference)
	p, err := f0.sidePoint(side0, 0)
	if err != nil {
		return Solid{}, err
	}
	vertices := []geom.Vec3{p}
	edges := []Edge{{Start: 0, End: 0, Curve: iso{f0, side0}, Range: normalized}}
	faces := []Face{f0, f1}
	for i, f := range faces {
		side := side0
		if i == 1 {
			side = side1
		}
		poleSide := 2 - side
		pole, err := f.sidePoint(poleSide, 0)
		if err != nil {
			return Solid{}, err
		}
		poleID := len(vertices)
		vertices = append(vertices, pole)
		collapsed, seam := len(edges), len(edges)+1
		start, end := poleID, 0
		if side == 0 {
			start, end = 0, poleID
		}
		edges = append(edges, Edge{Start: poleID, End: poleID}, Edge{Start: start, End: end, Curve: iso{f, 1}, Range: normalized})
		f.Boundary[side] = Use{Edge: 0, Reversed: side != side0}
		f.Boundary[poleSide] = Use{Edge: collapsed}
		f.Boundary[1] = Use{Edge: seam}
		f.Boundary[3] = Use{Edge: seam, Reversed: true}
		faces[i] = f
	}
	return NewSolid(vertices, edges, faces, tolerance)
}
