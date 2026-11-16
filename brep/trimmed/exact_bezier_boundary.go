package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ExactBezierBoundary describes source-backed spline or affine-wall faces
// and their shared edges. Model curves and pcurves use [0,1] in native edge
// direction. Coedge reversal gives the face-loop traversal. Homogeneous
// controls are owned by the result and can be changed without editing Solid.
type ExactBezierBoundary struct {
	Vertices []geom.Vec3
	Edges    []ExactBezierBoundaryEdge
	Faces    []ExactBezierBoundaryFace
}

type ExactBezierBoundaryEdge struct {
	Start, End int
	Model      geom.ExactBezierSpan
}

type ExactBezierBoundaryFace struct {
	Spline   *geom.BSplineSurface
	Affine   *edgecheck.Chart
	Loops    [][]ExactBezierBoundaryUse
	Reversed bool
	// Native graph walls use a unit-length clock independent of the physical
	// side vector. Preserve that same chart in existing standard exports.
	affineUnitU bool
}

type ExactBezierBoundaryUse struct {
	Edge     int
	Reversed bool
	UV       geom.ExactBezierSpan // coordinates are (u,v,0) before weighting
}

// ExtractExactBezierBoundary exports continuous model and UV laws from
// supported B-spline and affine-wall faces. It requires zero residual for
// every coedge, exact endpoint identity, and one positive-weight Bézier for
// each model edge and UV use. It refuses unsupported analytic, multi-span
// and algebraic trim laws rather than replacing them with fitted samples.
func (s Solid) ExtractExactBezierBoundary(ctx context.Context, o edgecheck.Options) (ExactBezierBoundary, error) {
	return s.extractExactBezierBoundary(ctx, o, false)
}

func (s Solid) extractExactBezierBoundary(ctx context.Context, o edgecheck.Options, splitKnots bool) (ExactBezierBoundary, error) {
	var zero ExactBezierBoundary
	if ctx == nil || len(s.faces) == 0 {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	out := ExactBezierBoundary{Vertices: s.Vertices(), Edges: make([]ExactBezierBoundaryEdge, len(s.edges)), Faces: make([]ExactBezierBoundaryFace, len(s.faces))}
	for i, f := range s.faces {
		exportable := ExactBezierBoundaryFace{Reversed: f.Reversed}
		switch x := f.Surface.(type) {
		case *geom.BSplineSurface:
			exportable.Spline = x
		case splineGraphAffineWall:
			c := x.chart()
			exportable.Affine = &c
			exportable.affineUnitU = true
		case geom.PlaneSurface:
			c, ok := edgeChart(x)
			if !ok {
				return zero, fmt.Errorf("trimmed: exact Bézier plane %d: %w", i, geom.ErrDomain)
			}
			exportable.Affine = &c
		default:
			return zero, fmt.Errorf("trimmed: exact Bézier face %d: %w", i, geom.ErrDomain)
		}
		out.Faces[i] = exportable
	}
	proof, err := s.ValidateEdgeAgreement(ctx, o)
	if err != nil {
		return zero, err
	}
	if !proof.Complete {
		return zero, geom.ErrConvergence
	}
	for _, use := range proof.Uses {
		if use.Report.SquaredUpperBound == nil || use.Report.SquaredUpperBound.Sign() != 0 {
			return zero, fmt.Errorf("trimmed: exact Bézier face %d edge %d: %w", use.Face, use.Edge, geom.ErrConvergence)
		}
	}
	limits := geom.SurfacePatchOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits}
	splits := make(map[int][]exactBoundaryKnotPart)
	for i, e := range s.edges {
		model, err := exactBoundaryModel(ctx, e, limits)
		if splitKnots && errors.Is(err, geom.ErrConvergence) {
			parts, splitErr := out.splitRationalKnotEdge(ctx, i, e, o)
			if splitErr != nil {
				return zero, fmt.Errorf("trimmed: exact Bézier edge %d knot cells: %w", i, splitErr)
			}
			splits[i] = parts
			continue
		}
		if err != nil {
			return zero, fmt.Errorf("trimmed: exact Bézier edge %d: %w", i, err)
		}
		if !exactBoundaryEnd(model.Control[0], s.vertices[e.Start]) || !exactBoundaryEnd(model.Control[len(model.Control)-1], s.vertices[e.End]) {
			return zero, fmt.Errorf("trimmed: exact Bézier model endpoint on edge %d: %w", i, geom.ErrConvergence)
		}
		out.Edges[i] = ExactBezierBoundaryEdge{Start: e.Start, End: e.End, Model: model}
	}
	for i, f := range s.faces {
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			pc := make([]ExactBezierBoundaryUse, 0, len(loop))
			for _, u := range loop {
				if parts := splits[u.Edge]; len(parts) != 0 {
					uses, err := out.splitRationalKnotUse(ctx, f.Surface, u, parts, limits)
					if err != nil {
						return zero, fmt.Errorf("trimmed: exact Bézier face %d edge %d knot coedges: %w", i, u.Edge, err)
					}
					pc = append(pc, uses...)
					continue
				}
				uv, err := exactBoundaryUV(ctx, u, limits)
				if err != nil {
					return zero, fmt.Errorf("trimmed: exact Bézier pcurve on face %d edge %d: %w", i, u.Edge, err)
				}
				if !exactBoundaryEnd(uv.Control[0], geom.Vec3{X: u.UVStart.X, Y: u.UVStart.Y}) ||
					!exactBoundaryEnd(uv.Control[len(uv.Control)-1], geom.Vec3{X: u.UVEnd.X, Y: u.UVEnd.Y}) {
					return zero, fmt.Errorf("trimmed: exact Bézier UV endpoint on face %d edge %d: %w", i, u.Edge, geom.ErrConvergence)
				}
				if u.Reversed {
					slices.Reverse(uv.Control)
				}
				pc = append(pc, ExactBezierBoundaryUse{Edge: u.Edge, Reversed: u.Reversed, UV: uv})
			}
			out.Faces[i].Loops = append(out.Faces[i].Loops, pc)
		}
	}
	return out, nil
}

func exactBoundaryEnd(h [4]*big.Rat, p geom.Vec3) bool {
	for i, x := range []float64{p.X, p.Y, p.Z} {
		if new(big.Rat).Mul(ratInset(x), h[3]).Cmp(h[i]) != 0 {
			return false
		}
	}
	return true
}

func exactBoundaryModel(ctx context.Context, e parametric.Edge, o geom.SurfacePatchOptions) (geom.ExactBezierSpan, error) {
	if err := ctx.Err(); err != nil {
		return geom.ExactBezierSpan{}, err
	}
	if line, ok := e.Curve.(geom.Line); ok {
		a, d := line.Origin(), line.Direction()
		net := make([][4]*big.Rat, 2)
		for i, t := range []float64{e.Range.Min, e.Range.Max} {
			for j, x := range []float64{a.X, a.Y, a.Z} {
				v := [3]float64{d.X, d.Y, d.Z}[j]
				net[i][j] = new(big.Rat).Add(ratInset(x), new(big.Rat).Mul(ratInset(t), ratInset(v)))
			}
			net[i][3] = big.NewRat(1, 1)
		}
		return exactBoundarySpan(ctx, net, o)
	}
	if trim, ok := e.Curve.(surfaceTrimCurve); ok && trim.curve != nil && e.Range == (parametric.Interval{Max: 1}) {
		curve, err := ExactTrimModelBezier(ctx, trim.surface, trim.curve, false, o)
		if errors.Is(err, geom.ErrConvergence) {
			curve, err = exactBoundaryGraphBezier(ctx, trim.surface, trim.a, trim.b, trim.curve, o)
		}
		if err != nil {
			return geom.ExactBezierSpan{}, fmt.Errorf("trim %v to %v on %T: %w", trim.a, trim.b, trim.surface, err)
		}
		return normalizedBoundarySpan(ctx, curve, false, o)
	}
	curve, reverse, err := exactEdgeSplineWithOptions(e, o)
	var span geom.ExactBezierSpan
	if err == nil && curve == nil {
		err = geom.ErrDomain
	}
	if err == nil {
		span, err = normalizedBoundarySpan(ctx, curve, reverse, o)
	}
	if errors.Is(err, geom.ErrConvergence) {
		if trim, ok := e.Curve.(surfaceTrimCurve); ok && trim.curve == nil && e.Range == (parametric.Interval{Max: 1}) {
			curve, err = exactBoundaryGraphBezier(ctx, trim.surface, trim.a, trim.b, nil, o)
			if err == nil {
				span, err = normalizedBoundarySpan(ctx, curve, false, o)
			}
		}
	}
	if err != nil {
		return geom.ExactBezierSpan{}, fmt.Errorf("model %T in %v: %w", e.Curve, e.Range, err)
	}
	return span, nil
}

func exactBoundaryGraphBezier(ctx context.Context, support geom.Surface, a, b geom.Vec2, trim PCurve, o geom.SurfacePatchOptions) (*geom.ExactSplineCurve, error) {
	cells, err := exactBoundaryGraphCells(ctx, support, a, b, trim, geom.SurfaceCurveAgreementOptions{SurfacePatchOptions: o})
	if err != nil {
		return nil, err
	}
	curve, _, err := cells.SingleExactBezier(ctx, [2]*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1)},
		geom.ExactCurvePlaneEventOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	return curve, err
}

func exactBoundaryGraphCells(ctx context.Context, support geom.Surface, a, b geom.Vec2, trim PCurve, options geom.SurfaceCurveAgreementOptions) (geom.SurfaceCurveCompositionCells, error) {
	o := options.SurfacePatchOptions
	p, err := exactFaceSupportPatch(Face{Surface: support}, o)
	if err != nil {
		return geom.SurfaceCurveCompositionCells{}, err
	}
	var c *geom.ExactSplineCurve
	if trim == nil {
		c, err = geom.NewExactHomogeneousBezier(ctx, [][4]*big.Rat{
			{ratInset(a.X), ratInset(a.Y), new(big.Rat), big.NewRat(1, 1)},
			{ratInset(b.X), ratInset(b.Y), new(big.Rat), big.NewRat(1, 1)},
		}, o)
	} else {
		c, err = polynomialTrimDescriptor(ctx, trim, false, o)
	}
	if err != nil {
		return geom.SurfaceCurveCompositionCells{}, err
	}
	return p.BuildSurfaceCurveCompositionCells(ctx, c, options)
}

func exactBoundaryUV(ctx context.Context, u Use, o geom.SurfacePatchOptions) (geom.ExactBezierSpan, error) {
	var reverse bool
	c := u.Curve
	for {
		r, ok := c.(reverseTrim)
		if !ok {
			break
		}
		c, reverse = r.differentiableTrim, !reverse
	}
	var net [][4]*big.Rat
	controls := func(uv [][3]*big.Rat) {
		net = make([][4]*big.Rat, len(uv))
		for i, p := range uv {
			net[i] = [4]*big.Rat{p[0], p[1], new(big.Rat), p[2]}
		}
	}
	switch q := c.(type) {
	case nil:
		// Nil is already linear in loop direction.
		controls([][3]*big.Rat{{ratInset(u.UVStart.X), ratInset(u.UVStart.Y), big.NewRat(1, 1)}, {ratInset(u.UVEnd.X), ratInset(u.UVEnd.Y), big.NewRat(1, 1)}})
	case ExactBezierUV:
		controls(q.HomogeneousControls())
	case ExactQuadraticUV:
		p := q.HomogeneousControls()
		controls(p[:])
	case wallTrim:
		var err error
		net, err = exactBoundaryWallUV(ctx, q, o)
		if err != nil {
			return geom.ExactBezierSpan{}, err
		}
	case affineGraphWallTrim:
		var err error
		net, err = exactBoundaryWallUV(ctx, q.wallTrim, o)
		if err != nil {
			return geom.ExactBezierSpan{}, err
		}
	default:
		return geom.ExactBezierSpan{}, geom.ErrDomain
	}
	if reverse {
		slices.Reverse(net)
	}
	return exactBoundarySpan(ctx, net, o)
}

func exactBoundaryWallUV(ctx context.Context, w wallTrim, o geom.SurfacePatchOptions) ([][4]*big.Rat, error) {
	source, reverse, err := exactTrimIsoWithOptions(w.surface, w.a, w.b, nil, parametric.Interval{Max: 1}, geom.Identity(), o)
	if err != nil {
		return nil, err
	}
	if source == nil {
		return nil, geom.ErrDomain
	}
	span, err := normalizedBoundarySpan(ctx, source, reverse, o)
	if errors.Is(err, geom.ErrConvergence) {
		source, err = exactBoundaryGraphBezier(ctx, w.surface, w.a, w.b, nil, o)
		if err == nil {
			span, err = normalizedBoundarySpan(ctx, source, false, o)
		}
	}
	if err != nil {
		return nil, err
	}
	// The original chart is U=length*t and V=Z-height. Multiply U by
	// the source weight polynomial, and elevate the other coordinates.
	n := len(span.Control)
	z, length := ratInset(w.height), ratInset(w.length)
	constantWeight := true
	for _, p := range span.Control[1:] {
		constantWeight = constantWeight && p[3].Cmp(span.Control[0][3]) == 0
	}
	if constantWeight {
		net := make([][4]*big.Rat, n)
		for i, p := range span.Control {
			u := new(big.Rat).Mul(length, big.NewRat(int64(i), int64(n-1)))
			u.Mul(u, p[3])
			v := new(big.Rat).Sub(p[2], new(big.Rat).Mul(z, p[3]))
			net[i] = [4]*big.Rat{u, v, new(big.Rat), p[3]}
		}
		return net, nil
	}
	net := make([][4]*big.Rat, n+1)
	for i := range net {
		net[i] = [4]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat)}
		for offset := 0; offset < 2; offset++ {
			j := i - offset
			if j < 0 || j >= n {
				continue
			}
			a := big.NewRat(int64([2]int{n - i, i}[offset]), int64(n))
			p := span.Control[j]
			weight := new(big.Rat).Mul(a, p[3])
			net[i][3].Add(net[i][3], weight)
			if offset == 1 {
				net[i][0].Mul(weight, length)
			}
			v := new(big.Rat).Sub(p[2], new(big.Rat).Mul(z, p[3]))
			net[i][1].Add(net[i][1], v.Mul(v, a))
		}
	}
	return net, nil
}

func normalizedBoundarySpan(ctx context.Context, c *geom.ExactSplineCurve, reverse bool, o geom.SurfacePatchOptions) (geom.ExactBezierSpan, error) {
	span, _, err := c.BezierControlNet(ctx, o)
	if err != nil {
		return geom.ExactBezierSpan{}, err
	}
	span.Start, span.End = 0, 1
	if reverse {
		slices.Reverse(span.Control)
	}
	return span, nil
}

func exactBoundarySpan(ctx context.Context, net [][4]*big.Rat, o geom.SurfacePatchOptions) (geom.ExactBezierSpan, error) {
	c, err := geom.NewExactHomogeneousBezier(ctx, net, o)
	if err != nil {
		return geom.ExactBezierSpan{}, err
	}
	return normalizedBoundarySpan(ctx, c, false, o)
}
