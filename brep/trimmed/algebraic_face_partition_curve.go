//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// AlgebraicPartitionCurve retains the original UV line parameter (U except
// on vertical chart lines, where it is V), exact endpoint identities, and
// complete model-space knot pieces. Both fragments refer to this one curve.
type AlgebraicPartitionCurve struct {
	Partition AlgebraicFacePartition
	Axis      int
	Endpoints [2]*geom.ExactAlgebraicRoot
	Model     geom.SurfaceCurveCompositionCells
}

func cloneAlgebraicPartitionCurve(c AlgebraicPartitionCurve) AlgebraicPartitionCurve {
	for i, x := range c.Partition.Line {
		if x != nil {
			c.Partition.Line[i] = new(big.Rat).Set(x)
		}
	}
	c.Partition.Fragments = c.Partition.Traversal()
	for i, h := range c.Partition.HoleFragments {
		q := make([][]AlgebraicSubdivisionUse, len(h))
		for j, l := range h {
			q[j] = append([]AlgebraicSubdivisionUse(nil), l...)
		}
		c.Partition.HoleFragments[i] = q
	}
	copyInterval := func(x geom.ExactParameterInterval) geom.ExactParameterInterval {
		if x.Lower != nil {
			x.Lower = new(big.Rat).Set(x.Lower)
		}
		if x.Upper != nil {
			x.Upper = new(big.Rat).Set(x.Upper)
		}
		return x
	}
	for i, x := range c.Partition.CutUV {
		x.Parameter = copyInterval(x.Parameter)
		for j, p := range x.Coordinates {
			x.Coordinates[j] = copyInterval(p)
		}
		c.Partition.CutUV[i] = x
	}
	c.Model.Cells = append([]*geom.ExactComposedCurveCell(nil), c.Model.Cells...)
	c.Model.Events = append([]geom.ExactParameterInterval(nil), c.Model.Events...)
	for i, x := range c.Model.Events {
		c.Model.Events[i] = copyInterval(x)
	}
	return c
}

// BuildAlgebraicPartitionCurve independently reproves the source partition,
// derives exact image roots of both original UV terminal parameters, and
// composes the complete model cut on its original spline surface.
func (s AlgebraicSubdivision) BuildAlgebraicPartitionCurve(ctx context.Context, face int, line [3]*big.Rat,
	o AlgebraicSeamOptions) (AlgebraicPartitionCurve, error) {
	var empty AlgebraicPartitionCurve
	p, e := s.PartitionFaceByAlgebraicUVLine(ctx, face, line, o)
	if e != nil {
		return empty, e
	}
	out := AlgebraicPartitionCurve{Partition: p, Axis: p.parameterAxis}
	lim := o.Root.ExactCurvePlaneEventOptions
	if lim.MaxOperations == 0 {
		lim.MaxOperations = 2000000
	}
	if lim.MaxOperations < 1 {
		return empty, geom.ErrInvalid
	}
	for i, ev := range p.terminal {
		if lim.MaxOperations < 1 {
			return empty, geom.ErrConvergence
		}
		x, ops, e := ev.CoordinateRoot(ctx, out.Axis, lim)
		lim.MaxOperations -= ops
		if e != nil {
			return empty, e
		}
		out.Endpoints[i] = x
	}
	if lim.MaxOperations < 1 {
		return empty, geom.ErrConvergence
	}
	order, e := out.Endpoints[0].Compare(ctx, out.Endpoints[1], lim)
	if e != nil {
		return empty, e
	}
	if !order.Proven || order.Order != -1 {
		return empty, geom.ErrConvergence
	}
	zero, one := new(big.Rat), big.NewRat(1, 1)
	origin, direction := [2]*big.Rat{zero, zero}, [2]*big.Rat{zero, zero}
	axis, other := out.Axis, out.Axis^1
	origin[other] = new(big.Rat).Neg(new(big.Rat).Quo(p.Line[2], p.Line[other]))
	direction[axis], direction[other] = one, new(big.Rat).Neg(new(big.Rat).Quo(p.Line[axis], p.Line[other]))
	support, e := exactFaceSupportPatch(s.source.faces[face], o.Composition.SurfacePatchOptions)
	if e != nil {
		return empty, e
	}
	out.Model, e = support.BuildAffineSection(ctx, origin, direction, out.Endpoints[0], out.Endpoints[1], o.Composition)
	if e != nil {
		return empty, e
	}
	if !out.Model.Complete || len(out.Model.Cells) == 0 {
		return empty, geom.ErrConvergence
	}
	return out, nil
}

// TessellateAlgebraicPartitionCutCertified independently verifies the same
// chosen display point at each existing topology vertex. It returns one
// model polyline for both oppositely oriented fragment uses.
func (s AlgebraicSubdivision) TessellateAlgebraicPartitionCutCertified(ctx context.Context, face int, line [3]*big.Rat,
	o AlgebraicSeamOptions, display geom.ComposedCurvePolylineOptions) (AlgebraicPartitionCurve, geom.CertifiedComposedCollectionPolyline, error) {
	var empty AlgebraicPartitionCurve
	var none geom.CertifiedComposedCollectionPolyline
	if display.EndpointPoints != nil {
		return empty, none, geom.ErrInvalid
	}
	p, e := s.BuildAlgebraicPartitionCurve(ctx, face, line, o)
	if e != nil {
		return empty, none, e
	}
	display.EndpointPoints = &[2]geom.Vec3{s.vertices[p.Partition.Vertices[0]].Display, s.vertices[p.Partition.Vertices[1]].Display}
	m, e := p.Model.TessellateCertified(ctx, geom.ComposedCollectionPolylineOptions{ComposedCurvePolylineOptions: display})
	if e != nil {
		return empty, none, e
	}
	if !m.Complete {
		return empty, none, geom.ErrConvergence
	}
	if m.DisplayDegenerate {
		return empty, none, geom.ErrDegenerate
	}
	return p, m, nil
}
