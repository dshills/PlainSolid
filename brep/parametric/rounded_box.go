package parametric

import (
	"fmt"
	"math"

	"plainsolid/geom"
)

// RoundedBox is the outer parallel body of a rectangular core: the Minkowski
// sum of [min,max] and a radius ball. The extents are expanded by radius.
// To round edges inside fixed extents, inset each extent by radius before
// calling RoundedBox. The inset must leave three positive core lengths.
func RoundedBox(min, max geom.Vec3, radius float64) (Solid, error) {
	d := max.Sub(min)
	if !min.IsFinite() || !max.IsFinite() || !d.IsFinite() || d.X <= 0 || d.Y <= 0 || d.Z <= 0 || radius <= 0 || !finite(radius) {
		return Solid{}, geom.ErrInvalid
	}
	lengths := [3]float64{d.X, d.Y, d.Z}
	axes := [3]geom.Vec3{{X: 1}, {Y: 1}, {Z: 1}}
	core := func(high [3]bool) geom.Vec3 {
		p := min
		for i, h := range high {
			if h {
				p = p.Add(axes[i].Scale(lengths[i]))
			}
		}
		return p
	}
	var faces []Face
	for normal := 0; normal < 3; normal++ {
		i, j := (normal+1)%3, (normal+2)%3
		for _, positive := range []bool{false, true} {
			a, b := i, j
			high := [3]bool{}
			high[normal] = positive
			n := axes[normal]
			if !positive {
				a, b = j, i
				n = n.Scale(-1)
			}
			frame, err := geom.NewFrame(core(high).Add(n.Scale(radius)), axes[a], n)
			if err != nil {
				return Solid{}, err
			}
			s, err := geom.NewPlaneSurface(frame)
			if err != nil {
				return Solid{}, err
			}
			faces = append(faces, Face{Surface: s, U: Interval{0, lengths[a]}, V: Interval{0, lengths[b]}})
		}
	}
	for parallel := 0; parallel < 3; parallel++ {
		i, j := (parallel+1)%3, (parallel+2)%3
		for si := range 2 {
			for sj := range 2 {
				x, y := axes[i].Scale(float64(2*si-1)), axes[j].Scale(float64(2*sj-1))
				z := x.Cross(y)
				high := [3]bool{}
				high[i], high[j], high[parallel] = si == 1, sj == 1, z.Dot(axes[parallel]) < 0
				frame, err := geom.NewFrame(core(high), x, z)
				if err != nil {
					return Solid{}, err
				}
				s, err := geom.NewCylinderSurface(frame, radius)
				if err != nil {
					return Solid{}, err
				}
				faces = append(faces, Face{Surface: s, U: Interval{0, math.Pi / 2}, V: Interval{0, lengths[parallel]}})
			}
		}
	}
	for k, signs := range [][2]bool{{true, true}, {false, true}, {false, false}, {true, false}} {
		u := Interval{float64(k) * math.Pi / 2, float64(k+1) * math.Pi / 2}
		for _, z := range []bool{false, true} {
			frame, err := geom.FrameFromNormal(core([3]bool{signs[0], signs[1], z}), axes[2])
			if err != nil {
				return Solid{}, err
			}
			s, err := geom.NewSphereSurface(frame, radius)
			if err != nil {
				return Solid{}, err
			}
			v := Interval{-math.Pi / 2, 0}
			if z {
				v = Interval{0, math.Pi / 2}
			}
			faces = append(faces, Face{Surface: s, U: u, V: v})
		}
	}
	scale := math.Max(radius, math.Max(d.X, math.Max(d.Y, d.Z)))
	tolerance := scale * 1e-12
	if tolerance <= 0 || minFloat(d.X, d.Y, d.Z, radius) <= tolerance*8 {
		return Solid{}, fmt.Errorf("parametric: unresolved rounded box feature size")
	}
	return joinPrimitivePatches(faces, tolerance)
}

func minFloat(values ...float64) float64 {
	x := math.Inf(1)
	for _, v := range values {
		x = math.Min(x, v)
	}
	return x
}

// joinPrimitivePatches identifies incidences for known analytic constructions.
// It matches end vertices and checks sample orientation before sharing an edge.
func joinPrimitivePatches(faces []Face, tolerance float64) (Solid, error) {
	var vertices []geom.Vec3
	var edges []Edge
	vertex := func(p geom.Vec3) (int, error) {
		id := -1
		for i, q := range vertices {
			if near(p, q, tolerance) {
				if id >= 0 {
					return 0, fmt.Errorf("%w: ambiguous analytic vertex", ErrTopology)
				}
				id = i
			}
		}
		if id >= 0 {
			return id, nil
		}
		vertices = append(vertices, p)
		return len(vertices) - 1, nil
	}
	for i, f := range faces {
		for side := range 4 {
			curve := iso{f, side}
			p, e := curve.Eval(0)
			if e != nil {
				return Solid{}, e
			}
			q, e := curve.Eval(1)
			if e != nil {
				return Solid{}, e
			}
			a, e := vertex(p)
			if e != nil {
				return Solid{}, e
			}
			b, e := vertex(q)
			if e != nil {
				return Solid{}, e
			}
			if a == b {
				r, e := curve.Eval(.5)
				if e != nil {
					return Solid{}, e
				}
				if !near(r, p, tolerance) {
					return Solid{}, fmt.Errorf("%w: unresolved patch loop", ErrTopology)
				}
				f.Boundary[side] = Use{Edge: len(edges)}
				edges = append(edges, Edge{Start: a, End: b})
				continue
			}
			id := -1
			reverse := false
			for j, edge := range edges {
				if edge.Curve == nil {
					continue
				}
				reversed := edge.Start == b && edge.End == a
				if !(edge.Start == a && edge.End == b) && !reversed {
					continue
				}
				match := true
				for _, t := range []float64{.25, .5, .75} {
					u := t
					if reversed {
						u = 1 - t
					}
					p, e := curve.Eval(t)
					if e != nil {
						return Solid{}, e
					}
					q, e := edge.Curve.Eval(u)
					if e != nil {
						return Solid{}, e
					}
					if !near(p, q, tolerance) {
						match = false
						break
					}
				}
				if match {
					if id >= 0 {
						return Solid{}, fmt.Errorf("%w: ambiguous analytic edge", ErrTopology)
					}
					id, reverse = j, reversed
				}
			}
			if id < 0 {
				id = len(edges)
				edges = append(edges, Edge{Start: a, End: b, Range: normalized, Curve: curve})
			}
			f.Boundary[side] = Use{Edge: id, Reversed: reverse}
		}
		faces[i] = f
	}
	return NewSolid(vertices, edges, faces, tolerance)
}
