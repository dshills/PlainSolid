package trimmed

import (
	"context"
	"math/big"
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
	roofs, e := exactPolynomialCutRoofs(r.sources, o.Patch)
	if e != nil {
		return zero, e
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
	if !arrangement.Proven || arrangement.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) || arrangement.ContactOrder != r.orders {
		return geom.ErrConvergence
	}
	for i, gap := range arrangement.DerivativeGap {
		if gap.Cmp(r.derivatives[i]) != 0 {
			return geom.ErrConvergence
		}
	}
	uv, xyz, e := arrangement.RepresentedVertices()
	if e != nil {
		return e
	}
	return r.material.bindVolumeRegions(r.sources, report.Regions, report.Volumes, arrangement.Regions[:], arrangement.Edges[:], arrangement.Vertices[:], arrangement.Nodes[:], uv[:], xyz[:])
}

func exactPolynomialCutRoofs(sources [3]*geom.BSplineSurface, options geom.SurfacePatchOptions) ([2]*geom.SurfacePatch, error) {
	var roofs [2]*geom.SurfacePatch
	for i := range roofs {
		p, e := geom.NewSurfacePatch(sources[i+1], options)
		if e != nil {
			return [2]*geom.SurfacePatch{}, e
		}
		roofs[i] = p
	}
	return roofs, nil
}

func (material polynomialNodeCutMaterial) bindVolumeRegions(sources [3]*geom.BSplineSurface, regions []int, volumes []*big.Rat,
	sourceRegions []geom.SplineGraphPolynomialRegion, edges []geom.SplineGraphPolynomialPiece, vertices []geom.SplineGraphPolynomialVertex,
	nodes []geom.SplineGraphPolynomialJunction, uv []geom.Vec2, xyz []geom.Vec3) error {
	if !slices.Equal(regions, material.regions) || len(volumes) != len(material.components) || len(material.contacts) != len(nodes) {
		return geom.ErrConvergence
	}
	pieces, e := polynomialNodePieces(edges, uv)
	if e != nil {
		return e
	}
	loops, e := polynomialNodeRegionLoops(sourceRegions, uv, pieces)
	if e != nil {
		return e
	}
	for i, part := range material.components {
		if len(part.faces) < 2 || volumes[i].Sign() <= 0 {
			return geom.ErrConvergence
		}
		loop, e := splitPolynomialNodeCutPerimeter(loops[regions[i]])
		if e != nil {
			return e
		}
		for owner := 0; owner < 2; owner++ {
			if e = bindPolynomialTangencyFace(part.faces[owner], sources[owner+1], owner == 1, loop); e != nil {
				return e
			}
		}
	}
	for i, c := range material.contacts {
		node := nodes[i]
		vertex := vertices[node.Vertex]
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
			if component < 0 || component >= len(material.components) || !slices.Contains(node.Regions[:], regions[component]) {
				return geom.ErrConvergence
			}
			part, v := material.components[component], c.Vertices[j]
			if v < 0 || v >= len(part.vertices) || part.vertices[v] != xyz[node.Vertex] {
				return geom.ErrConvergence
			}
		}
	}
	return nil
}
