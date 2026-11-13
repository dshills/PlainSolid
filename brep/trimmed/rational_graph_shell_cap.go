package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// RationalGraphCapEdgeSource identifies the section piece behind a local cap
// edge. Reversed is relative to the directed piece in the section result.
type RationalGraphCapEdgeSource struct {
	Wire, Branch, Piece, Face int
	Reversed                  bool
}

type projectedGraphSectionPiece struct {
	source  RationalGraphCapEdgeSource
	control [][3]*big.Rat
}

// RationalGraphSectionCap is an independently validated planar face, with
// section ancestry in local edge order. It is not sewn to either half-shell.
type RationalGraphSectionCap struct {
	Boundary SewFace
	Edges    []RationalGraphCapEdgeSource
}

type RationalGraphSectionCapOptions struct {
	Patches  geom.SurfacePatchOptions
	Topology trimvalidate.Options
}

// BuildCaps transports the section regions onto a caller-supplied affine
// spline support in their exact plane. An outer cap faces the oriented section
// plane direction; each void is a separate inner loop. The support rectangle
// must contain all projected control points. The current sewable BRep
// requires exactly representable model and UV endpoints; failure is atomic.
// Input is the previously certified private arrangement, so edits to returned
// diagnostic wires or material rows do not change the caps.
func (r *RationalGraphShellRegions) BuildCaps(ctx context.Context, surface geom.Surface, domain geom.ParameterRectangle, o RationalGraphSectionCapOptions) ([]RationalGraphSectionCap, error) {
	if ctx == nil || r == nil || r.certificate == nil || surface == nil || !domain.Valid() {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	topology, e := o.Topology.WithDefaults()
	if e != nil {
		return nil, e
	}
	base, placement := parametric.SurfaceMapping(surface)
	b, ok := base.(*geom.BSplineSurface)
	if !ok {
		return nil, geom.ErrConvergence
	}
	p, e := geom.NewSurfacePatch(b, o.Patches)
	if e == nil {
		p, e = p.Trim(domain)
	}
	if e == nil && placement != geom.Identity() {
		p, e = p.Transformed(placement)
	}
	if e != nil {
		return nil, e
	}
	chart, e := geom.NewAffineSurfaceChart(ctx, p, geom.AffineSurfaceChartOptions{
		MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits,
	})
	if e != nil {
		return nil, e
	}
	axes := orientedRationalGraphProjection(r.plane)
	d := chart.Domain()
	uv := [3][2]float64{{d.UMin, d.VMin}, {d.UMax, d.VMin}, {d.UMin, d.VMax}}
	var q [3][2]*big.Rat
	for i, x := range uv {
		v, e := chart.EvalExact(ratInset(x[0]), ratInset(x[1]))
		if e != nil {
			return nil, e
		}
		if r.plane.dot(v).Cmp(r.plane.offset) != 0 {
			return nil, geom.ErrDomain
		}
		q[i] = [2]*big.Rat{v[axes[0]], v[axes[1]]}
	}
	reverse := slabUVSide(q[0], q[1], q[2]).Sign()
	if reverse == 0 {
		return nil, geom.ErrDegenerate
	}
	count := 0
	for _, w := range r.wires {
		count += len(w)
	}
	if count > topology.MaxEdges {
		return nil, geom.ErrConvergence
	}
	var out []RationalGraphSectionCap
	for _, region := range r.material {
		cap := RationalGraphSectionCap{Boundary: SewFace{Face: Face{Surface: surface, Reversed: reverse < 0}}}
		for i, id := range append([]int{region.Outer}, region.Holes...) {
			wire := r.wires[id]
			part, edges, e := r.buildCapWire(ctx, wire, surface, p, chart, reverse < 0, o.Patches)
			if e != nil {
				return nil, e
			}
			nv, ne := len(cap.Boundary.Vertices), len(cap.Boundary.Edges)
			cap.Boundary.Vertices = append(cap.Boundary.Vertices, part.Vertices...)
			for _, edge := range part.Edges {
				edge.Start += nv
				edge.End += nv
				cap.Boundary.Edges = append(cap.Boundary.Edges, edge)
			}
			for j := range part.Face.Outer {
				part.Face.Outer[j].Edge += ne
			}
			if i == 0 {
				cap.Boundary.Face.Outer = part.Face.Outer
			} else {
				cap.Boundary.Face.Holes = append(cap.Boundary.Face.Holes, part.Face.Outer)
			}
			cap.Edges = append(cap.Edges, edges...)
		}
		proof, e := ValidateFaceTrimLoops(ctx, cap.Boundary.Face, topology)
		if e != nil {
			return nil, e
		}
		if proof.Status != trimvalidate.Certified {
			return nil, geom.ErrConvergence
		}
		out = append(out, cap)
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return out, nil
}

func (r *RationalGraphShellRegions) buildCapWire(ctx context.Context, wire []projectedGraphSectionPiece, surface geom.Surface, patch *geom.SurfacePatch, chart *geom.AffineSurfaceChart, reverse bool, o geom.SurfacePatchOptions) (SewFace, []RationalGraphCapEdgeSource, error) {
	var zero SewFace
	axes := orientedRationalGraphProjection(r.plane)
	limits := geom.AffineChartInverseOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	var points [][2]*big.Rat
	var curves []PCurve
	var sources []RationalGraphCapEdgeSource
	for i := range wire {
		j := i
		if reverse {
			j = len(wire) - 1 - i
		}
		x := wire[j]
		control := make([][3]*big.Rat, len(x.control))
		for k, p := range x.control {
			at := k
			if reverse {
				at = len(x.control) - 1 - k
			}
			var point [3]*big.Rat
			point[axes[0]], point[axes[1]] = new(big.Rat).Quo(p[0], p[2]), new(big.Rat).Quo(p[1], p[2])
			z := new(big.Rat).Set(r.plane.offset)
			for _, axis := range axes {
				z.Sub(z, new(big.Rat).Mul(point[axis], r.plane.normal[axis]))
			}
			point[r.plane.dropped] = z
			uv, e := chart.InvertExactContext(ctx, point, limits)
			if e != nil {
				return zero, nil, e
			}
			if !uv.OnPlane || !uv.Inside {
				return zero, nil, geom.ErrDomain
			}
			control[at] = [3]*big.Rat{new(big.Rat).Mul(uv.U, p[2]), new(big.Rat).Mul(uv.V, p[2]), p[2]}
		}
		var c PCurve
		var e error
		switch len(control) {
		case 3:
			c, e = NewExactQuadraticUV([3][3]*big.Rat{control[0], control[1], control[2]})
		case 4, 5:
			c, e = NewExactBezierUV(control)
		default:
			e = geom.ErrConvergence
		}
		if e != nil {
			return zero, nil, e
		}
		ends, _, e := polynomialPlaneEndpoints(c)
		if e != nil {
			return zero, nil, e
		}
		x.source.Reversed = reverse
		points, curves, sources = append(points, ends[0]), append(curves, c), append(sources, x.source)
	}
	part, e := slabSewFace(ctx, Face{Surface: surface, Reversed: reverse}, patch, points, SplineRectangleEmbeddingOptions{Patches: o})
	if e != nil {
		return zero, nil, e
	}
	for i, c := range curves {
		u := &part.Face.Outer[i]
		_, xy, e := polynomialPlaneEndpoints(c)
		if e != nil || xy != [2]geom.Vec2{u.UVStart, u.UVEnd} {
			return zero, nil, geom.ErrConvergence
		}
		u.Curve = c
		part.Edges[i].Curve = surfaceTrimCurve{surface, u.UVStart, u.UVEnd, c}
	}
	return part, sources, nil
}
