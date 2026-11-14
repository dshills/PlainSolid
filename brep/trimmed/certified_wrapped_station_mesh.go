package trimmed

import (
	"context"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// CertifiedWrappedStationMeshOptions shares Band.Patch.MaxTriangleChecks over
// all bands and synchronization retries. Band.MaxTriangles bounds each local
// candidate and the final mesh. Source validation, each local contact proof and
// the final contact proof retain their separately bounded options. Exact shared
// station samples are bounded independently (default65536, maximum1M).
type CertifiedWrappedStationMeshOptions struct {
	Band              CertifiedWrappedFrustumMeshOptions
	MaxStationSamples int
}

// CertifiedWrappedStationMesh reports two-sided source/display squared bounds
// for each retained face triangle, including shared-sample rounding. Exact
// display volume is diagnostic; no source-volume error bound is asserted.
type CertifiedWrappedStationMesh struct {
	Mesh                                            mesh.Mesh
	Source                                          WrappedStationLoftProof
	OriginalFace                                    []int
	StationVertices                                 [][]int
	SquaredErrors                                   []*big.Rat
	SourceImageSquaredDistance, SignedDisplayVolume *big.Rat
	Collision                                       collision.Report
	Subdivisions, TriangleChecks, StationSamples    int
}

// TessellateWrappedStationLoftCertified revalidates the original Solid's entire
// canonical circular station topology, including exact model/pcurve agreement.
// Each shared station sample is evaluated rationally on both original supports;
// full exact equality is required before assigning one display ID. End caps
// share those IDs. Interior station caps are omitted, retaining true slope
// creases and original face/rim ancestry. Nonmatching sample parameterizations
// reject without tolerance welding. The complete display must be closed with
// manifold links, positive exact volume and no non-incidence contacts.
func (s Solid) TessellateWrappedStationLoftCertified(ctx context.Context, o CertifiedWrappedStationMeshOptions) (out CertifiedWrappedStationMesh, err error) {
	return s.tessellateWrappedStationLoftFrom(ctx, o, 1)
}
func (s Solid) tessellateWrappedStationLoftFrom(ctx context.Context, o CertifiedWrappedStationMeshOptions, first int) (out CertifiedWrappedStationMesh, err error) {
	return s.tessellateWrappedStationLoftBoundaries(ctx, o, first, nil)
}

func (s Solid) tessellateWrappedStationLoftBoundaries(ctx context.Context, o CertifiedWrappedStationMeshOptions, first int, capture *[]wrappedDisplayBand) (out CertifiedWrappedStationMesh, err error) {
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
			out = CertifiedWrappedStationMesh{}
		}
	}()
	if ctx == nil {
		return out, geom.ErrInvalid
	}
	if o.MaxStationSamples == 0 {
		o.MaxStationSamples = 65536
	}
	if o.MaxStationSamples < 1 || o.MaxStationSamples > 1000000 {
		return out, geom.ErrInvalid
	}
	if o.Band.Patch.MaxTriangleChecks == 0 {
		o.Band.Patch.MaxTriangleChecks = 100000
	}
	if o.Band.Patch.MaxTriangleChecks < 1 {
		return out, geom.ErrInvalid
	}
	if o.Band.Patch.MaxSubdivisions == 0 {
		o.Band.Patch.MaxSubdivisions = 32
	}
	if o.Band.MaxTriangles == 0 {
		o.Band.MaxTriangles = 1000000
	}
	if o.Band.MaxTriangles < 1 || o.Band.MaxTriangles > 8000000 {
		return out, geom.ErrInvalid
	}
	proof, bands, e := s.validateWrappedCircularStationLoft(ctx, o.Band.Embedding)
	if e != nil {
		return out, e
	}
	if !proof.Complete || len(bands) == 0 {
		return out, geom.ErrConvergence
	}
	checks, minimum := 0, first
	var local []CertifiedWrappedFrustumMesh
	for {
		local = make([]CertifiedWrappedFrustumMesh, len(bands))
		maximum := minimum
		equal := true
		for i, w := range bands {
			remaining := o.Band.Patch.MaxTriangleChecks - checks
			if remaining < 1 {
				return out, geom.ErrConvergence
			}
			opts := o.Band
			opts.Patch.MaxTriangleChecks = remaining
			m, e := tessellateWrappedFrustumFrom(ctx, w, opts, minimum)
			if e != nil {
				return out, e
			}
			checks += m.Lateral.TriangleChecks
			local[i] = m
			if m.Lateral.Subdivisions > maximum {
				maximum = m.Lateral.Subdivisions
			}
			if i > 0 && m.Lateral.Subdivisions != local[0].Lateral.Subdivisions {
				equal = false
			}
		}
		if equal {
			minimum = maximum
			break
		}
		minimum = maximum
	}
	out.Source = proof
	out.Subdivisions = minimum
	out.TriangleChecks = checks
	out.SourceImageSquaredDistance = new(big.Rat)
	out.StationVertices = make([][]int, len(bands)+1)
	for band, m := range local {
		mapping := make([]int, len(m.Mesh.Vertices))
		for i := range mapping {
			mapping[i] = -1
		}
		if band > 0 {
			previous := local[band-1]
			lower, upper := m.RimVertices[0], previous.RimVertices[1]
			if len(lower) != len(upper) || len(lower) != len(out.StationVertices[band]) {
				return out, geom.ErrConvergence
			}
			for k, id := range lower {
				if out.StationSamples >= o.MaxStationSamples {
					return out, geom.ErrConvergence
				}
				out.StationSamples++
				a, e := bands[band-1].FundamentalPatch().EvaluateRational(ctx, ratInset(previous.Lateral.UParameters[k]), ratInset(previous.Lateral.VParameters[len(previous.Lateral.VParameters)-1]), o.Band.Patch.Evaluation)
				if e != nil {
					return out, e
				}
				b, e := bands[band].FundamentalPatch().EvaluateRational(ctx, ratInset(m.Lateral.UParameters[k]), ratInset(m.Lateral.VParameters[0]), o.Band.Patch.Evaluation)
				if e != nil {
					return out, e
				}
				for axis := 0; axis < 3; axis++ {
					if a.Point[axis].Cmp(b.Point[axis]) != 0 {
						return out, geom.ErrConvergence
					}
				}
				shared := out.StationVertices[band][k]
				if m.Mesh.Vertices[id] != out.Mesh.Vertices[shared] {
					return out, geom.ErrConvergence
				}
				mapping[id] = shared
			}
		}
		for i, tri := range m.Mesh.Triangles {
			face := m.OriginalFace[i]
			if face == 1 && band != 0 || face == 2 && band != len(bands)-1 {
				continue
			}
			if len(out.Mesh.Triangles) >= o.Band.MaxTriangles {
				return out, geom.ErrConvergence
			}
			var q [3]int
			for j, id := range tri {
				if mapping[id] < 0 {
					mapping[id] = len(out.Mesh.Vertices)
					out.Mesh.Vertices = append(out.Mesh.Vertices, m.Mesh.Vertices[id])
				}
				q[j] = mapping[id]
			}
			original := proof.History.WallFaces[band]
			if face > 0 {
				original = proof.History.CapFaces[face-1]
			}
			out.Mesh.Triangles = append(out.Mesh.Triangles, q)
			out.OriginalFace = append(out.OriginalFace, original)
			bound := m.SquaredErrors[i]
			out.SquaredErrors = append(out.SquaredErrors, new(big.Rat).Set(bound))
			if bound.Cmp(out.SourceImageSquaredDistance) > 0 {
				out.SourceImageSquaredDistance.Set(bound)
			}
		}
		if capture != nil {
			ids := append([]int(nil), mapping[:len(m.Lateral.Mesh.Vertices)]...)
			for _, id := range ids {
				if id < 0 {
					return out, geom.ErrConvergence
				}
			}
			*capture = append(*capture, wrappedDisplayBand{proof.History.WallFaces[band], append([]float64(nil), m.Lateral.UParameters...), append([]float64(nil), m.Lateral.VParameters...), ids})
		}
		if band == 0 {
			for _, id := range m.RimVertices[0] {
				out.StationVertices[0] = append(out.StationVertices[0], mapping[id])
			}
		}
		for _, id := range m.RimVertices[1] {
			if mapping[id] < 0 {
				return out, geom.ErrConvergence
			}
			out.StationVertices[band+1] = append(out.StationVertices[band+1], mapping[id])
		}
	}
	out.SignedDisplayVolume, out.Collision, e = checkClosedPeriodicMesh(ctx, out.Mesh, o.Band.Collision)
	if e != nil {
		return out, e
	}
	return out, nil
}
