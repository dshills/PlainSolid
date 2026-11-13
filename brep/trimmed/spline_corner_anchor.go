package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// SplineCornerAnchor binds an anchored tube to two original model edges and an
// existing vertex. It is pending fan evidence, not an assembled shell section.
type SplineCornerAnchor struct {
	solids        [2]Solid
	pair          [2]int
	chart, vertex int
	edges         [2]int
	edgeEnds      [2]int
	tube          geom.VerifiedSurfaceTube
	endpoint      *geom.TubeCornerEndpoint
}

func (a *SplineCornerAnchor) FacePair() [2]int               { return a.pair }
func (a *SplineCornerAnchor) Operand() int                   { return a.chart }
func (a *SplineCornerAnchor) Edges() [2]int                  { return a.edges }
func (a *SplineCornerAnchor) Vertex() int                    { return a.vertex }
func (a *SplineCornerAnchor) Tube() geom.VerifiedSurfaceTube { return a.tube }
func (a *SplineCornerAnchor) Point() [3]*big.Rat             { return a.endpoint.Point() }
func (a *SplineCornerAnchor) PendingFan() bool               { return a != nil && a.endpoint != nil }

// EdgeEnds identifies original model-edge Start (0) or End (1), in Sides order.
func (a *SplineCornerAnchor) EdgeEnds() [2]int { return a.edgeEnds }

// CertifySplineFaceCornerAnchor constructs its tube from the original faces,
// so no caller-supplied endpoint or surface identity is trusted. Exact rectangle
// edge agreement is re-proved using splineEmbeddingFace. The proposed local
// boxes must remain inside both complete faces; one original chart owns a
// corner and the other is strictly interior. No edge is split or rewritten.
func CertifySplineFaceCornerAnchor(ctx context.Context, a, b Solid, pair [2]int, boxes [2]geom.ParameterRectangle, chart int,
	boundary geom.SurfaceTubeBoundaryOptions, o SplineRectangleEmbeddingOptions) (*SplineCornerAnchor, error) {
	if ctx == nil || chart < 0 || chart > 1 {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	out := &SplineCornerAnchor{solids: [2]Solid{a, b}, pair: pair, chart: chart}
	var faces [2]splineEmbeddingFace
	var local [2]*geom.SurfacePatch
	var domains [2]geom.ParameterRectangle
	for i, s := range out.solids {
		if pair[i] < 0 || pair[i] >= len(s.faces) {
			return nil, geom.ErrInvalid
		}
		f, _, e := s.splineEmbeddingFace(ctx, s.faces[pair[i]], o)
		if e != nil {
			return nil, e
		}
		faces[i] = f
		domains[i] = f.patch.Domain()
		local[i], e = f.patch.Trim(boxes[i])
		if e != nil {
			return nil, e
		}
	}
	tube, e := geom.NewBoundaryAnchoredSurfaceTube(ctx, local[0], local[1], boundary)
	if e != nil {
		return nil, e
	}
	endpoint, e := tube.CertifyCornerEndpoint(ctx, boundary.AtUpper, chart, domains, geom.TubeEndpointOptions{MaxOperations: boundary.MaxOperations, MaxCoefficientBits: boundary.MaxCoefficientBits})
	if e != nil {
		return nil, e
	}
	out.tube, out.endpoint = tube, endpoint
	for i, side := range endpoint.Sides() {
		out.edges[i] = faces[chart].edge[side]
	}
	if out.edges[0] == out.edges[1] {
		return nil, geom.ErrConvergence
	}
	uv := endpoint.Parameters()
	d := domains[chart]
	corners := [4][2]float64{{d.UMin, d.VMin}, {d.UMax, d.VMin}, {d.UMax, d.VMax}, {d.UMin, d.VMax}}
	found := -1
	for i, c := range corners {
		if uv[2*chart].Cmp(new(big.Rat).SetFloat64(c[0])) == 0 && uv[2*chart+1].Cmp(new(big.Rat).SetFloat64(c[1])) == 0 {
			found = i
		}
	}
	if found < 0 {
		return nil, geom.ErrConvergence
	}
	out.vertex = faces[chart].vertex[found]
	stock := out.solids[chart]
	p := stock.vertices[out.vertex]
	model := endpoint.Point()
	for i, x := range []float64{p.X, p.Y, p.Z} {
		if model[i].Cmp(new(big.Rat).SetFloat64(x)) != 0 {
			return nil, geom.ErrConvergence
		}
	}
	for index, id := range out.edges {
		edge := stock.edges[id]
		if edge.Start != out.vertex && edge.End != out.vertex {
			return nil, geom.ErrConvergence
		}
		if edge.Start == edge.End {
			return nil, geom.ErrConvergence
		}
		if edge.End == out.vertex {
			out.edgeEnds[index] = 1
		}
		matched := false
		for _, use := range stock.faces[pair[chart]].Outer {
			if use.Edge != id {
				continue
			}
			corner := geom.Vec2{X: corners[found][0], Y: corners[found][1]}
			atEnd := false
			if use.UVStart == corner {
				atEnd = use.Reversed
			} else if use.UVEnd == corner {
				atEnd = !use.Reversed
			} else {
				return nil, geom.ErrConvergence
			}
			expected := 0
			if atEnd {
				expected = 1
			}
			if expected != out.edgeEnds[index] {
				return nil, geom.ErrConvergence
			}
			matched = true
		}
		if !matched {
			return nil, geom.ErrConvergence
		}
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	return out, nil
}
