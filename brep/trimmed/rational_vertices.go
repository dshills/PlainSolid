package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// RationalVertex carries a model point proved from native rational trim
// endpoints. Display is its nearest finite float64 representative; topology
// uses the index, and exact geometric checks use Point.
type RationalVertex struct {
	Vertex  int
	Point   [3]*big.Rat
	Display geom.Vec3
}

// RationalVertices reconstructs exact model points for every endpoint of a
// rational-endpoint model edge. The returned coordinates are caller-owned.
func (s Solid) RationalVertices(ctx context.Context) ([]RationalVertex, error) {
	return collectRationalVertices(ctx, s.vertices, s.edges, true)
}

func exactQuadraticModelEndpoint(ctx context.Context, m surfaceTrimCurve, q ExactQuadraticUV, side int, o geom.SurfacePatchOptions) ([3]*big.Rat, error) {
	var empty [3]*big.Rat
	p, e := exactFaceSupportPatch(Face{Surface: m.surface}, o)
	if e != nil {
		return empty, e
	}
	uv, _ := q.Endpoints()
	j, e := p.EvaluateRational(ctx, uv[side][0], uv[side][1], geom.SurfacePatchEvaluationOptions{
		MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
	})
	return j.Point, e
}

func rationalDisplayPoint(p [3]*big.Rat) (geom.Vec3, error) {
	var v geom.Vec3
	v.X, _ = p[0].Float64()
	v.Y, _ = p[1].Float64()
	v.Z, _ = p[2].Float64()
	if !v.IsFinite() {
		return geom.Vec3{}, geom.ErrUnrepresentable
	}
	return v, nil
}

func graphTrimStartDisplay(ctx context.Context, s *geom.BSplineSurface, x openSectionPiece) (geom.Vec3, error) {
	q, ok := x.c.(ExactQuadraticUV)
	if !ok || !q.rationalEndpoints {
		return s.Eval(x.a.X, x.a.Y)
	}
	p, err := exactQuadraticModelEndpoint(ctx, surfaceTrimCurve{s, x.a, x.b, x.c}, q, 0, geom.SurfacePatchOptions{})
	if err != nil {
		return geom.Vec3{}, err
	}
	return rationalDisplayPoint(p)
}

func sameRationalVertex(a, b [3]*big.Rat) bool {
	return a[0].Cmp(b[0]) == 0 && a[1].Cmp(b[1]) == 0 && a[2].Cmp(b[2]) == 0
}

func usesRationalEndpoints(s Solid) bool {
	for _, e := range s.edges {
		if m, ok := e.Curve.(surfaceTrimCurve); ok {
			if q, ok := m.curve.(ExactQuadraticUV); ok && q.rationalEndpoints {
				return true
			}
		}
	}
	for _, f := range s.faces {
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				if q, ok := u.Curve.(ExactQuadraticUV); ok && q.rationalEndpoints {
					return true
				}
			}
		}
	}
	return false
}

func collectRationalVertices(ctx context.Context, v []geom.Vec3, edges []parametric.Edge, checkDisplay bool) ([]RationalVertex, error) {
	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	points := make(map[int][3]*big.Rat)
	count := 0
	for _, e := range edges {
		m, ok := e.Curve.(surfaceTrimCurve)
		q, qok := m.curve.(ExactQuadraticUV)
		if !ok || !qok || !q.rationalEndpoints {
			continue
		}
		if e.Range != (parametric.Interval{Max: 1}) {
			return nil, geom.ErrConvergence
		}
		count++
		if count > 256 {
			return nil, geom.ErrConvergence
		}
		for side, id := range [2]int{e.Start, e.End} {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			p, err := exactQuadraticModelEndpoint(ctx, m, q, side, geom.SurfacePatchOptions{})
			if err != nil {
				return nil, err
			}
			if old, ok := points[id]; ok && !sameRationalVertex(old, p) {
				return nil, geom.ErrConvergence
			}
			points[id] = p
		}
	}
	if len(points) == 0 {
		return nil, nil
	}
	// A nonbinary vertex cannot rely on a neighbor that only provides
	// tolerance-based endpoint agreement. The initial native subset keeps
	// such junctions among supported exact rational trim model edges.
	for _, e := range edges {
		for side, id := range [2]int{e.Start, e.End} {
			p, ok := points[id]
			if !ok {
				continue
			}
			m, supported := e.Curve.(surfaceTrimCurve)
			q, qok := m.curve.(ExactQuadraticUV)
			if supported && qok && q.rationalEndpoints {
				continue
			}
			if !supported || !qok || e.Range != (parametric.Interval{Max: 1}) {
				if _, err := geom.RepresentedExactPoint(p); err == nil {
					continue
				}
				return nil, geom.ErrConvergence
			}
			other, err := exactQuadraticModelEndpoint(ctx, m, q, side, geom.SurfacePatchOptions{})
			if err != nil || !sameRationalVertex(other, p) {
				if err == nil {
					err = geom.ErrConvergence
				}
				return nil, err
			}
		}
	}
	out := make([]RationalVertex, 0, len(points))
	for id, display := range v {
		if p, ok := points[id]; ok {
			want, err := rationalDisplayPoint(p)
			if err != nil {
				return nil, err
			}
			if checkDisplay && display != want {
				return nil, geom.ErrConvergence
			}
			out = append(out, RationalVertex{id, p, want})
		}
	}
	return out, nil
}
