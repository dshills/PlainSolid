package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SplineShellRepeatedEllipseOptions selects a face pair for exact closed
// double contact. With no hint the ellipse is derived from the original
// supports. Supply at most one hint in the first original face's UV.
// Projection orders the model axes for height comparison and defaults to XY.
type SplineShellRepeatedEllipseOptions struct {
	Faces, Projection [2]int
	Hint              geom.GraphEllipseHint
	QuadraticHint     geom.GraphQuadraticEllipseHint
	Shells            SplineShellBooleanOptions
	Contact           geom.GraphPointContactOptions
}

// SplineShellRepeatedEllipse retains both original face IDs and the exact
// affine map from each point of the first face's contact ellipse to the
// second original chart. Height signs refer to the omitted model axis.
type SplineShellRepeatedEllipse struct {
	Faces, Projection [2]int
	// Contact is populated for the axis-aligned certificate. QuadraticContact
	// is populated for the general definite conic certificate.
	Contact                         geom.SplineGraphEllipse
	QuadraticContact                geom.SplineGraphQuadraticEllipse
	MapAtoB                         [2][3]*big.Rat
	SeparatedPairs, ExactOperations int
	// Enclosing selects the original operand of a proved nested contact,
	// or is -1 for an exterior contact.
	Enclosing int
}

type tangentEllipseRelation uint8

const (
	exteriorTangentEllipse tangentEllipseRelation = iota
	containedTangentEllipse
	discoverTangentEllipseRelation
)

// CombineExternallyTangentSplineShellEllipse proves a closed curve of double
// contact between two exterior single shells. Every other original face pair
// must be strictly separated. Common is empty and Cut retains the original
// stock boundary. The contacting union has no single manifold boundary in
// this arrangement and reports ErrConvergence.
func CombineExternallyTangentSplineShellEllipse(ctx context.Context, a, b Solid, operation parametric.BooleanOp,
	o SplineShellRepeatedEllipseOptions) (Solid, SplineShellCombineHistory, SplineShellRepeatedEllipse, error) {
	return combineTangentSplineShellEllipse(ctx, [2]Solid{a, b}, [2][]splineEmbeddingFace{}, operation, o, exteriorTangentEllipse)
}

// CombineInternallyTangentSplineShellEllipse proves one original shell
// contains the other with contact only on a closed double ellipse. Common
// retains the inner shell, Fuse retains the enclosing shell, and cutting
// the inner shell by its enclosure gives empty material. An enclosing-stock
// cut would leave a cavity meeting its boundary and requires a different
// representation of the singular result.
func CombineInternallyTangentSplineShellEllipse(ctx context.Context, a, b Solid, operation parametric.BooleanOp,
	o SplineShellRepeatedEllipseOptions) (Solid, SplineShellCombineHistory, SplineShellRepeatedEllipse, error) {
	return combineTangentSplineShellEllipse(ctx, [2]Solid{a, b}, [2][]splineEmbeddingFace{}, operation, o, containedTangentEllipse)
}

// Supplied embeddings are the exact original snapshots on which the ordinary
// dispatch has already separated every pair except the selected one.
func combineTangentSplineShellEllipse(ctx context.Context, solids [2]Solid, all [2][]splineEmbeddingFace, operation parametric.BooleanOp,
	o SplineShellRepeatedEllipseOptions, relation tangentEllipseRelation) (out Solid, history SplineShellCombineHistory, proof SplineShellRepeatedEllipse, err error) {
	if ctx == nil || operation > parametric.Difference || o.Shells.MaxRays < 0 || o.Shells.MaxRays > 12 || o.Contact.MaxOperations < 0 ||
		o.Contact.MaxCoefficientBits != 0 && o.Contact.MaxCoefficientBits < 64 || o.Contact.MaxCells < 0 || o.Contact.MaxDepth < 0 {
		return out, history, proof, geom.ErrInvalid
	}
	axisHint := o.Hint != (geom.GraphEllipseHint{})
	quadraticHint := o.QuadraticHint != (geom.GraphQuadraticEllipseHint{})
	if axisHint && quadraticHint {
		return out, history, proof, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, history, proof, e
	}
	if operation == parametric.Union && relation == exteriorTangentEllipse {
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
	a, b := solids[0], solids[1]
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
	prevalidated := len(all[0]) != 0 && len(all[1]) != 0
	var source [2]*geom.SurfacePatch
	var outward [2]int
	for i, s := range solids {
		face := o.Faces[i]
		if face < 0 || face >= len(s.faces) {
			return out, history, proof, geom.ErrInvalid
		}
		if !prevalidated {
			if _, e := s.validateSplineRectangleEmbedding(ctx, o.Shells.Embedding, &all[i]); e != nil {
				return out, history, proof, fmt.Errorf("trimmed: tangent operand %d embedding: %w", i, e)
			}
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
	internal := outward[0] == outward[1]
	if (relation != discoverTangentEllipseRelation && (relation == containedTangentEllipse) != internal) ||
		(operation == parametric.Union && !internal) || limit().MaxOperations < 1 {
		return out, history, proof, geom.ErrConvergence
	}
	align := func() (*geom.AffineXYChartAlignment, error) {
		p := limit()
		if p.MaxOperations < 1 {
			return nil, geom.ErrConvergence
		}
		x, used, e := geom.AlignAffineXYChartsWithWork(ctx, source[0], source[1], p)
		proof.ExactOperations += used
		return x, e
	}
	alignment, e := align()
	if errors.Is(e, geom.ErrDomain) {
		p := limit()
		if p.MaxOperations < 1 {
			return out, history, proof, geom.ErrConvergence
		}
		x, clipErr := geom.ClipAffineXYGraphOverlap(ctx, source[0], source[1], p)
		proof.ExactOperations += x.ExactOperations
		if clipErr != nil {
			return out, history, proof, fmt.Errorf("trimmed: tangent chart overlap: %w", clipErr)
		}
		if x.Disjoint {
			return out, history, proof, geom.ErrConvergence
		}
		for i, box := range [2]geom.ParameterRectangle{x.A, x.B} {
			if source[i].Domain() == box {
				continue
			}
			if limit().MaxOperations < 1 {
				return out, history, proof, geom.ErrConvergence
			}
			restricted, used, e := source[i].TrimWithWorkLimit(box, limit().MaxOperations)
			proof.ExactOperations += used
			if e != nil {
				return out, history, proof, fmt.Errorf("trimmed: tangent chart %d overlap restriction: %w", i, e)
			}
			source[i] = restricted
		}
		alignment, e = align()
	}
	if e != nil {
		return out, history, proof, fmt.Errorf("trimmed: tangent chart alignment: %w", e)
	}
	child := o.Contact
	child.MaxOperations = limit().MaxOperations
	if child.MaxOperations < 1 {
		return out, history, proof, geom.ErrConvergence
	}
	var proven bool
	var inside, outside int
	if !quadraticHint {
		var contact geom.SplineGraphEllipse
		if axisHint {
			contact, e = geom.CertifySplineGraphRepeatedEllipse(ctx, source[0], alignment.TargetOnSource(), o.Hint, child)
		} else {
			contact, e = geom.DiscoverSplineGraphRepeatedEllipse(ctx, source[0], alignment.TargetOnSource(), child)
		}
		proof.ExactOperations += contact.ExactOperations
		proven, inside, outside = contact.Proven, contact.InsideSide, contact.OutsideSide
		proof.Contact = contact
	}
	if quadraticHint || (!axisHint && errors.Is(e, geom.ErrConvergence)) {
		child.MaxOperations = limit().MaxOperations
		if child.MaxOperations < 1 {
			return out, history, proof, geom.ErrConvergence
		}
		var contact geom.SplineGraphQuadraticEllipse
		if quadraticHint {
			contact, e = geom.CertifySplineGraphRepeatedQuadraticEllipse(ctx, source[0], alignment.TargetOnSource(), o.QuadraticHint, child)
		} else {
			contact, e = geom.DiscoverSplineGraphRepeatedQuadraticEllipse(ctx, source[0], alignment.TargetOnSource(), child)
		}
		proof.ExactOperations += contact.ExactOperations
		proven, inside, outside = contact.Proven, contact.InsideSide, contact.OutsideSide
		proof.Contact, proof.QuadraticContact = geom.SplineGraphEllipse{}, contact
	}
	if e != nil {
		return out, history, proof, fmt.Errorf("trimmed: tangent ellipse certificate: %w", e)
	}
	if !proven || inside != outside || (!internal && inside != -outward[0]) {
		return out, history, proof, geom.ErrConvergence
	}
	for i, f := range all[0] {
		for j, g := range all[1] {
			if [2]int{i, j} == o.Faces {
				continue
			}
			if !prevalidated {
				if e := proveSplinePatchPairDisjoint(ctx, f.patch, g.patch, o.Shells.Separation); e != nil {
					return out, history, proof, fmt.Errorf("trimmed: repeated ellipse unseparated original face pair %d,%d: %w", i, j, e)
				}
			}
			proof.SeparatedPairs++
		}
	}
	mapAtoB, e := alignment.MapCoefficients()
	if e != nil {
		return out, history, proof, e
	}
	proof.Faces, proof.Projection, proof.MapAtoB = o.Faces, axes, mapAtoB
	proof.Enclosing = -1
	if !internal {
		if e := proveExternalSplineShellWitnesses(ctx, solids, all, o.Shells); e != nil {
			return out, history, proof, e
		}
		out, history = isolatedGraphTouchResult(a, b, operation)
		history.Method = "repeated-ellipse-tangency"
		return out, history, proof, nil
	}
	outer := 0
	if inside != outward[0] {
		outer = 1
	}
	for i, s := range solids {
		other := 1 - i
		inside, e := classifySeparatedSplineWitness(ctx, solids[other], splineShellPatchBounds(all[other]), s.vertices[0], o.Shells)
		if e != nil {
			return out, history, proof, fmt.Errorf("trimmed: tangent operand %d nesting witness: %w", i, e)
		}
		if inside != (i != outer) {
			return out, history, proof, geom.ErrConvergence
		}
	}
	proof.Enclosing = outer
	if operation == parametric.Difference && outer == 0 {
		return out, history, proof, geom.ErrConvergence
	}
	history = SplineShellCombineHistory{Method: "contained-repeated-ellipse-tangency", FacesA: make([][]int, len(a.faces)), FacesB: make([][]int, len(b.faces)),
		EdgesA: make([][]int, len(a.edges)), EdgesB: make([][]int, len(b.edges))}
	if operation == parametric.Difference {
		return out, history, proof, nil
	}
	owner := outer
	if operation == parametric.Intersection {
		owner = 1 - outer
	}
	faces := [2][][]int{history.FacesA, history.FacesB}
	edges := [2][][]int{history.EdgesA, history.EdgesB}
	for i := range solids[owner].faces {
		faces[owner][i] = []int{i}
	}
	for i := range solids[owner].edges {
		edges[owner][i] = []int{i}
	}
	out = solids[owner]
	return out, history, proof, nil
}
