package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
)

// MappedChainFaceOptions keeps independent boundary, complete UV-fill/lift and
// global represented-triangle contact budgets. Closing trims must be original
// exact quadratic/Bezier curves in the chosen owner's chart, ending at the
// first chain endpoint; the full outer ring must satisfy the existing CCW proof.
type MappedChainFaceOptions struct {
	Boundary  MappedChainPolylineOptions
	Face      MixedExactMultiSpanMeshOptions
	Collision collision.Options
}

// MappedChainFaceSample stores the original part index and raw parameter on
// each owner at one sample of the common model cell.
type MappedChainFaceSample struct {
	Cell, Vertex       int
	AuthoredParts      [2]int
	ModelParameter     *big.Rat
	AuthoredParameters [2]*big.Rat
}

// CertifiedMappedChainFace fills one original chart region. LedgerVertices map
// all certified shared edge samples into the actual face mesh; extra adaptive
// samples retain both original source histories. A joint has one sample per
// cell, each with its own original source parameter. No shell or material claim.
type CertifiedMappedChainFace struct {
	Face           CertifiedMixedExactCurvedMesh
	Boundary       CertifiedMappedChainPolyline
	LedgerVertices []int
	Samples        []MappedChainFaceSample
	Collision      collision.Report
	Owner          int
	supportCells   [][2]int
}

// TessellateFaceCertified expands the proved source cells only for exact trim
// validation and filling; no polynomial replacement or native Solid admission
// occurs. Closing topology, two-sided surface error and all nonincident triangle
// contacts are independently certified. Additional boundary refinement is allowed,
// but every shared ledger point must retain its exact model t and bit-equal XYZ.
func (r MappedSplineChainAgreement) TessellateFaceCertified(ctx context.Context, owner int, closing []PCurve, o MappedChainFaceOptions) (CertifiedMappedChainFace, error) {
	var zero CertifiedMappedChainFace
	if ctx == nil || owner < 0 || owner > 1 || len(closing) < 1 || len(closing) > 16 || len(r.cuts) < 2 {
		return zero, geom.ErrInvalid
	}
	boundary, e := r.TessellateCertified(ctx, o.Boundary)
	if e != nil {
		return zero, e
	}
	out, e := r.tessellateFaceWithBoundary(ctx, owner, closing, boundary, nil, o.Face, false, 0, 0)
	if e != nil {
		return zero, e
	}
	report, e := collision.SelfIntersectionsBVHContext(ctx, out.Face.Mesh, o.Collision)
	if e != nil {
		return zero, fmt.Errorf("mapped chain face contact: %w", e)
	}
	if !report.Complete || len(report.Contacts) > 0 {
		return zero, geom.ErrConvergence
	}
	out.Collision = report
	return out, nil
}

// The caller supplies the shared boundary and checks represented contacts on
// either the individual face or the complete assembled mesh.
func (r MappedSplineChainAgreement) tessellateFaceWithBoundary(ctx context.Context, owner int, closing []PCurve, boundary CertifiedMappedChainPolyline, prescribed [][]*big.Rat, o MixedExactMultiSpanMeshOptions, alignSupportKnots bool, maxFlips int, angle float64) (CertifiedMappedChainFace, error) {
	var zero CertifiedMappedChainFace
	if len(boundary.Vertices)+len(r.cuts)-2 > 2048 {
		return zero, geom.ErrConvergence
	}
	if o.Face.Cuts != nil {
		return zero, geom.ErrInvalid
	}
	outer, e := r.mappedChainOwnerOuter(owner, closing)
	if e != nil {
		return zero, e
	}
	cuts := prescribed
	if cuts == nil {
		cuts = make([][]*big.Rat, len(outer))
		for i := 0; i+1 < len(r.cuts); i++ {
			width := new(big.Rat).Sub(r.cuts[i+1], r.cuts[i])
			for _, v := range boundary.Vertices {
				t := v.ModelParameter
				if t.Cmp(r.cuts[i]) >= 0 && t.Cmp(r.cuts[i+1]) <= 0 {
					cuts[i] = append(cuts[i], new(big.Rat).Quo(new(big.Rat).Sub(t, r.cuts[i]), width))
				}
			}
		}
	}
	p, e := exactSplineSupportPatch(r.supports[owner], o.Face.Boundary.Composition.SurfacePatchOptions)
	if e != nil {
		return zero, e
	}
	if alignSupportKnots {
		cuts, e = mappedChainKnotTrimCuts(ctx, p, outer, cuts, o.Face.Boundary.Composition)
		if e != nil {
			return zero, fmt.Errorf("mapped chain support knot samples: %w", e)
		}
	}
	o.Face.Cuts = [][][]*big.Rat{cuts}
	face, e := tessellateMixedExactMultiSpanRegionCertified(ctx, p, outer, nil, o, alignSupportKnots, maxFlips, angle)
	if e != nil {
		return zero, fmt.Errorf("mapped chain face fill: %w", e)
	}
	out := CertifiedMappedChainFace{Face: face, Boundary: boundary, Owner: owner, LedgerVertices: make([]int, len(boundary.Vertices))}
	if alignSupportKnots {
		out.supportCells, e = mappedChainTriangleSupportCells(p, face.CertifiedCurvedQuadraticTrimMesh)
		if e != nil {
			return zero, fmt.Errorf("mapped chain triangle support cells: %w", e)
		}
	}
	for i := range out.LedgerVertices {
		out.LedgerVertices[i] = -1
	}
	ledger := make(map[string]int, len(boundary.Vertices))
	for i, v := range boundary.Vertices {
		ledger[v.ModelParameter.RatString()] = i
	}
	for _, use := range face.UVRegion.Samples {
		if use.Loop != 0 || use.Trim >= len(r.cuts)-1 {
			continue
		}
		cell := use.Trim
		width := new(big.Rat).Sub(r.cuts[cell+1], r.cuts[cell])
		for j, t := range use.Parameters {
			global := new(big.Rat).Add(r.cuts[cell], new(big.Rat).Mul(width, t))
			id := use.Vertices[j]
			sample := MappedChainFaceSample{Cell: cell, Vertex: id, ModelParameter: global}
			for side := 0; side < 2; side++ {
				part, raw, e := r.sources[side].authoredCellParameter(r.cuts[cell], r.cuts[cell+1], t)
				if e != nil {
					return zero, e
				}
				sample.AuthoredParts[side], sample.AuthoredParameters[side] = part, raw
			}
			out.Samples = append(out.Samples, sample)
			if k, ok := ledger[global.RatString()]; ok {
				if face.Mesh.Vertices[id] != boundary.Vertices[k].Point {
					return zero, geom.ErrConvergence
				}
				if out.LedgerVertices[k] >= 0 && out.LedgerVertices[k] != id {
					return zero, geom.ErrConvergence
				}
				out.LedgerVertices[k] = id
			}

		}
	}
	for _, id := range out.LedgerVertices {
		if id < 0 {
			return zero, geom.ErrConvergence
		}
	}
	return out, nil
}

func (r MappedSplineChainAgreement) mappedChainOwnerOuter(owner int, closing []PCurve) ([]PCurve, error) {
	outer := make([]PCurve, 0, len(r.cuts)-1+len(closing))
	for i := 0; i+1 < len(r.cuts); i++ {
		q, err := r.sources[owner].cell(r.cuts[i], r.cuts[i+1])
		if err != nil {
			return nil, err
		}
		outer = append(outer, q)
	}
	return append(outer, closing...), nil
}
