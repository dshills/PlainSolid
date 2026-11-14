package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// SideOpenDualDiskCut is a rectangular stock with two overlapping equal
// circular cutters opening the same lower side. Cutters start on z=0
// and terminate at one common depth in the supplied signed-coordinate frame.
// WallFaces retains original input cutter order (one exact chart each).
// FloorFace is -1 for a through cut.
type SideOpenDualDiskCut struct {
	Solid     Solid
	WallFaces [2][]int
	FloorFace int
	source    *sideOpenDualDiskSource
}

// CutSideOpenDualDisks constructs the solid from the certified section and
// its straight axial continuation. Bounded rational cylindrical charts
// represent the exposed mathematical circles without angle rounding.
// Only the exact section/float-control domain is accepted; no faceted
// fallback or supported-footprint expansion occurs.
func CutSideOpenDualDisks(ctx context.Context, frame geom.Frame, min, max geom.Vec2, discs [2]ExtrusionCircle, height, depth, tolerance float64, topology trimvalidate.Options) (SideOpenDualDiskCut, error) {
	return cutSideOpenDualDisks(ctx, frame, min, max, discs, height, depth, tolerance, topology, false)
}

func cutSideOpenDualDisks(ctx context.Context, frame geom.Frame, min, max geom.Vec2, discs [2]ExtrusionCircle, height, depth, tolerance float64, topology trimvalidate.Options, unequal bool) (SideOpenDualDiskCut, error) {
	var zero SideOpenDualDiskCut
	if ctx == nil || !finite(height) || !finite(depth) || !finite(tolerance) || tolerance <= 0 || depth <= 0 || depth > height {
		return zero, geom.ErrInvalid
	}
	s, e := certifySideOpenDualDiskSection(ctx, min, max, discs, topology, unequal)
	if e != nil {
		return zero, e
	}
	// The section splits each circle at its highest point for proof and
	// display. A single projective chart across each exposed physical arc
	// can have exactly representable storage even when a subdivided
	// Euclidean pole does not. It uses the same proved circular graph.
	chain, owners := make([]ExactQuadraticUV, 0, len(s.Outer)), make([]int, 0, len(s.Owner))
	for i := 0; i < len(s.Outer); i++ {
		o := s.Owner[i]
		q := s.Outer[i]
		if o >= 0 {
			if i+1 >= len(s.Outer) || s.Owner[i+1] != o {
				return zero, geom.ErrConvergence
			}
			a, _ := q.Endpoints()
			z, _ := s.Outer[i+1].Endpoints()
			d := discs[o]
			q, e = shortRationalCircleArc(a[0], z[1], [2]*big.Rat{ratInset(d.Center.X), ratInset(d.Center.Y)}, ratInset(d.Radius))
			if e != nil {
				return zero, e
			}
			i++
		}
		chain, owners = append(chain, q), append(owners, o)
	}
	s.Outer, s.Owner = chain, owners
	var parts []SewFace
	out := SideOpenDualDiskCut{FloorFace: -1}
	appendFace := func(s *geom.BSplineSurface, loop []ExactQuadraticUV, reverse bool) error {
		p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
		if e != nil {
			return e
		}
		var points [][2]*big.Rat
		for _, q := range loop {
			x, _ := q.Endpoints()
			points = append(points, x[0])
		}
		f, e := slabSewFace(ctx, Face{Surface: s, Reversed: reverse}, p, points, SplineRectangleEmbeddingOptions{})
		if e != nil {
			return e
		}
		cs := make([]PCurve, len(loop))
		for i, q := range loop {
			cs[i] = q
		}
		if e = installRationalGraphFragmentTrims(ctx, &f, cs, false); e != nil {
			return e
		}
		parts = append(parts, f)
		return nil
	}
	capOrigin := func(z float64) (*geom.BSplineSurface, error) {
		d := [4]geom.Vec3{{X: min.X, Y: min.Y, Z: z}, {X: min.X, Y: max.Y, Z: z}, {X: max.X, Y: min.Y, Z: z}, {X: max.X, Y: max.Y, Z: z}}
		return exactSignedBilinearChart(frame, d, [2]float64{min.X, max.X}, [2]float64{min.Y, max.Y})
	}
	horizontal := func(z float64, loop []ExactQuadraticUV, reverse bool) error {
		s, e := capOrigin(z)
		if e != nil {
			return e
		}
		return appendFace(s, loop, reverse)
	}
	if e = horizontal(0, s.Outer, true); e != nil {
		return zero, e
	}
	for i, q := range s.Outer {
		p, _ := q.Endpoints()
		var wall *geom.BSplineSurface
		if owner := s.Owner[i]; owner >= 0 {
			c := discs[owner]
			wall, e = geom.NewRationalCylindricalSpan(ctx, frame, [2]*big.Rat{ratInset(c.Center.X), ratInset(c.Center.Y)}, ratInset(c.Radius), new(big.Rat), ratInset(depth), q.HomogeneousControls())
			out.WallFaces[owner] = append(out.WallFaces[owner], len(parts))
		} else {
			wall, e = dualDiskWallChart(frame, p, 0, depth)
		}
		if e != nil {
			return zero, e
		}
		if e = appendFace(wall, dualDiskUnitSquare(), false); e != nil {
			return zero, e
		}
	}
	if depth == height {
		if e = horizontal(height, s.Outer, false); e != nil {
			return zero, e
		}
	} else {
		// At the blind floor, reverse the material-wall arc chain
		// and close it with the chord on the original side plane.
		var floor []ExactQuadraticUV
		beg, end := -1, -1
		for i, owner := range s.Owner {
			if owner >= 0 {
				if beg < 0 {
					beg = i
				}
				end = i
			}
		}
		x, _ := s.Outer[beg].Endpoints()
		y, _ := s.Outer[end].Endpoints()
		line, e := quadraticLine(x[0], y[1])
		if e != nil {
			return zero, e
		}
		floor = append(floor, line)
		for i := end; i >= beg; i-- {
			floor = append(floor, reversePlaneQuadratic(s.Outer[i]))
		}
		out.FloorFace = len(parts)
		if e = horizontal(depth, floor, true); e != nil {
			return zero, e
		}
		// The original box side above the floor uses three through-
		// going subdivisions so every adjacent face has conforming edges.
		var rectangle []ExactQuadraticUV
		for i, q := range s.Outer {
			if i == beg {
				rectangle = append(rectangle, line)
			}
			if s.Owner[i] < 0 {
				rectangle = append(rectangle, q)
			}
		}
		for _, q := range rectangle {
			p, _ := q.Endpoints()
			wall, e := dualDiskWallChart(frame, p, depth, height)
			if e != nil {
				return zero, e
			}
			if e = appendFace(wall, dualDiskUnitSquare(), false); e != nil {
				return zero, e
			}
		}
		if e = horizontal(height, rectangle, false); e != nil {
			return zero, e
		}
	}
	solid, _, e := Sew(ctx, parts, SewOptions{Tolerance: tolerance})
	if e != nil {
		return zero, e
	}
	if len(solid.Topology().Shells) != 1 {
		return zero, geom.ErrConvergence
	}
	out.Solid = solid
	if !unequal {
		out.source = &sideOpenDualDiskSource{frame: frame, min: min, max: max, discs: discs, height: height, depth: depth,
			outer: s.Outer, owners: s.Owner}
	}
	return out, nil
}

func dualDiskWallChart(frame geom.Frame, p [2][2]*big.Rat, z0, z1 float64) (*geom.BSplineSurface, error) {
	a, e := geom.RepresentedExactPoint([3]*big.Rat{p[0][0], p[0][1], ratInset(z0)})
	if e != nil {
		return nil, e
	}
	b, e := geom.RepresentedExactPoint([3]*big.Rat{p[1][0], p[1][1], ratInset(z0)})
	if e != nil {
		return nil, e
	}
	c, d := a, b
	c.Z, d.Z = z1, z1
	return exactSignedBilinearChart(frame, [4]geom.Vec3{a, c, b, d}, [2]float64{0, 1}, [2]float64{0, 1})
}

func exactSignedBilinearChart(frame geom.Frame, poles [4]geom.Vec3, u, v [2]float64) (*geom.BSplineSurface, error) {
	for _, d := range []geom.Vec3{frame.XAxis(), frame.YAxis(), frame.ZAxis()} {
		n := 0
		for _, c := range []float64{d.X, d.Y, d.Z} {
			if c != 0 {
				if c != 1 && c != -1 {
					return nil, geom.ErrDomain
				}
				n++
			}
		}
		if n != 1 {
			return nil, geom.ErrDomain
		}
	}
	for i, p := range poles {
		o := frame.Origin()
		x, y, z := frame.XAxis(), frame.YAxis(), frame.ZAxis()
		var q [3]*big.Rat
		for j := range q {
			q[j] = ratInset([3]float64{o.X, o.Y, o.Z}[j])
			for k, a := range [3]geom.Vec3{x, y, z} {
				c := [3]float64{a.X, a.Y, a.Z}[j]
				if c != 0 {
					q[j].Add(q[j], new(big.Rat).Mul(ratInset(c), ratInset([3]float64{p.X, p.Y, p.Z}[k])))
				}
			}
		}
		w, e := geom.RepresentedExactPoint(q)
		if e != nil {
			return nil, e
		}
		poles[i] = w
	}
	return geom.NewBSplineSurface(1, 1, 2, 2, poles[:], []float64{u[0], u[0], u[1], u[1]}, []float64{v[0], v[0], v[1], v[1]}, nil)
}

func dualDiskUnitSquare() []ExactQuadraticUV {
	a := []geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}
	var out []ExactQuadraticUV
	for i, p := range a {
		b := a[(i+1)%len(a)]
		q, _ := quadraticLine([2]*big.Rat{ratInset(p.X), ratInset(p.Y)}, [2]*big.Rat{ratInset(b.X), ratInset(b.Y)})
		out = append(out, q)
	}
	return out
}
