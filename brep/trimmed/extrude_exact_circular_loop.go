package trimmed

import (
	"context"
	"fmt"
	"math"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// ExactCircularLoopSegment is one authored straight edge or a directed circular
// arc. Positive Radius requires exact endpoints on the represented circle and
// a counterclockwise sweep for ExtrudeExactCircularLoop. The directed constructor
// also accepts clockwise minor arcs. A line has Radius zero.
type ExactCircularLoopSegment struct {
	Start, End, Center geom.Vec2
	Radius             float64
}

// ExactCircularLoopOptions bounds source construction and validation. The
// display tolerance defaults to 1e-8 and affects finite evaluation checks.
type ExactCircularLoopOptions struct {
	DisplayTolerance float64
	Surface          geom.SurfacePatchOptions
	Trims            trimvalidate.Options
	Edges            edgecheck.Options
}

// ExtrudeExactCircularLoop extrudes a validated CCW line/arc loop along model
// Z. The arc supports and cap p-curves use the rational stereographic circle
// map and share the same homogeneous controls. The complete planar trim and
// every continuous coedge are certified. The finished closed boundary retains
// original analytic arcs even when their middle controls cannot fit binary64.
func ExtrudeExactCircularLoop(ctx context.Context, input []ExactCircularLoopSegment, height float64, o ExactCircularLoopOptions) (Solid, error) {
	return extrudeExactCircularLoop(ctx, input, [2]float64{min(height, 0), max(height, 0)}, false, o)
}

// ExtrudeExactDirectedCircularLoop validates and sweeps a CCW profile whose
// individual circular segments may turn clockwise or counterclockwise.
// Exact endpoint radii and cross products determine the minor-arc direction.
// Both original model-space cap elevations are retained.
func ExtrudeExactDirectedCircularLoop(ctx context.Context, input []ExactCircularLoopSegment, zMin, zMax float64, o ExactCircularLoopOptions) (Solid, error) {
	return extrudeExactCircularLoop(ctx, input, [2]float64{zMin, zMax}, true, o)
}

func extrudeExactCircularLoop(ctx context.Context, input []ExactCircularLoopSegment, z [2]float64, directed bool, o ExactCircularLoopOptions) (Solid, error) {
	if o.DisplayTolerance == 0 {
		o.DisplayTolerance = 1e-8
	}
	if ctx == nil || len(input) < 3 || len(input) > 64 || !finite(z[0]) || !finite(z[1]) || z[0] >= z[1] || !finite(z[1]-z[0]) ||
		!finite(o.DisplayTolerance) || o.DisplayTolerance <= 0 {
		return Solid{}, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return Solid{}, err
	}
	loops := make([][][3]*big.Rat, len(input))
	box := [2]geom.Vec2{{X: math.Inf(1), Y: math.Inf(1)}, {X: math.Inf(-1), Y: math.Inf(-1)}}
	for i, segment := range input {
		if err := ctx.Err(); err != nil {
			return Solid{}, err
		}
		if segment.End != input[(i+1)%len(input)].Start || segment.Start == segment.End ||
			!uvFinite(segment.Start) || !uvFinite(segment.End) || !uvFinite(segment.Center) || !finite(segment.Radius) || segment.Radius < 0 {
			return Solid{}, geom.ErrInvalid
		}
		controls, err := exactCircularLoopControls(segment, directed)
		if err != nil {
			return Solid{}, err
		}
		loops[i] = controls
		for _, p := range controls {
			for axis := 0; axis < 2; axis++ {
				x, exact := new(big.Rat).Quo(p[axis], p[2]).Float64()
				if !finite(x) {
					return Solid{}, geom.ErrUnrepresentable
				}
				lo, hi := x, x
				if !exact {
					lo, hi = math.Nextafter(x, math.Inf(-1)), math.Nextafter(x, math.Inf(1))
				}
				if axis == 0 {
					box[0].X, box[1].X = min(box[0].X, lo), max(box[1].X, hi)
				} else {
					box[0].Y, box[1].Y = min(box[0].Y, lo), max(box[1].Y, hi)
				}
			}
		}
	}
	n := len(input)
	faces := make([]Face, n+2)
	vertices := make([]geom.Vec3, 2*n)
	for j, h := range z {
		low, high := box[0], box[1]
		surface, err := geom.NewBSplineSurface(1, 1, 2, 2,
			[]geom.Vec3{{X: low.X, Y: low.Y, Z: h}, {X: low.X, Y: high.Y, Z: h},
				{X: high.X, Y: low.Y, Z: h}, {X: high.X, Y: high.Y, Z: h}},
			[]float64{low.X, low.X, high.X, high.X}, []float64{low.Y, low.Y, high.Y, high.Y}, nil)
		if err != nil {
			return Solid{}, err
		}
		faces[j] = Face{Surface: surface, Reversed: j == 0}
		for i, edge := range input {
			vertices[j*n+i] = geom.Vec3{X: edge.Start.X, Y: edge.Start.Y, Z: h}
		}
	}
	edges := make([]parametric.Edge, 3*n)
	chart := [4]geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}
	for i, controls := range loops {
		next := (i + 1) % n
		net := make([][4]*big.Rat, 0, 2*len(controls))
		for _, h := range controls {
			for _, level := range z {
				net = append(net, [4]*big.Rat{h[0], h[1], new(big.Rat).Mul(ratInset(level), h[2]), h[2]})
			}
		}
		patch, err := geom.NewExactHomogeneousBezierSurface(ctx, len(controls)-1, 1, net, o.Surface)
		if err != nil {
			return Solid{}, err
		}
		wall, err := geom.NewExactBezierSurface(patch)
		if err != nil {
			return Solid{}, err
		}
		interval := parametric.Interval{Max: 1}
		edges[i] = parametric.Edge{Start: i, End: next, Curve: surfaceTrimCurve{surface: wall, a: chart[0], b: chart[1]}, Range: interval}
		edges[n+i] = parametric.Edge{Start: n + i, End: n + next, Curve: surfaceTrimCurve{surface: wall, a: chart[3], b: chart[2]}, Range: interval}
		edges[2*n+i] = parametric.Edge{Start: i, End: n + i, Curve: surfaceTrimCurve{surface: wall, a: chart[0], b: chart[3]}, Range: interval}
		faces[2+i] = Face{Surface: wall, Outer: []Use{
			{Edge: i, UVStart: chart[0], UVEnd: chart[1]},
			{Edge: 2*n + next, UVStart: chart[1], UVEnd: chart[2]},
			{Edge: n + i, Reversed: true, UVStart: chart[2], UVEnd: chart[3]},
			{Edge: 2*n + i, Reversed: true, UVStart: chart[3], UVEnd: chart[0]},
		}}
		var curve PCurve
		if len(controls) == 3 {
			curve, err = NewExactQuadraticUV([3][3]*big.Rat(controls))
			if err != nil {
				return Solid{}, err
			}
		}
		for j := range z {
			faces[j].Outer = append(faces[j].Outer, Use{Edge: j*n + i, UVStart: input[i].Start, UVEnd: input[i].End, Curve: curve})
		}
	}
	for i, cap := range faces[:2] {
		var certificate *trimvalidate.CertifiedLoops
		var retain **trimvalidate.CertifiedLoops
		if directed && i == 0 {
			retain = &certificate
		}
		proof, err := proveFaceTrimLoops(ctx, cap, o.Trims, retain, nil, nil)
		if err != nil {
			return Solid{}, err
		}
		if proof.Status != trimvalidate.Certified {
			return Solid{}, fmt.Errorf("trimmed: unresolved exact circular cap loops: %w", geom.ErrConvergence)
		}
		if retain != nil {
			dx := new(big.Rat).Sub(ratInset(box[1].X), ratInset(box[0].X))
			dy := new(big.Rat).Sub(ratInset(box[1].Y), ratInset(box[0].Y))
			width := new(big.Rat).Mul(new(big.Rat).Mul(dx, dy), big.NewRat(1, 128))
			area, err := certificate.BoundSignedArea(ctx, width)
			if err != nil {
				return Solid{}, err
			}
			if area.Lower.Sign() <= 0 {
				return Solid{}, fmt.Errorf("trimmed: directed circular profile must have certified positive orientation: %w", geom.ErrInvalid)
			}
		}
	}
	out, err := NewSolidContext(ctx, vertices, edges, faces, o.DisplayTolerance)
	if err != nil {
		return Solid{}, err
	}
	proof, err := out.ValidateEdgeAgreement(ctx, o.Edges)
	if err != nil {
		return Solid{}, err
	}
	if !proof.Complete {
		return Solid{}, fmt.Errorf("trimmed: unresolved exact circular sweep coedges: %w", geom.ErrConvergence)
	}
	return out, nil
}

func exactCircularLoopControls(segment ExactCircularLoopSegment, directed bool) ([][3]*big.Rat, error) {
	start := [2]*big.Rat{ratInset(segment.Start.X), ratInset(segment.Start.Y)}
	end := [2]*big.Rat{ratInset(segment.End.X), ratInset(segment.End.Y)}
	one := big.NewRat(1, 1)
	if segment.Radius == 0 {
		return [][3]*big.Rat{{start[0], start[1], one}, {end[0], end[1], one}}, nil
	}
	center := [2]*big.Rat{ratInset(segment.Center.X), ratInset(segment.Center.Y)}
	sub := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
	mul := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
	add := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Add(a, b) }
	x := [2]*big.Rat{sub(start[0], center[0]), sub(start[1], center[1])}
	y := [2]*big.Rat{sub(end[0], center[0]), sub(end[1], center[1])}
	dot := func(a, b [2]*big.Rat) *big.Rat { return add(mul(a[0], b[0]), mul(a[1], b[1])) }
	want := mul(ratInset(segment.Radius), ratInset(segment.Radius))
	cross := sub(mul(x[0], y[1]), mul(x[1], y[0]))
	if dot(x, x).Cmp(want) != 0 || dot(y, y).Cmp(want) != 0 || cross.Sign() == 0 || (!directed && cross.Sign() < 0) {
		return nil, geom.ErrUnrepresentable
	}
	// Tangent half-angle is rational for rational endpoints. Restrict
	// ((1-t²)/(1+t²), 2t/(1+t²)) to that finite parameter and use the
	// original endpoint center and radius vectors before dehomogenization.
	t := new(big.Rat).Quo(cross, add(want, dot(x, y)))
	endWeight := add(one, mul(t, t))
	middle := [2]*big.Rat{sub(start[0], mul(t, x[1])), add(start[1], mul(t, x[0]))}
	return [][3]*big.Rat{{start[0], start[1], one}, {middle[0], middle[1], one},
		{mul(end[0], endWeight), mul(end[1], endWeight), endWeight}}, nil
}
