package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
)

// MappedChainPolylineOptions bounds the entire output and composition/chord work.
// Descriptor preparation retains a separate MaxOperations allowance per cell
// (at most15), plus one support extraction; it is not counted in ExactOperations.
type MappedChainPolylineOptions struct {
	Deflection                                               float64
	MaxSegments, MaxOperations, MaxCoefficientBits, MaxDepth int
}
type MappedChainPolylineVertex struct {
	Point              geom.Vec3
	ModelParameter     *big.Rat
	AuthoredParameters [2]*big.Rat
}
type MappedChainPolylineSegment struct {
	Vertices   [2]int
	Cell       int
	ErrorUpper float64
}

// CertifiedMappedChainPolyline covers the complete proved piecewise edge. The
// pointwise chord bound is two-sided under affine interpolation of ModelParameter.
// Both original owners share this one index array. KnotVertices retain every
// original model/source station. No face fill or embedding is certified.
type CertifiedMappedChainPolyline struct {
	Vertices        []MappedChainPolylineVertex
	Segments        []MappedChainPolylineSegment
	KnotVertices    []int
	MaximumError    float64
	ExactOperations int
}

// TessellateCertified currently requires no additional support-knot crossing
// inside each common source cell. It uses exact homogeneous composition and the
// existing second-derivative/endpoint-rounding certificate, never sampled fitting.
// All failures discard the complete partial ledger. Defaults: .001,4096 segments,
// 8M counted operations,16384 coefficient bits,32 subdivision levels.
func (r MappedSplineChainAgreement) TessellateCertified(ctx context.Context, o MappedChainPolylineOptions) (CertifiedMappedChainPolyline, error) {
	var zero CertifiedMappedChainPolyline
	if ctx == nil || len(r.cuts) < 2 || len(r.cuts) > 16 || len(r.points) != len(r.cuts) {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if o.Deflection == 0 {
		o.Deflection = .001
	}
	if o.MaxSegments == 0 {
		o.MaxSegments = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 32
	}
	if math.IsNaN(o.Deflection) || math.IsInf(o.Deflection, 0) || o.Deflection <= 0 || o.MaxSegments < 1 || o.MaxSegments > 65536 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 16384 || o.MaxDepth < 1 || o.MaxDepth > 64 {
		return zero, geom.ErrInvalid
	}
	limits := geom.SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	p, e := exactSplineSupportPatch(r.supports[0], limits)
	if e != nil {
		return zero, e
	}
	junctions := make([]geom.Vec3, len(r.points))
	for i, point := range r.points {
		var v [3]float64
		for k := range v {
			v[k], _ = point[k].Float64()
		}
		junctions[i] = geom.Vec3{X: v[0], Y: v[1], Z: v[2]}
		if !junctions[i].IsFinite() {
			return zero, geom.ErrUnrepresentable
		}
	}
	var out CertifiedMappedChainPolyline
	for i := 0; i+1 < len(r.cuts); i++ {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		remaining := o.MaxOperations - out.ExactOperations
		if remaining < 1 || len(out.Segments) >= o.MaxSegments {
			return zero, geom.ErrConvergence
		}
		curve, e := r.sources[0].cell(r.cuts[i], r.cuts[i+1])
		if e != nil {
			return zero, e
		}
		descriptor, e := curve.descriptor(ctx, limits)
		if e != nil {
			return zero, e
		}
		cells, e := p.BuildSurfaceCurveCompositionCells(ctx, descriptor, geom.SurfaceCurveAgreementOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits}})
		out.ExactOperations += cells.ExactOperations
		if e != nil {
			return zero, e
		}
		if !cells.Complete || len(cells.Cells) != 1 {
			return zero, geom.ErrConvergence
		}
		remaining = o.MaxOperations - out.ExactOperations
		if remaining < 1 {
			return zero, geom.ErrConvergence
		}
		poly, e := cells.Cells[0].TessellateCertified(ctx, geom.ComposedCurvePolylineOptions{SurfacePatchEvaluationOptions: geom.SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits}, Deflection: o.Deflection, MaxSegments: o.MaxSegments - len(out.Segments), MaxDepth: o.MaxDepth, EndpointPoints: &[2]geom.Vec3{junctions[i], junctions[i+1]}})
		out.ExactOperations += poly.ExactOperations
		if e != nil {
			return zero, e
		}
		if !poly.Complete || len(poly.Vertices) < 2 {
			return zero, geom.ErrConvergence
		}
		base := len(out.Vertices)
		if i > 0 {
			base--
		}
		out.KnotVertices = append(out.KnotVertices, base)
		width := new(big.Rat).Sub(r.cuts[i+1], r.cuts[i])
		for j, v := range poly.Vertices {
			if v.Algebraic || v.Parameter.Lower == nil || v.Parameter.Upper == nil || v.Parameter.Lower.Cmp(v.Parameter.Upper) != 0 {
				return zero, geom.ErrConvergence
			}
			t := new(big.Rat).Add(r.cuts[i], new(big.Rat).Mul(width, v.Parameter.Lower))
			if j == 0 && i > 0 {
				last := out.Vertices[len(out.Vertices)-1]
				if last.ModelParameter.Cmp(t) != 0 || last.Point != v.Point {
					return zero, geom.ErrConvergence
				}
				continue
			}
			sample := MappedChainPolylineVertex{Point: v.Point, ModelParameter: t}
			for owner := 0; owner < 2; owner++ {
				raw, e := r.sources[owner].AuthoredParameter(t)
				if e != nil {
					return zero, e
				}
				sample.AuthoredParameters[owner] = raw
			}
			out.Vertices = append(out.Vertices, sample)
		}
		for _, s := range poly.Segments {
			if s.Vertices[0] < 0 || s.Vertices[1] >= len(poly.Vertices) {
				return zero, geom.ErrConvergence
			}
			out.Segments = append(out.Segments, MappedChainPolylineSegment{Vertices: [2]int{base + s.Vertices[0], base + s.Vertices[1]}, Cell: i, ErrorUpper: s.ErrorUpper})
		}
		if poly.MaximumError > out.MaximumError {
			out.MaximumError = poly.MaximumError
		}
	}
	out.KnotVertices = append(out.KnotVertices, len(out.Vertices)-1)
	return out, nil
}
