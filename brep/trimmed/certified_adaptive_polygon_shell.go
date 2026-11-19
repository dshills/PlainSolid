//go:build research

package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// CertifiedAdaptivePolygonShellOptions specifies global triangle-check and
// output budgets. MaxRounds defaults to 8 (maximum 16); each round bisects all
// edges of failing triangles and any incident neighboring triangle. Arithmetic
// and edge-proof options retain their individual limits.
type CertifiedAdaptivePolygonShellOptions struct {
	Deflection                                 float64
	MaxRounds, MaxTriangleChecks, MaxTriangles int
	Evaluation                                 geom.SurfacePatchEvaluationOptions
	Edges                                      edgecheck.Options
}

// CertifiedAdaptivePolygonShellMesh carries the same triangle certificates and
// original face ancestry as the uniform shell API. Subdivisions is zero because
// this mesh has no uniform level. Rounds counts global edge refinement passes.
type CertifiedAdaptivePolygonShellMesh struct {
	CertifiedPolygonShellMesh
	Rounds int
}

// TessellateAdaptivePolygonShellCertified retains exact continuous model-edge
// agreement, but refines only failed triangles and their shared edges. Interior
// and boundary samples are shared by mesh edge identity. Green neighbor splits
// eliminate T junctions; all resulting triangles are independently certified.
// Only straight UV trims on supported spline faces are accepted. This proves
// conforming closed topology and stated deviation, not smooth/mesh embedding.
func (s Solid) TessellateAdaptivePolygonShellCertified(ctx context.Context, o CertifiedAdaptivePolygonShellOptions) (CertifiedAdaptivePolygonShellMesh, error) {
	empty := CertifiedAdaptivePolygonShellMesh{}
	if ctx == nil || !finite(o.Deflection) || o.Deflection <= 0 {
		return empty, geom.ErrInvalid
	}
	if o.MaxRounds == 0 {
		o.MaxRounds = 8
	}
	if o.MaxTriangleChecks == 0 {
		o.MaxTriangleChecks = 100000
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 1000000
	}
	if o.MaxRounds < 1 || o.MaxRounds > 16 || o.MaxTriangleChecks < 1 || o.MaxTriangles < 1 || o.MaxTriangles > 8000000 {
		return empty, geom.ErrInvalid
	}
	minimum := 0
	for _, f := range s.faces {
		count := len(f.Outer) + 2*len(f.Holes) - 2
		for _, h := range f.Holes {
			count += len(h)
		}
		if count < 1 || count > o.MaxTriangleChecks-minimum {
			return empty, geom.ErrConvergence
		}
		minimum += count
	}
	seed, err := s.TessellatePolygonShellCertified(ctx, CertifiedPolygonShellOptions{Face: CertifiedPatchMeshOptions{Deflection: math.MaxFloat64, MaxSubdivisions: 1, MaxTriangleChecks: o.MaxTriangleChecks, Evaluation: o.Evaluation}, Edges: o.Edges, MaxTriangles: o.MaxTriangles})
	if err != nil {
		return empty, err
	}
	if seed.TriangleChecks > o.MaxTriangleChecks {
		return empty, geom.ErrConvergence
	}
	patches := make([]*geom.SurfacePatch, len(s.faces))
	for i, f := range s.faces {
		base, m := parametric.SurfaceMapping(f.Surface)
		switch v := base.(type) {
		case *geom.BSplineSurface:
			patches[i], err = geom.NewSurfacePatch(v, geom.SurfacePatchOptions{})
		case *geom.WrappedSurfacePatch:
			patches[i] = v.FundamentalPatch()
		default:
			return empty, geom.ErrConvergence
		}
		if err != nil {
			return empty, err
		}
		if m != geom.Identity() {
			patches[i], err = patches[i].Transformed(m)
			if err != nil {
				return empty, err
			}
		}
	}
	result := CertifiedAdaptivePolygonShellMesh{CertifiedPolygonShellMesh: seed}
	result.Subdivisions = 0
	tolerance := ratInset(o.Deflection)
	tolerance.Mul(tolerance, tolerance)
	type edge [2]int
	key := func(a, b int) edge {
		if a > b {
			a, b = b, a
		}
		return edge{a, b}
	}
	midpoint := func(a, b geom.Vec2) (geom.Vec2, error) {
		var out [2]float64
		for i, pair := range [][2]float64{{a.X, b.X}, {a.Y, b.Y}} {
			r := new(big.Rat).Quo(new(big.Rat).Add(ratInset(pair[0]), ratInset(pair[1])), big.NewRat(2, 1))
			f, _ := r.Float64()
			if !finite(f) || ratInset(f).Cmp(r) != 0 {
				return geom.Vec2{}, geom.ErrConvergence
			}
			out[i] = f
		}
		return geom.Vec2{X: out[0], Y: out[1]}, nil
	}
	for {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		marked := map[edge]bool{}
		for i, b := range result.SquaredErrors {
			if b == nil || b.Cmp(tolerance) > 0 {
				t := result.Mesh.Triangles[i]
				for j, a := range t {
					marked[key(a, t[(j+1)%3])] = true
				}
			}
		}
		if len(marked) == 0 {
			if err := result.Mesh.ValidateSolid(); err != nil {
				return empty, err
			}
			return result, nil
		}
		if result.Rounds == o.MaxRounds {
			return empty, geom.ErrConvergence
		}
		// Each marked edge increases each incident triangle's count by one.
		count := 0
		for _, t := range result.Mesh.Triangles {
			count++
			for j, a := range t {
				if marked[key(a, t[(j+1)%3])] {
					count++
				}
			}
			if count > o.MaxTriangles {
				return empty, geom.ErrConvergence
			}
		}
		if count > o.MaxTriangleChecks-result.TriangleChecks {
			return empty, geom.ErrConvergence
		}
		next := CertifiedAdaptivePolygonShellMesh{Rounds: result.Rounds + 1, CertifiedPolygonShellMesh: CertifiedPolygonShellMesh{BothDirections: true, TriangleChecks: result.TriangleChecks}}
		next.Mesh.Vertices = append(next.Mesh.Vertices, result.Mesh.Vertices...)
		mids := map[edge]int{}
		for ti, t := range result.Mesh.Triangles {
			face := result.OriginalFace[ti]
			uv := result.UVTriangles[ti]
			type triangle struct {
				ids [3]int
				uv  [3]geom.Vec2
			}
			pieces := []triangle{{t, uv}}
			// Ordered original edges provide deterministic green triangulation. All
			// adjacent faces still split precisely the same shared edge intervals.
			for j, a := range t {
				b := t[(j+1)%3]
				k := key(a, b)
				if !marked[k] {
					continue
				}
				midUV, e := midpoint(uv[j], uv[(j+1)%3])
				if e != nil {
					return empty, e
				}
				jet, e := patches[face].EvaluateRational(ctx, ratInset(midUV.X), ratInset(midUV.Y), o.Evaluation)
				if e != nil {
					return empty, e
				}
				x, _ := jet.Point[0].Float64()
				y, _ := jet.Point[1].Float64()
				z, _ := jet.Point[2].Float64()
				point := geom.Vec3{X: x, Y: y, Z: z}
				if !point.IsFinite() {
					return empty, geom.ErrConvergence
				}
				midID, ok := mids[k]
				if ok {
					if next.Mesh.Vertices[midID] != point {
						return empty, geom.ErrConvergence
					}
				} else {
					midID = len(next.Mesh.Vertices)
					mids[k] = midID
					next.Mesh.Vertices = append(next.Mesh.Vertices, point)
				}
				found := false
				for pi, p := range pieces {
					for side, start := range p.ids {
						end := p.ids[(side+1)%3]
						if key(start, end) != k {
							continue
						}
						other := (side + 2) % 3
						one := triangle{[3]int{start, midID, p.ids[other]}, [3]geom.Vec2{p.uv[side], midUV, p.uv[other]}}
						two := triangle{[3]int{midID, end, p.ids[other]}, [3]geom.Vec2{midUV, p.uv[(side+1)%3], p.uv[other]}}
						pieces[pi] = one
						pieces = append(pieces, two)
						found = true
						break
					}
					if found {
						break
					}
				}
				if !found {
					return empty, geom.ErrConvergence
				}
			}

			// The all-red case uses the central midpoint triangle. Sequential
			// bisection would introduce long diagonals and degrade triangle quality.
			if marked[key(t[0], t[1])] && marked[key(t[1], t[2])] && marked[key(t[2], t[0])] {
				a, _ := midpoint(uv[0], uv[1])
				b, _ := midpoint(uv[1], uv[2])
				c, _ := midpoint(uv[2], uv[0])
				ai, bi, ci := mids[key(t[0], t[1])], mids[key(t[1], t[2])], mids[key(t[2], t[0])]
				pieces = []triangle{{[3]int{t[0], ai, ci}, [3]geom.Vec2{uv[0], a, c}}, {[3]int{ai, t[1], bi}, [3]geom.Vec2{a, uv[1], b}}, {[3]int{ci, bi, t[2]}, [3]geom.Vec2{c, b, uv[2]}}, {[3]int{ai, bi, ci}, [3]geom.Vec2{a, b, c}}}
			}
			for _, p := range pieces {
				model := [3]geom.Vec3{next.Mesh.Vertices[p.ids[0]], next.Mesh.Vertices[p.ids[1]], next.Mesh.Vertices[p.ids[2]]}
				bound, e := BoundSplineTriangleDeviation(ctx, patches[face], p.uv, model)
				if e != nil && !errors.Is(e, geom.ErrDegenerate) {
					return empty, fmt.Errorf("face %d triangle %v: %w", face, p.uv, e)
				}
				if errors.Is(e, geom.ErrDegenerate) {
					bound.SquaredDistance = nil
					bound.BothDirections = true
				}
				next.TriangleChecks++
				next.BothDirections = next.BothDirections && bound.BothDirections
				next.Mesh.Triangles = append(next.Mesh.Triangles, p.ids)
				next.UVTriangles = append(next.UVTriangles, p.uv)
				next.OriginalFace = append(next.OriginalFace, face)
				next.SquaredErrors = append(next.SquaredErrors, bound.SquaredDistance)
			}
		}
		result = next
	}
}
