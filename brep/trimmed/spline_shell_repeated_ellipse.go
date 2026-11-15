package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SplineShellRepeatedEllipseOptions selects a face pair for exact closed
// double contact. An empty Hint derives the ellipse from the original
// supports. A supplied hint uses the first original face's UV. Projection
// orders the model axes for height comparison and defaults to XY.
type SplineShellRepeatedEllipseOptions struct {
	Faces, Projection [2]int
	Hint              geom.GraphEllipseHint
	Shells            SplineShellBooleanOptions
	Contact           geom.GraphPointContactOptions
}

// SplineShellRepeatedEllipse retains both original face IDs and the exact
// affine map from each point of the first face's contact ellipse to the
// second original chart. Height signs refer to the omitted model axis.
type SplineShellRepeatedEllipse struct {
	Faces, Projection               [2]int
	Contact                         geom.SplineGraphEllipse
	MapAtoB                         [2][3]*big.Rat
	SeparatedPairs, ExactOperations int
}

// CombineExternallyTangentSplineShellEllipse proves a closed curve of double
// contact between two exterior single shells. Every other original face pair
// must be strictly separated. Common is empty and Cut retains the original
// stock boundary. The contacting union has no single manifold boundary in
// this arrangement and reports ErrConvergence.
func CombineExternallyTangentSplineShellEllipse(ctx context.Context, a, b Solid, operation parametric.BooleanOp,
	o SplineShellRepeatedEllipseOptions) (out Solid, history SplineShellCombineHistory, proof SplineShellRepeatedEllipse, err error) {
	if ctx == nil || operation > parametric.Difference || o.Shells.MaxRays < 0 || o.Shells.MaxRays > 12 || o.Contact.MaxOperations < 0 ||
		o.Contact.MaxCoefficientBits != 0 && o.Contact.MaxCoefficientBits < 64 || o.Contact.MaxCells < 0 || o.Contact.MaxDepth < 0 {
		return out, history, proof, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, history, proof, e
	}
	if operation == parametric.Union {
		return out, history, proof, geom.ErrConvergence
	}
	if o.Shells.MaxRays == 0 {
		o.Shells.MaxRays = 12
	}
	if o.Contact.MaxOperations == 0 {
		o.Contact.MaxOperations = 8000000
	}
	axes := o.Projection
	if axes == [2]int{} {
		axes = [2]int{0, 1}
	}
	if axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] {
		return out, history, proof, geom.ErrInvalid
	}
	solids := [2]Solid{a, b}
	if len(a.faces)*len(b.faces) > 256 {
		return out, history, proof, geom.ErrConvergence
	}
	defer func() {
		if err != nil {
			proof = SplineShellRepeatedEllipse{ExactOperations: proof.ExactOperations}
		}
	}()
	limit := func() geom.SurfacePatchOptions {
		return geom.SurfacePatchOptions{MaxOperations: o.Contact.MaxOperations - proof.ExactOperations, MaxCoefficientBits: o.Contact.MaxCoefficientBits}
	}
	var all [2][]splineEmbeddingFace
	var source [2]*geom.SurfacePatch
	var outward [2]int
	for i, s := range solids {
		face := o.Faces[i]
		if face < 0 || face >= len(s.faces) {
			return out, history, proof, geom.ErrInvalid
		}
		if _, e := s.validateSplineRectangleEmbedding(ctx, o.Shells.Embedding, &all[i]); e != nil {
			return out, history, proof, e
		}
		p := all[i][face].patch
		if axes != [2]int{0, 1} {
			var m geom.Affine
			for j, axis := range [3]int{axes[0], axes[1], 3 - axes[0] - axes[1]} {
				m[j][axis] = 1
			}
			if limit().MaxOperations < 1 {
				return out, history, proof, geom.ErrConvergence
			}
			q, used, e := p.TransformedWithWorkLimit(m, limit().MaxOperations)
			proof.ExactOperations += used
			if e != nil {
				return out, history, proof, e
			}
			p = q
		}
		if limit().MaxOperations < 1 {
			return out, history, proof, geom.ErrConvergence
		}
		graph, used, e := geom.NewAffineProjectedGraphWithWork(ctx, p, [2]int{0, 1}, limit())
		proof.ExactOperations += used
		if e != nil {
			return out, history, proof, e
		}
		outward[i] = graph.Orientation() * projectedAxisParity(axes)
		if s.faces[face].Reversed {
			outward[i] = -outward[i]
		}
		source[i] = p
	}
	if outward[0] == outward[1] || limit().MaxOperations < 1 {
		return out, history, proof, geom.ErrConvergence
	}
	alignment, used, e := geom.AlignAffineXYChartsWithWork(ctx, source[0], source[1], limit())
	proof.ExactOperations += used
	if e != nil {
		return out, history, proof, e
	}
	child := o.Contact
	child.MaxOperations = limit().MaxOperations
	if child.MaxOperations < 1 {
		return out, history, proof, geom.ErrConvergence
	}
	var contact geom.SplineGraphEllipse
	if o.Hint == (geom.GraphEllipseHint{}) {
		contact, e = geom.DiscoverSplineGraphRepeatedEllipse(ctx, source[0], alignment.TargetOnSource(), child)
	} else {
		contact, e = geom.CertifySplineGraphRepeatedEllipse(ctx, source[0], alignment.TargetOnSource(), o.Hint, child)
	}
	proof.ExactOperations += contact.ExactOperations
	if e != nil {
		return out, history, proof, e
	}
	if !contact.Proven || contact.InsideSide != -outward[0] || contact.OutsideSide != -outward[0] {
		return out, history, proof, geom.ErrConvergence
	}
	for i, f := range all[0] {
		for j, g := range all[1] {
			if [2]int{i, j} == o.Faces {
				continue
			}
			if e := proveSplinePatchPairDisjoint(ctx, f.patch, g.patch, o.Shells.Separation); e != nil {
				return out, history, proof, fmt.Errorf("trimmed: repeated ellipse unseparated original face pair %d,%d: %w", i, j, e)
			}
			proof.SeparatedPairs++
		}
	}
	if e := proveExternalSplineShellWitnesses(ctx, solids, all, o.Shells); e != nil {
		return out, history, proof, e
	}
	mapAtoB, e := alignment.MapCoefficients()
	if e != nil {
		return out, history, proof, e
	}
	proof.Faces, proof.Projection, proof.Contact, proof.MapAtoB = o.Faces, axes, contact, mapAtoB
	out, history = isolatedGraphTouchResult(a, b, operation)
	history.Method = "repeated-ellipse-tangency"
	return out, history, proof, nil
}
