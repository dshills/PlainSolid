package boolean

import (
	"context"
	"errors"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// measure integrates the volume and area of a solid for V5 and maps a
// failure as V5 states. An empty solid measures zero.
func measure(ctx context.Context, s trimmed.Solid, empty bool, o parametric.IntegrationOptions) (parametric.Properties, error) {
	if empty {
		return parametric.Properties{}, nil
	}
	p, err := s.MassProperties(ctx, o)
	switch {
	case err == nil:
		return p, nil
	case ctx.Err() != nil:
		return parametric.Properties{}, ctx.Err()
	case errors.Is(err, parametric.ErrBudget):
		return parametric.Properties{}, failWith(ErrBudget, "V5", err)
	}
	return parametric.Properties{}, failWith(ErrValidation, "V5", err)
}

// residuals measures every edge of the assembled result at 17 parameters
// against the carriers of its faces and against their p-curves (V3), and
// every vertex against the ends of its edges (V4).
func residuals(b *body) (edgeResidual, vertexResidual float64, err error) {
	point := func(e parametric.Edge, t float64) (geom.Vec3, error) {
		if t == 1 {
			return e.Curve.Eval(e.Range.Max)
		}
		return e.Curve.Eval(e.Range.Min + (e.Range.Max-e.Range.Min)*t)
	}
	for fi, f := range b.faces {
		for _, loop := range append([][]trimmed.Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				for k := 0; k < edgeSamples; k++ {
					t := float64(k) / (edgeSamples - 1)
					uv := geom.Vec2{X: (1-t)*u.UVStart.X + t*u.UVEnd.X, Y: (1-t)*u.UVStart.Y + t*u.UVEnd.Y}
					if u.Curve != nil {
						if uv, err = u.Curve.EvalUV(t); err != nil {
							return 0, 0, failWith(ErrValidation, "V3", err)
						}
					}
					if u.Reversed {
						t = 1 - t
					}
					onEdge, err := point(b.edges[u.Edge], t)
					if err != nil {
						return 0, 0, failWith(ErrValidation, "V3", err)
					}
					onFace, err := f.Surface.Eval(uv.X, uv.Y)
					if err != nil {
						return 0, 0, failWith(ErrValidation, "V3", err)
					}
					d := math.Max(math.Abs(b.sheets[fi].s.offset(onEdge)), onEdge.Distance(onFace))
					if edgeResidual = math.Max(edgeResidual, d); !(d <= b.tau+b.slack) {
						return 0, 0, fail(ErrValidation, "V3", "edge %d is %g from face %d", u.Edge, d, fi)
					}
				}
			}
		}
	}
	for ei, e := range b.edges {
		for k, v := range [2]int{e.Start, e.End} {
			p, err := point(e, float64(k))
			if err != nil {
				return 0, 0, failWith(ErrValidation, "V4", err)
			}
			d := p.Distance(b.verts[v])
			if vertexResidual = math.Max(vertexResidual, d); !(d <= b.tau+b.slack) {
				return 0, 0, fail(ErrValidation, "V4", "vertex %d is %g from an end of edge %d", v, d, ei)
			}
		}
	}
	return edgeResidual, vertexResidual, nil
}

// identities checks the volume identities of V5. The volumes of A, B and
// the result are integrated; those of the four combinations come from the
// classified pieces of the one arrangement.
func identities(f *fabric, pieces []piece, op Operation, a, b, result parametric.Properties, o Options, t tolerances) error {
	beta := o.Volume*math.Max(a.Volume, b.Volume) + t.eps*(a.Area+b.Area)
	common := f.volume(pieces, choose(pieces, Common, false))
	union := f.volume(pieces, choose(pieces, Union, false))
	aLessB := f.volume(pieces, choose(pieces, Cut, false))
	bLessA := f.volume(pieces, choose(pieces, Cut, true))
	asked := [...]float64{Union: union, Common: common, Cut: aLessB}[op]
	for _, c := range []struct {
		name  string
		value float64
	}{
		{"V(A−B) + V(A∩B) − V(A)", aLessB + common - a.Volume},
		{"V(B−A) + V(A∩B) − V(B)", bLessA + common - b.Volume},
		{"V(A∪B) − V(A) − V(B) + V(A∩B)", union - a.Volume - b.Volume + common},
		{"V(result) − the volume of its pieces", result.Volume - asked},
		{"the lower bound of V(A∩B)", math.Min(common, 0)},
		{"the upper bound of V(A∩B)", math.Max(common-math.Min(a.Volume, b.Volume), 0)},
		{"the lower bound of V(A∪B)", math.Min(union-math.Max(a.Volume, b.Volume), 0)},
	} {
		if !(math.Abs(c.value) <= beta) {
			return fail(ErrValidation, "V5", "%s is %g, beyond %g", c.name, c.value, beta)
		}
	}
	return nil
}

// halton is term i of the Halton sequence in the given base.
func halton(i, base int) float64 {
	h, f := 0., 1.
	for ; i > 0; i /= base {
		f /= float64(base)
		h += f * float64(i%base)
	}
	return h
}

// membership checks V6: at sample points of the box of both operands,
// the result must contain exactly the points the operation selects.
// Points that any of the three solids reports as boundary are skipped.
func membership(ops [2]*operand, empty [2]bool, result *body, op Operation, o Options, t tolerances) error {
	var box bounds
	switch {
	case empty[0]:
		box = ops[1].box
	case empty[1]:
		box = ops[0].box
	default:
		box = ops[0].box.with(ops[1].box)
	}
	box = box.grown(4 * t.eps)
	size := box.hi.Sub(box.lo)
	locate := func(s trimmed.Solid, none bool, p geom.Vec3, tol float64) (parametric.Location, error) {
		if none {
			return parametric.Outside, nil
		}
		return s.Classify(p, tol)
	}
	for i := 1; i <= o.Samples; i++ {
		p := box.lo.Add(geom.Vec3{X: halton(i, 2) * size.X, Y: halton(i, 3) * size.Y, Z: halton(i, 5) * size.Z})
		var where [3]parametric.Location
		for k, s := range [3]struct {
			solid trimmed.Solid
			none  bool
			tol   float64
		}{{ops[0].solid, empty[0], t.eps}, {ops[1].solid, empty[1], t.eps}, {result.solid, result.faces == nil, result.tau}} {
			at, err := locate(s.solid, s.none, p, s.tol)
			if err != nil {
				return failWith(ErrValidation, "V6", err)
			}
			where[k] = at
		}
		if where[0] == parametric.Boundary || where[1] == parametric.Boundary || where[2] == parametric.Boundary {
			continue
		}
		inA, inB := where[0] == parametric.Inside, where[1] == parametric.Inside
		want := [...]bool{Union: inA || inB, Common: inA && inB, Cut: inA && !inB}[op]
		if (where[2] == parametric.Inside) != want {
			return fail(ErrValidation, "V6", "the result disagrees with the operands at sample %d, %v", i, p)
		}
	}
	return nil
}
