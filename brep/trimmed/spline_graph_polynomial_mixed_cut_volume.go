package trimmed

import (
	"context"
	"slices"

	"plainsolid/geom"
)

// PolynomialMixedCutVolumeOptions bounds original patch conversion and the
// exact source reproof and integration.
type PolynomialMixedCutVolumeOptions struct {
	Patch geom.SurfacePatchOptions
	Proof geom.GraphPointContactOptions
}

// ExactComponentVolumes integrates the original physical height difference
// over the Cut's proven positive regions. It returns one exact rational volume
// per native component in Components order. The roof-height difference must
// have a positive constant denominator on each original union-knot cell.
// The source and contact ledgers are checked again, including on a loaded Cut.
func (r *PolynomialMixedCut) ExactComponentVolumes(ctx context.Context, o PolynomialMixedCutVolumeOptions) (geom.SplineGraphPolynomialMixedVolumes, error) {
	zero := geom.SplineGraphPolynomialMixedVolumes{}
	if ctx == nil || r == nil || r.sources[0] == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	var roofs [2]*geom.SurfacePatch
	for i := range roofs {
		p, e := geom.NewSurfacePatch(r.sources[i+1], o.Patch)
		if e != nil {
			return zero, e
		}
		roofs[i] = p
	}
	report, e := geom.IntegrateSplineGraphPolynomialMixedVolumes(ctx, roofs[0], roofs[1], r.axis, o.Proof)
	if e != nil {
		return zero, e
	}
	if e = r.bindVolumeRegions(report); e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return report, nil
}

func (r *PolynomialMixedCut) bindVolumeRegions(report geom.SplineGraphPolynomialMixedVolumes) error {
	arrangement := report.Arrangement
	if !arrangement.Proven || arrangement.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) ||
		!slices.Equal(report.Regions, r.material.regions) || len(report.Volumes) != len(r.material.components) ||
		arrangement.ContactOrder != r.orders || len(r.material.contacts) != len(arrangement.Nodes) {
		return geom.ErrConvergence
	}
	uv, xyz, e := arrangement.RepresentedVertices()
	if e != nil {
		return e
	}
	pieces, e := polynomialNodePieces(arrangement.Edges[:], uv[:])
	if e != nil {
		return e
	}
	loops, e := polynomialNodeRegionLoops(arrangement.Regions[:], uv[:], pieces)
	if e != nil {
		return e
	}
	for i, part := range r.material.components {
		if len(part.faces) < 2 || report.Volumes[i].Sign() <= 0 {
			return geom.ErrConvergence
		}
		loop, e := splitPolynomialNodeCutPerimeter(loops[report.Regions[i]])
		if e != nil {
			return e
		}
		for owner := 0; owner < 2; owner++ {
			if e = bindPolynomialTangencyFace(part.faces[owner], r.sources[owner+1], owner == 1, loop); e != nil {
				return e
			}
		}
	}
	for i, c := range r.material.contacts {
		node := arrangement.Nodes[i]
		vertex := arrangement.Vertices[node.Vertex]
		if arrangement.DerivativeGap[i].Cmp(r.derivatives[i]) != 0 {
			return geom.ErrConvergence
		}
		for j, q := range vertex.Parameters {
			if q.Cmp(c.Parameters[j]) != 0 {
				return geom.ErrConvergence
			}
		}
		for j, q := range vertex.Point {
			if q.Cmp(c.Point[j]) != 0 {
				return geom.ErrConvergence
			}
		}
		if c.Components[0] >= c.Components[1] {
			return geom.ErrConvergence
		}
		for j, component := range c.Components {
			if component < 0 || component >= len(r.material.components) || !slices.Contains(node.Regions[:], report.Regions[component]) {
				return geom.ErrConvergence
			}
			part, v := r.material.components[component], c.Vertices[j]
			if v < 0 || v >= len(part.vertices) || part.vertices[v] != xyz[node.Vertex] {
				return geom.ErrConvergence
			}
		}
	}
	return nil
}
