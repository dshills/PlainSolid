package trimmed

import (
	"context"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
	"slices"
)

// CertifiedWrappedStationBoreMesh retains independently partitioned original
// outer/inner station families. OriginalFace always indexes the input Solid.
// StationVertices[wall][station] shares exact samples only within that family;
// no artificial station is inserted into the opposite wall. Source's station
// subproof edge IDs refer to reconstructed capped shells, as documented by
// WrappedStationBoreProof. SquaredErrors are two-sided surface/display bounds,
// not material-distance or volume-error bounds.
type CertifiedWrappedStationBoreMesh struct {
	Mesh                                            mesh.Mesh
	Source                                          WrappedStationBoreProof
	OriginalFace                                    []int
	StationVertices                                 [2][][]int
	SquaredErrors                                   []*big.Rat
	SourceImageSquaredDistance, SignedDisplayVolume *big.Rat
	Collision                                       collision.Report
	Subdivisions, TriangleChecks, StationSamples    int
}

// TessellateWrappedStationBoreCertified validates the entire original material
// topology and strict nesting at all union station heights. Wall station sets
// may differ. End profiles must have matching U knot bases and proportional
// rational weights, enabling exact common radial cap charts. Shared angular
// refinement is synchronized globally; cap residuals can request finer walls.
// Options share triangle/sample budgets across families and retry passes;
// source validation and contact checks retain their separately bounded budgets.
func (s Solid) TessellateWrappedStationBoreCertified(ctx context.Context, o CertifiedWrappedStationMeshOptions) (out CertifiedWrappedStationBoreMesh, err error) {
	return s.tessellateWrappedProfileStationBore(ctx, o, nil, true)
}

// TessellateWrappedRationalStationBoreCertified certifies the two original
// noncircular wall families and their exact annular cap charts.
func (s Solid) TessellateWrappedRationalStationBoreCertified(ctx context.Context, o CertifiedWrappedStationMeshOptions) (CertifiedWrappedStationBoreMesh, error) {
	return s.tessellateWrappedProfileStationBore(ctx, o, nil, false)
}

func (s Solid) tessellateWrappedProfileStationBore(ctx context.Context, o CertifiedWrappedStationMeshOptions, capture *[]wrappedDisplayBand, circular bool) (out CertifiedWrappedStationBoreMesh, err error) {
	if capture != nil {
		*capture = nil
		defer func() {
			if err != nil {
				*capture = nil
			}
		}()
	}
	defer func() {
		if err != nil {
			out = CertifiedWrappedStationBoreMesh{}
		}
	}()
	if ctx == nil {
		return out, geom.ErrInvalid
	}
	if o.Band.MaxTriangles == 0 {
		o.Band.MaxTriangles = 1000000
	}
	if o.Band.Patch.MaxTriangleChecks == 0 {
		o.Band.Patch.MaxTriangleChecks = 100000
	}
	if o.Band.Patch.MaxSubdivisions == 0 {
		o.Band.Patch.MaxSubdivisions = 32
	}
	if o.MaxStationSamples == 0 {
		o.MaxStationSamples = 65536
	}
	if o.Band.MaxTriangles < 1 || o.Band.MaxTriangles > 8000000 || o.Band.Patch.MaxTriangleChecks < 1 || o.MaxStationSamples < 1 || o.MaxStationSamples > 1000000 || o.Band.Patch.MaxSubdivisions < 1 || o.Band.Patch.MaxSubdivisions > 128 || o.Band.Patch.MaxSubdivisions&(o.Band.Patch.MaxSubdivisions-1) != 0 || !finite(o.Band.Patch.Deflection) || o.Band.Patch.Deflection <= 0 {
		return out, geom.ErrInvalid
	}
	proof, e := s.validateWrappedProfileStationBore(ctx, o.Band.Embedding, circular)
	if e != nil {
		return out, e
	}
	if !proof.Complete {
		return out, geom.ErrConvergence
	}
	var families [2]Solid
	var bands [2][]*geom.WrappedSurfacePatch
	build := NewWrappedRationalStationLoft
	if circular {
		build = NewWrappedCircularStationLoft
	}
	originals := [2][]int{proof.OuterWalls, proof.InnerWalls}
	for wall, ids := range originals {
		for _, id := range ids {
			if id < 0 || id >= len(s.faces) {
				return out, geom.ErrInvalid
			}
			w, ok := s.faces[id].Surface.(*geom.WrappedSurfacePatch)
			if !ok {
				return out, geom.ErrConvergence
			}
			bands[wall] = append(bands[wall], w)
		}
		families[wall], _, e = build(ctx, bands[wall], s.tolerance, o.Band.Embedding)
		if e != nil {
			return out, e
		}
	}
	var caps [2]*geom.SurfacePatch
	var capCuts [2][]float64
	weight := func(d geom.BSplineSurfaceData, i int) *big.Rat {
		if len(d.Weights) == 0 {
			return big.NewRat(1, 1)
		}
		return ratInset(d.Weights[i])
	}
	for cap := 0; cap < 2; cap++ {
		var d [2]geom.BSplineSurfaceData
		for wall := 0; wall < 2; wall++ {
			index := 0
			if cap == 1 {
				index = len(bands[wall]) - 1
			}
			bs, e := bands[wall][index].FundamentalPatch().ToBSplineSurface()
			if e != nil {
				return out, e
			}
			d[wall] = bs.Data()
		}
		if !slices.Equal(d[0].KnotsU, d[1].KnotsU) {
			return out, geom.ErrConvergence
		}
		for i := 0; i < 9; i++ {
			a := new(big.Rat).Mul(weight(d[0], 2*i+cap), weight(d[1], cap))
			b := new(big.Rat).Mul(weight(d[1], 2*i+cap), weight(d[0], cap))
			if a.Cmp(b) != 0 {
				return out, geom.ErrConvergence
			}
		}
		var poles []geom.Vec3
		var weights []float64
		for i := 0; i < 9; i++ {
			for _, wall := range []int{1, 0} {
				poles = append(poles, d[wall].Poles[2*i+cap])
				f, _ := weight(d[0], 2*i+cap).Float64()
				weights = append(weights, f)
			}
		}
		bs, e := geom.NewBSplineSurface(2, 1, 9, 2, poles, d[0].KnotsU, []float64{0, 0, 1, 1}, weights)
		if e != nil {
			return out, e
		}
		caps[cap], e = geom.NewSurfacePatch(bs, o.Band.Embedding)
		if e != nil {
			return out, e
		}
		capCuts[cap], _ = caps[cap].KnotBreaks()
	}
	checks, samples := 0, 0
	tolerance := ratInset(o.Band.Patch.Deflection)
	tolerance.Mul(tolerance, tolerance)
	for minimum := 1; minimum <= o.Band.Patch.MaxSubdivisions; {
		var local [2]CertifiedWrappedStationMesh
		var localBands [2][]wrappedDisplayBand
		maximum := minimum
		for wall := 0; wall < 2; wall++ {
			remaining := o.Band.Patch.MaxTriangleChecks - checks
			if remaining < 1 {
				return out, geom.ErrConvergence
			}
			opts := o
			opts.Band.Patch.MaxTriangleChecks = remaining
			opts.MaxStationSamples = o.MaxStationSamples - samples
			if opts.MaxStationSamples < 1 {
				if len(bands[wall]) > 1 {
					return out, geom.ErrConvergence
				}
				opts.MaxStationSamples = 1
			}
			var record *[]wrappedDisplayBand
			if capture != nil {
				record = &localBands[wall]
			}
			local[wall], e = families[wall].tessellateWrappedProfileStationLoft(ctx, opts, minimum, record, circular)
			if e != nil {
				return out, e
			}
			checks += local[wall].TriangleChecks
			samples += local[wall].StationSamples
			if samples > o.MaxStationSamples {
				return out, geom.ErrConvergence
			}
			if local[wall].Subdivisions > maximum {
				maximum = local[wall].Subdivisions
			}
		}
		if local[0].Subdivisions != local[1].Subdivisions {
			minimum = maximum
			continue
		}
		var candidateBoundary []wrappedDisplayBand
		candidate := CertifiedWrappedStationBoreMesh{Source: proof, Subdivisions: maximum, SourceImageSquaredDistance: new(big.Rat)}
		for wall, m := range local {
			mapping := make([]int, len(m.Mesh.Vertices))
			for i := range mapping {
				mapping[i] = -1
			}
			faceMap := map[int]int{}
			for i, id := range m.Source.History.WallFaces {
				faceMap[id] = originals[wall][i]
			}
			for i, tri := range m.Mesh.Triangles {
				face, retain := faceMap[m.OriginalFace[i]]
				if !retain {
					continue
				}
				if len(candidate.Mesh.Triangles) >= o.Band.MaxTriangles {
					return out, geom.ErrConvergence
				}
				var q [3]int
				for j, id := range tri {
					if mapping[id] < 0 {
						mapping[id] = len(candidate.Mesh.Vertices)
						candidate.Mesh.Vertices = append(candidate.Mesh.Vertices, m.Mesh.Vertices[id])
					}
					q[j] = mapping[id]
				}
				if wall == 1 {
					q[1], q[2] = q[2], q[1]
				}
				candidate.Mesh.Triangles = append(candidate.Mesh.Triangles, q)
				candidate.OriginalFace = append(candidate.OriginalFace, face)
				bound := m.SquaredErrors[i]
				candidate.SquaredErrors = append(candidate.SquaredErrors, new(big.Rat).Set(bound))
				if bound.Cmp(candidate.SourceImageSquaredDistance) > 0 {
					candidate.SourceImageSquaredDistance.Set(bound)
				}
			}
			if capture != nil {
				for _, band := range localBands[wall] {
					face, ok := faceMap[band.face]
					if !ok {
						return out, geom.ErrConvergence
					}
					ids := make([]int, len(band.indices))
					for i, id := range band.indices {
						if id < 0 || id >= len(mapping) || mapping[id] < 0 {
							return out, geom.ErrConvergence
						}
						ids[i] = mapping[id]
					}
					candidateBoundary = append(candidateBoundary, wrappedDisplayBand{face, band.u, band.v, ids})
				}
			}
			for _, ring := range m.StationVertices {
				var ids []int
				for _, id := range ring {
					if mapping[id] < 0 {
						return out, geom.ErrConvergence
					}
					ids = append(ids, mapping[id])
				}
				candidate.StationVertices[wall] = append(candidate.StationVertices[wall], ids)
			}
		}
		accepted := true
		for cap := 0; cap < 2; cap++ {
			var rings [2][]int
			for wall := 0; wall < 2; wall++ {
				station := 0
				if cap == 1 {
					station = len(candidate.StationVertices[wall]) - 1
				}
				rings[wall] = candidate.StationVertices[wall][station]
			}
			if len(rings[0]) != len(rings[1]) || len(rings[0]) != 4*maximum {
				return out, geom.ErrConvergence
			}
			var us []float64
			for span := 0; span < 4; span++ {
				a, b := ratInset(capCuts[cap][span]), ratInset(capCuts[cap][span+1])
				width := new(big.Rat).Sub(b, a)
				for i := 0; i < maximum; i++ {
					u := new(big.Rat).Add(a, new(big.Rat).Mul(width, big.NewRat(int64(i), int64(maximum))))
					f, _ := u.Float64()
					if !finite(f) || ratInset(f).Cmp(u) != 0 {
						return out, geom.ErrConvergence
					}
					us = append(us, f)
				}
			}
			us = append(us, capCuts[cap][4])
			for i := range rings[0] {
				j := (i + 1) % len(rings[0])
				ids := [4]int{rings[1][i], rings[1][j], rings[0][j], rings[0][i]}
				uv := [4]geom.Vec2{{X: us[i]}, {X: us[i+1]}, {X: us[i+1], Y: 1}, {X: us[i], Y: 1}}
				for _, tri := range [][3]int{{0, 1, 2}, {0, 2, 3}} {
					if checks >= o.Band.Patch.MaxTriangleChecks || len(candidate.Mesh.Triangles) >= o.Band.MaxTriangles {
						return out, geom.ErrConvergence
					}
					if cap == 1 {
						tri[1], tri[2] = tri[2], tri[1]
					}
					var q [3]int
					var param [3]geom.Vec2
					var model [3]geom.Vec3
					for k, corner := range tri {
						q[k] = ids[corner]
						param[k] = uv[corner]
						model[k] = candidate.Mesh.Vertices[q[k]]
					}
					bound, e := BoundSplineTriangleDeviation(ctx, caps[cap], param, model)
					checks++
					if e != nil {
						return out, e
					}
					if !bound.BothDirections {
						return out, geom.ErrConvergence
					}
					if bound.SquaredDistance.Cmp(tolerance) > 0 {
						accepted = false
					}
					if bound.SquaredDistance.Cmp(candidate.SourceImageSquaredDistance) > 0 {
						candidate.SourceImageSquaredDistance.Set(bound.SquaredDistance)
					}
					candidate.Mesh.Triangles = append(candidate.Mesh.Triangles, q)
					candidate.OriginalFace = append(candidate.OriginalFace, proof.Caps[cap])
					candidate.SquaredErrors = append(candidate.SquaredErrors, new(big.Rat).Set(bound.SquaredDistance))
				}
			}
		}
		if accepted {
			candidate.TriangleChecks, candidate.StationSamples = checks, samples
			candidate.SignedDisplayVolume, candidate.Collision, e = checkClosedPeriodicMesh(ctx, candidate.Mesh, o.Band.Collision)
			if e != nil {
				return out, e
			}
			if capture != nil {
				*capture = candidateBoundary
			}
			return candidate, nil
		}
		minimum = maximum * 2
	}
	return out, geom.ErrConvergence
}
