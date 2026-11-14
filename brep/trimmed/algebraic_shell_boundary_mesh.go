package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math"
	"plainsolid/geom"
)

// AlgebraicShellBoundaryOptions bounds the complete network. Preparation proof
// limits apply per original edge; MaxOperations bounds all display certificates
// and endpoint enclosures together. Source and Section supply subordinate
// depth/precision options; their deflection, segment and operation limits are
// replaced by the remaining network budget. No caller endpoint is accepted.
type AlgebraicShellBoundaryOptions struct {
	// AutoSectionProjection proves a suitable affine projection of each opposite
	// original support. It is exclusive with Section.Projection.
	AutoSectionProjection bool
	// PlanarSectionFallback permits exact rational point recovery on a proved
	// planar opposite support with nonlinear chart coordinates. Requires auto selection.
	PlanarSectionFallback                            bool
	Proof                                            AlgebraicSeamOptions
	Deflection                                       float64
	MaxEdges, MaxCoedges, MaxSegments, MaxOperations int
	Source                                           geom.ComposedCurvePolylineOptions
	Section                                          RationalTubePolylineOptions
}

type AlgebraicShellBoundaryCoedge struct {
	Face, Loop, Use, Edge           int
	Operand, OriginalFace, Fragment int
	Reversed                        bool
	Vertices                        []int
}

type CertifiedAlgebraicShellBoundary struct {
	// SectionProjections records axes independently selected for each section edge.
	SectionProjections        map[int][2]int
	PlanarSections            []int
	Vertices                  []geom.Vec3
	TopologyVertices          []int
	Edges                     [][]int
	Ancestry                  []AlgebraicShellBooleanEdge
	Coedges                   []AlgebraicShellBoundaryCoedge
	SourceCertificates        map[int]geom.CertifiedComposedCollectionPolyline
	SectionCertificates       map[int]CertifiedRationalTubePolyline
	Segments, ExactOperations int
}

// TessellateBoundaryCertified approximates every retained original edge and
// generated section with independently certified chords and shared indexed
// endpoints. Section support is the rational-free-domain graph-tube subset
// accepted by TessellateRationalTubeSegmentCertified, including an explicitly
// chosen Section.Projection. Source and section curve
// certificates include display rounding and both directed chord distances.
// A face fill additionally needs certified original-chart boundary transport,
// fixed-boundary triangulation of every retained loop, original-surface
// triangle residual bounds including endpoint displacement, and independent
// orientation/incidence and intersection checks. Multiple support patches are
// not covered by the specialized two-face lens filler.
// This proves neither face filling nor embedding of source or display. Distinct
// exact vertices that collapse in binary64 are rejected, never welded. Errors
// return no partial network. Mutable public Boolean diagnostics are ignored.
func (s AlgebraicShellBoolean) TessellateBoundaryCertified(ctx context.Context, o AlgebraicShellBoundaryOptions) (CertifiedAlgebraicShellBoundary, error) {
	var empty CertifiedAlgebraicShellBoundary
	if ctx == nil || s.source == nil || !(o.Deflection > 0) || math.IsInf(o.Deflection, 0) {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.MaxEdges == 0 {
		o.MaxEdges = 256
	}
	if o.MaxCoedges == 0 {
		o.MaxCoedges = 512
	}
	if o.MaxSegments == 0 {
		o.MaxSegments = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxEdges < 1 || o.MaxEdges > 4096 || o.MaxCoedges < 1 || o.MaxCoedges > 8192 || o.MaxSegments < 1 || o.MaxSegments > 65536 || o.MaxOperations < 1 || o.MaxOperations > 1000000000 || o.Source.EndpointPoints != nil || o.Section.EndpointPoints != nil {
		return empty, geom.ErrInvalid
	}
	if o.PlanarSectionFallback && !o.AutoSectionProjection || o.AutoSectionProjection && (o.Section.Projection != nil || o.Section.PlanarPointEvaluation) {
		return empty, geom.ErrInvalid
	}
	if len(s.index) == 0 || len(s.index) > o.MaxEdges || len(s.vertices) == 0 {
		return empty, geom.ErrConvergence
	}
	// Bound topology work before any exact endpoint or curve computation.
	coedges := 0
	for _, f := range s.faces {
		coedges += len(f.Outer)
		for _, ring := range f.Holes {
			coedges += len(ring)
		}
		if coedges > o.MaxCoedges {
			return empty, geom.ErrConvergence
		}
	}
	if len(s.vertices) > 2*len(s.index) {
		return empty, geom.ErrInvalid
	}
	r := CertifiedAlgebraicShellBoundary{Edges: make([][]int, len(s.index)), TopologyVertices: make([]int, len(s.vertices)), Ancestry: append([]AlgebraicShellBooleanEdge(nil), s.index...), SourceCertificates: map[int]geom.CertifiedComposedCollectionPolyline{}, SectionCertificates: map[int]CertifiedRationalTubePolyline{}}
	projections := map[[2]int][2]int{}
	planar := map[[2]int]bool{}
	if o.AutoSectionProjection {
		r.SectionProjections = map[int][2]int{}
	}
	seen := map[geom.Vec3]bool{}
	add := func(p geom.Vec3) (int, error) {
		if !p.IsFinite() {
			return 0, geom.ErrInvalid
		}
		if seen[p] {
			return 0, geom.ErrDegenerate
		}
		seen[p] = true
		i := len(r.Vertices)
		r.Vertices = append(r.Vertices, p)
		return i, nil
	}
	for i, v := range s.vertices {
		var p geom.Vec3
		if v.SectionPoint != nil {
			if r.ExactOperations >= o.MaxOperations {
				return empty, geom.ErrConvergence
			}
			b, e := v.SectionPoint.Enclose(ctx, geom.ComposedCurveEndpointOptions{Tolerance: o.Deflection / 16, ExactCurvePlaneEventOptions: geom.ExactCurvePlaneEventOptions{MaxOperations: o.MaxOperations - r.ExactOperations}})
			if e != nil {
				return empty, e
			}
			r.ExactOperations += b.ExactOperations
			if !b.Converged || !b.HasRepresentative {
				return empty, geom.ErrConvergence
			}
			p = b.Point
		} else {
			if v.Operand < 0 || v.Operand > 1 || v.OriginalSubdivisionVertex < 0 || v.OriginalSubdivisionVertex >= len(s.parts[v.Operand].Subdivision.vertices) {
				return empty, geom.ErrInvalid
			}
			p = s.parts[v.Operand].Subdivision.vertices[v.OriginalSubdivisionVertex].Display
		}
		id, e := add(p)
		if e != nil {
			return empty, e
		}
		r.TopologyVertices[i] = id
	}
	for ei, edge := range s.index {
		if r.Segments >= o.MaxSegments || r.ExactOperations >= o.MaxOperations {
			return empty, geom.ErrConvergence
		}
		if edge.Start < 0 || edge.End < 0 || edge.Start >= len(s.vertices) || edge.End >= len(s.vertices) {
			return empty, geom.ErrInvalid
		}
		ends := [2]geom.Vec3{r.Vertices[r.TopologyVertices[edge.Start]], r.Vertices[r.TopologyVertices[edge.End]]}
		var points []geom.Vec3
		if edge.OriginalSubdivisionEdge >= 0 {
			cells, e := s.SourceCurveCells(ctx, ei, o.Proof)
			if e != nil {
				return empty, e
			}
			opt := o.Source
			if r.ExactOperations >= o.MaxOperations {
				return empty, geom.ErrConvergence
			}
			opt.Deflection = o.Deflection
			opt.MaxSegments = o.MaxSegments - r.Segments
			opt.MaxOperations = o.MaxOperations - r.ExactOperations
			opt.EndpointPoints = &ends
			line, e := cells.TessellateCertified(ctx, geom.ComposedCollectionPolylineOptions{ComposedCurvePolylineOptions: opt})
			if e != nil {
				return empty, fmt.Errorf("source edge %d: %w", ei, e)
			}
			if !line.Complete || line.DisplayDegenerate {
				return empty, geom.ErrConvergence
			}
			for _, v := range line.Vertices {
				points = append(points, v.Point)
			}
			r.ExactOperations += line.ExactOperations
			r.SourceCertificates[ei] = line
		} else {
			segment, pair, e := s.SectionSegment(ei)
			if e != nil {
				return empty, e
			}
			freeAxis, domain, reverse, e := segment.ExactFreeDomain()
			if e != nil {
				return empty, e
			}
			opt := o.Section
			if o.AutoSectionProjection {
				owner := 1 - freeAxis/2
				key := [2]int{owner, pair[owner]}
				axes, ok := projections[key]
				if !ok && !planar[key] {
					var used int
					axes, used, e = s.selectBoundaryGraphProjection(ctx, key, o.MaxOperations-r.ExactOperations, o.Section.Point.MaxCoefficientBits)
					r.ExactOperations += used
					if e != nil {
						if !o.PlanarSectionFallback || !errors.Is(e, geom.ErrConvergence) {
							return empty, fmt.Errorf("section edge %d projection: %w", ei, e)
						}
						planar[key] = true
					} else {
						projections[key] = axes
					}
				}
				if planar[key] {
					opt.PlanarPointEvaluation = true
					r.PlanarSections = append(r.PlanarSections, ei)
				} else {
					r.SectionProjections[ei] = axes
					opt.Projection = &axes
				}
			}

			if r.ExactOperations >= o.MaxOperations {
				return empty, geom.ErrConvergence
			}
			opt.Deflection = o.Deflection
			opt.MaxSegments = o.MaxSegments - r.Segments
			opt.MaxOperations = o.MaxOperations - r.ExactOperations
			supplied := ends
			if reverse {
				supplied[0], supplied[1] = supplied[1], supplied[0]
			}
			opt.EndpointPoints = &supplied
			line, e := TessellateRationalTubeSegmentCertified(ctx, segment, domain.Lower, domain.Upper, opt)
			if e != nil {
				return empty, fmt.Errorf("section edge %d: %w", ei, e)
			}
			for _, v := range line.Samples {
				points = append(points, v.Point)
			}
			if reverse {
				for a, b := 0, len(points)-1; a < b; a, b = a+1, b-1 {
					points[a], points[b] = points[b], points[a]
				}
			}
			r.ExactOperations += line.ExactOperations
			r.SectionCertificates[ei] = line
		}
		if len(points) < 2 || points[0] != ends[0] || points[len(points)-1] != ends[1] {
			return empty, geom.ErrConvergence
		}
		path := make([]int, len(points))
		path[0] = r.TopologyVertices[edge.Start]
		path[len(path)-1] = r.TopologyVertices[edge.End]
		for j := 1; j < len(points)-1; j++ {
			id, e := add(points[j])
			if e != nil {
				return empty, e
			}
			path[j] = id
		}
		r.Edges[ei] = path
		r.Segments += len(path) - 1
	}
	counts := make([]int, len(s.index))
	balance := make([]int, len(s.index))
	for fi, f := range s.faces {
		for li, loop := range append([][]AlgebraicShellBooleanUse{f.Outer}, f.Holes...) {
			for ui, u := range loop {
				if len(r.Coedges) >= o.MaxCoedges {
					return empty, geom.ErrConvergence
				}
				if u.Edge < 0 || u.Edge >= len(r.Edges) {
					return empty, geom.ErrInvalid
				}
				path := append([]int(nil), r.Edges[u.Edge]...)
				reverse := u.Reversed != f.Reversed
				if reverse {
					for a, b := 0, len(path)-1; a < b; a, b = a+1, b-1 {
						path[a], path[b] = path[b], path[a]
					}
					balance[u.Edge]--
				} else {
					balance[u.Edge]++
				}
				counts[u.Edge]++
				r.Coedges = append(r.Coedges, AlgebraicShellBoundaryCoedge{Face: fi, Loop: li, Use: ui, Edge: u.Edge, Operand: f.Operand, OriginalFace: f.Original, Fragment: f.Fragment, Reversed: reverse, Vertices: path})
			}
		}
	}
	for i, n := range counts {
		if n != 2 || balance[i] != 0 {
			return empty, geom.ErrConvergence
		}
	}
	return r, nil
}
