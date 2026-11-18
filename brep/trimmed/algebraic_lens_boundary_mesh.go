//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

type AlgebraicLensBoundaryOptions struct {
	Display                                     geom.ComposedCurvePolylineOptions
	Proof                                       AlgebraicSeamOptions
	UV                                          geom.ComposedCurveEndpointOptions // Per-sample proof limits; default tolerance1e-9.
	MaxUVSamples, MaxCapVertices, MaxPredicates int
}

// AlgebraicLensUVSample retains exact rational UV or an enclosure with immutable
// original parameter root. The displayed UV is NOT silently substituted for
// its source point when composing surfaces.
type AlgebraicLensUVSample struct {
	Exact   [2]*big.Rat
	Bounds  [2]geom.ExactParameterInterval
	Display geom.Vec2
	Root    *geom.ExactAlgebraicRoot
}

type AlgebraicLensBoundaryOwner struct {
	Side, Face, OriginalFace, Edge int // Face=-1 and OriginalFace=-1 for a cap use.
	Cap, Reverse                   bool
	Vertices                       []int
	UV                             []AlgebraicLensUVSample // Source support UV; cap uses retain curve identity instead.
}

// CertifiedAlgebraicLensBoundary assigns shared model samples BEFORE face
// tessellation. Each half has closed opposite edge incidence. This is not a
// closed triangle mesh or an embedding certificate. Model samples carry curve
// chord certificates; UV values retain independent exact source provenance.
type CertifiedAlgebraicLensBoundary struct {
	Vertices         []geom.Vec3
	TopologyVertices []int
	Edges            [][]int // Arrangement edge order, increasing source model parameter.
	Owners           []AlgebraicLensBoundaryOwner
	Rim              CertifiedAlgebraicBoundaryMesh
	Cap              CertifiedAlgebraicCapMesh
	UVSamples        int
}

func (s AlgebraicLensPlaneSplit) TessellateSharedBoundaryCertified(ctx context.Context, o AlgebraicLensBoundaryOptions) (CertifiedAlgebraicLensBoundary, error) {
	var empty CertifiedAlgebraicLensBoundary
	if ctx == nil {
		return empty, geom.ErrInvalid
	}
	if o.Display.MaxOperations == 0 {
		o.Display.MaxOperations = 8000000
	}
	if o.Display.MaxSegments == 0 {
		o.Display.MaxSegments = 4096
	}
	if o.MaxUVSamples == 0 {
		o.MaxUVSamples = 16384
	}
	if o.MaxUVSamples < 1 || o.MaxUVSamples > 1000000 {
		return empty, geom.ErrInvalid
	}
	cap, e := s.TessellateCapCertified(ctx, -1, AlgebraicCapMeshOptions{Display: o.Display, MaxVertices: o.MaxCapVertices, MaxPredicates: o.MaxPredicates})
	if e != nil {
		return empty, e
	}
	// This local copy chooses the already recertified cap endpoint displays.
	// All original rim owners independently certify those same points below.
	sub := s.source.subdivision
	sub.vertices = sub.Vertices()
	for i, id := range cap.TopologyVertices {
		sub.vertices[id].Display = cap.EndpointDisplays[i]
	}
	display := o.Display
	display.MaxOperations -= cap.ExactOperations
	for _, curve := range cap.Curves {
		display.MaxSegments -= len(curve.Segments)
	}
	if display.MaxOperations < 1 || display.MaxSegments < 1 {
		return empty, geom.ErrConvergence
	}
	rim, e := sub.TessellateBoundaryCertified(ctx, AlgebraicBoundaryMeshOptions{Proof: o.Proof, Display: display})
	if e != nil {
		return empty, e
	}
	result := CertifiedAlgebraicLensBoundary{Vertices: append([]geom.Vec3(nil), rim.Vertices...), TopologyVertices: append([]int(nil), rim.TopologyVertices...), Edges: make([][]int, len(s.source.edges)), Rim: rim, Cap: cap}
	seen := map[geom.Vec3]bool{}
	for _, p := range result.Vertices {
		seen[p] = true
	}
	for i, edge := range rim.Edges {
		result.Edges[i] = append([]int(nil), edge...)
	}
	for ci, path := range cap.CurveVertices {
		ids := make([]int, len(path))
		ids[0] = rim.TopologyVertices[cap.TopologyVertices[0]]
		ids[len(ids)-1] = rim.TopologyVertices[cap.TopologyVertices[1]]
		for j := 1; j < len(ids)-1; j++ {
			p := cap.Mesh.Vertices[path[j]]
			if seen[p] {
				return empty, geom.ErrDegenerate
			}
			seen[p] = true
			ids[j] = len(result.Vertices)
			result.Vertices = append(result.Vertices, p)
		}
		result.Edges[cap.SourceEdges[ci]] = ids
	}
	type key struct{ edge, face int }
	cache := map[key][]AlgebraicLensUVSample{}
	sampleRim := func(edge, face int) ([]AlgebraicLensUVSample, error) {
		chart, e := sub.CoedgeChart(ctx, edge, face, o.Proof)
		if e != nil {
			return nil, e
		}
		poly := rim.Certificates[edge]
		roots := map[int]*geom.ExactAlgebraicRoot{}
		for _, j := range poly.Junctions {
			roots[j.Vertex] = j.Root
		}
		samples := make([]AlgebraicLensUVSample, len(poly.Vertices))
		for i, v := range poly.Vertices {
			if result.UVSamples >= o.MaxUVSamples {
				return nil, geom.ErrConvergence
			}
			result.UVSamples++
			if root := roots[i]; root != nil {
				event, e := chart.UV.BindRoot(ctx, root, o.UV.ExactCurvePlaneEventOptions)
				if e != nil {
					return nil, e
				}
				bounds, e := event.Enclose(ctx, o.UV)
				if e != nil {
					return nil, e
				}
				if !bounds.Converged || !bounds.HasRepresentative {
					return nil, geom.ErrConvergence
				}
				samples[i] = AlgebraicLensUVSample{Bounds: [2]geom.ExactParameterInterval{bounds.Coordinates[0], bounds.Coordinates[1]}, Display: geom.Vec2{X: bounds.Point.X, Y: bounds.Point.Y}, Root: root}
			} else {
				if v.Parameter.Lower.Cmp(v.Parameter.Upper) != 0 {
					return nil, geom.ErrConvergence
				}
				point, e := chart.UV.EvaluateRational(ctx, v.Parameter.Lower, o.UV.ExactCurvePlaneEventOptions)
				if e != nil {
					return nil, e
				}
				for axis := 0; axis < 2; axis++ {
					q := point.Point[axis]
					samples[i].Exact[axis] = new(big.Rat).Set(q)
					samples[i].Bounds[axis] = geom.ExactParameterInterval{Lower: new(big.Rat).Set(q), Upper: new(big.Rat).Set(q)}
				}
				samples[i].Display.X, _ = point.Point[0].Float64()
				samples[i].Display.Y, _ = point.Point[1].Float64()
			}
		}
		return samples, nil
	}
	sampleCut := func(ci int) ([]AlgebraicLensUVSample, error) {
		curve := cap.SourceCurves[ci]
		poly := cap.Curves[ci]
		samples := make([]AlgebraicLensUVSample, len(poly.Vertices))
		roots := map[int]*geom.ExactAlgebraicRoot{}
		for _, j := range poly.Junctions {
			roots[j.Vertex] = j.Root
		}
		axis, other := curve.Axis, curve.Axis^1
		line := curve.Partition.Line
		for i, v := range poly.Vertices {
			if result.UVSamples >= o.MaxUVSamples {
				return nil, geom.ErrConvergence
			}
			result.UVSamples++
			lo, hi := v.Parameter.Lower, v.Parameter.Upper
			if roots[i] != nil {
				junction := -1
				for j, x := range poly.Junctions {
					if x.Vertex == i {
						junction = j
					}
				}
				if junction < 0 || len(poly.Junctions) != len(curve.Model.Cells)+1 {
					return nil, geom.ErrConvergence
				}
				cell, endpoint := junction, 0
				if cell == len(curve.Model.Cells) {
					cell--
					endpoint = 1
				}
				refined, e := curve.Model.Cells[cell].EncloseEndpoint(ctx, endpoint, o.UV)
				if e != nil {
					return nil, e
				}
				if !refined.Converged {
					return nil, geom.ErrConvergence
				}
				lo, hi = refined.Parameter.Lower, refined.Parameter.Upper
			}
			samples[i].Root = roots[i]
			samples[i].Bounds[axis] = geom.ExactParameterInterval{Lower: new(big.Rat).Set(lo), Upper: new(big.Rat).Set(hi)}
			a := new(big.Rat).Neg(new(big.Rat).Quo(new(big.Rat).Add(line[2], new(big.Rat).Mul(line[axis], lo)), line[other]))
			b := new(big.Rat).Neg(new(big.Rat).Quo(new(big.Rat).Add(line[2], new(big.Rat).Mul(line[axis], hi)), line[other]))
			if a.Cmp(b) > 0 {
				a, b = b, a
			}
			samples[i].Bounds[other] = geom.ExactParameterInterval{Lower: a, Upper: b}
			var display [2]float64
			for j := 0; j < 2; j++ {
				interval := samples[i].Bounds[j]
				mid := new(big.Rat).Quo(new(big.Rat).Add(interval.Lower, interval.Upper), big.NewRat(2, 1))
				display[j], _ = mid.Float64()
				if interval.Lower.Cmp(interval.Upper) == 0 {
					samples[i].Exact[j] = new(big.Rat).Set(interval.Lower)
				}
			}
			samples[i].Display = geom.Vec2{X: display[0], Y: display[1]}
		}
		return samples, nil
	}
	for _, side := range []int{-1, 1} {
		half, e := s.Half(side)
		if e != nil {
			return empty, e
		}
		counts := make([]int, len(result.Edges))
		orientation := make([]int, len(result.Edges))
		for _, fi := range half.Fragments {
			if fi < 0 || fi >= len(s.source.faces) {
				return empty, geom.ErrInvalid
			}
			face := s.source.faces[fi]
			if len(face.Holes) != 0 {
				return empty, geom.ErrConvergence
			}
			for _, use := range face.Outer {
				edge := s.source.edges[use.Edge]
				k := key{use.Edge, face.Original}
				uv, ok := cache[k]
				if !ok {
					if edge.Boundary >= 0 {
						uv, e = sampleRim(edge.Boundary, face.Original)
					} else {
						ci := -1
						for j, id := range cap.SourceEdges {
							if id == use.Edge {
								ci = j
							}
						}
						if ci < 0 {
							return empty, geom.ErrConvergence
						}
						uv, e = sampleCut(ci)
					}
					if e != nil {
						return empty, e
					}
					cache[k] = uv
				}
				reverse := use.Reversed != face.Reversed
				owner := AlgebraicLensBoundaryOwner{Side: side, Face: fi, OriginalFace: face.Original, Edge: use.Edge, Reverse: reverse, Vertices: append([]int(nil), result.Edges[use.Edge]...), UV: append([]AlgebraicLensUVSample(nil), uv...)}
				if reverse {
					for a, b := 0, len(owner.Vertices)-1; a < b; a, b = a+1, b-1 {
						owner.Vertices[a], owner.Vertices[b] = owner.Vertices[b], owner.Vertices[a]
						owner.UV[a], owner.UV[b] = owner.UV[b], owner.UV[a]
					}
				}
				result.Owners = append(result.Owners, owner)
				counts[use.Edge]++
				if reverse {
					orientation[use.Edge]--
				} else {
					orientation[use.Edge]++
				}
			}
		}
		for ci, id := range half.CapEdges {
			reverse := half.CapReversed[ci]
			owner := AlgebraicLensBoundaryOwner{Side: side, Face: -1, OriginalFace: -1, Edge: id, Cap: true, Reverse: reverse, Vertices: append([]int(nil), result.Edges[id]...)}
			if reverse {
				for a, b := 0, len(owner.Vertices)-1; a < b; a, b = a+1, b-1 {
					owner.Vertices[a], owner.Vertices[b] = owner.Vertices[b], owner.Vertices[a]
				}
			}
			result.Owners = append(result.Owners, owner)
			counts[id]++
			if reverse {
				orientation[id]--
			} else {
				orientation[id]++
			}
		}
		for i, count := range counts {
			if count != 0 && (count != 2 || orientation[i] != 0) {
				return empty, geom.ErrConvergence
			}
		}
	}
	return result, nil
}
