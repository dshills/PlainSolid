package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SplineShellTangencyOptions selects one original face pair and its affine
// world projection. The supplied pair is a candidate; all other original
// face pairs must be independently separated. Zero projection means XY.
type SplineShellTangencyOptions struct {
	Faces      [2]int
	Projection [2]int
	Shells     SplineShellBooleanOptions
	Contact    geom.GraphPointContactDiscoveryOptions
}

// SplineShellTangency retains the exactly proved original graph point and
// both original chart coordinates. StrictOrder is the sign of A minus B
// in the residual model coordinate everywhere away from the point.
type SplineShellTangency struct {
	Faces, Projection [2]int
	Point             [3]*big.Rat
	UV                [2][2]*big.Rat
	StrictOrder       int
	SeparatedPairs    int
	ExactOperations   int
}

// CombineExternallyTangentSplineShells handles a sole isolated quadratic
// or definite graph contact of two independently embedded rectangular
// spline shells. Opposite outward graph normals and strict graph order
// over the complete common projection prove no local material overlap at
// the contact. With all other face pairs
// disjoint and original vertex witnesses outside either shell, common is
// empty and cut retains the stock. Union at a single point is not a native
// manifold and returns ErrConvergence.
func CombineExternallyTangentSplineShells(ctx context.Context, a, b Solid, op parametric.BooleanOp,
	o SplineShellTangencyOptions) (Solid, SplineShellCombineHistory, SplineShellTangency, error) {
	var history SplineShellCombineHistory
	var empty SplineShellTangency
	if ctx == nil || op > parametric.Difference || o.Shells.MaxRays < 0 || o.Shells.MaxRays > 12 ||
		o.Contact.MaxOperations < 0 || o.Contact.MaxCoefficientBits != 0 && o.Contact.MaxCoefficientBits < 64 {
		return Solid{}, history, empty, geom.ErrInvalid
	}
	if op == parametric.Union {
		return Solid{}, history, empty, geom.ErrConvergence
	}
	axes := o.Projection
	if axes == [2]int{} {
		axes = [2]int{0, 1}
	}
	if axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] {
		return Solid{}, history, empty, geom.ErrInvalid
	}
	if o.Shells.MaxRays == 0 {
		o.Shells.MaxRays = 12
	}
	if o.Contact.MaxOperations == 0 {
		o.Contact.MaxOperations = 8000000
	}
	solids := [2]Solid{a, b}
	var all [2][]splineEmbeddingFace
	for i, s := range solids {
		if o.Faces[i] < 0 || o.Faces[i] >= len(s.faces) {
			return Solid{}, history, empty, geom.ErrInvalid
		}
		if _, e := s.validateSplineRectangleEmbedding(ctx, o.Shells.Embedding, &all[i]); e != nil {
			return Solid{}, history, empty, e
		}
	}
	if len(all[0])*len(all[1]) > 256 {
		return Solid{}, history, empty, geom.ErrConvergence
	}
	proof := SplineShellTangency{Faces: o.Faces, Projection: axes}
	limit := func() geom.SurfacePatchOptions {
		return geom.SurfacePatchOptions{MaxOperations: o.Contact.MaxOperations - proof.ExactOperations, MaxCoefficientBits: o.Contact.MaxCoefficientBits}
	}
	order := [3]int{axes[0], axes[1], 3 - axes[0] - axes[1]}
	var patches [2]*geom.SurfacePatch
	sign := [2]int{}
	for i := range patches {
		p := all[i][o.Faces[i]].patch
		if axes != [2]int{0, 1} {
			var m geom.Affine
			for j, k := range order {
				m[j][k] = 1
			}
			if limit().MaxOperations < 1 {
				return Solid{}, history, empty, geom.ErrConvergence
			}
			var used int
			var e error
			p, used, e = p.TransformedWithWorkLimit(m, limit().MaxOperations)
			proof.ExactOperations += used
			if e != nil {
				return Solid{}, history, empty, e
			}
		}
		if limit().MaxOperations < 1 {
			return Solid{}, history, empty, geom.ErrConvergence
		}
		g, used, e := geom.NewAffineProjectedGraphWithWork(ctx, p, [2]int{0, 1}, limit())
		proof.ExactOperations += used
		if e != nil {
			return Solid{}, history, empty, e
		}
		sign[i] = g.Orientation() * projectedAxisParity(axes)
		if solids[i].faces[o.Faces[i]].Reversed {
			sign[i] = -sign[i]
		}
		patches[i] = p
	}
	if sign[0] == sign[1] || limit().MaxOperations < 1 {
		return Solid{}, history, empty, geom.ErrConvergence
	}
	align := func() (*geom.AffineXYChartAlignment, error) {
		p := limit()
		if p.MaxOperations < 1 {
			return nil, geom.ErrConvergence
		}
		a, used, e := geom.AlignAffineXYChartsWithWork(ctx, patches[0], patches[1], p)
		proof.ExactOperations += used
		return a, e
	}
	alignment, e := align()
	if errors.Is(e, geom.ErrDomain) {
		// Discard only the part of either original chart with no
		// possible common projection. Analyze the full shared rectangle.
		p := limit()
		if p.MaxOperations < 1 {
			return Solid{}, history, empty, geom.ErrConvergence
		}
		overlap, e := geom.ClipAffineXYGraphOverlap(ctx, patches[0], patches[1], p)
		proof.ExactOperations += min(overlap.ExactOperations, p.MaxOperations)
		if e != nil {
			return Solid{}, history, empty, e
		}
		if overlap.Disjoint {
			return Solid{}, history, empty, geom.ErrConvergence
		}
		for i, d := range [2]geom.ParameterRectangle{overlap.A, overlap.B} {
			if patches[i].Domain() == d {
				continue
			}
			if limit().MaxOperations < 1 {
				return Solid{}, history, empty, geom.ErrConvergence
			}
			var used int
			patches[i], used, e = patches[i].TrimWithWorkLimit(d, limit().MaxOperations)
			proof.ExactOperations += used
			if e != nil {
				return Solid{}, history, empty, e
			}
		}
		alignment, e = align()
		if e != nil {
			return Solid{}, history, empty, e
		}
	} else if e != nil {
		return Solid{}, history, empty, e
	}
	o.Contact.MaxOperations = limit().MaxOperations
	if o.Contact.MaxOperations < 1 {
		return Solid{}, history, empty, geom.ErrConvergence
	}
	point, e := geom.DiscoverSplineGraphPointContact(ctx, patches[0], alignment.TargetOnSource(), o.Contact)
	proof.ExactOperations += point.ExactOperations
	c := point.Contact
	if e != nil {
		return Solid{}, history, empty, e
	}
	if !c.Proven || c.BoundaryMask != 0 || c.Side != -sign[0] {
		return Solid{}, history, empty, geom.ErrConvergence
	}
	for i, f := range all[0] {
		for j, g := range all[1] {
			if i == o.Faces[0] && j == o.Faces[1] {
				continue
			}
			if e := proveSplinePatchPairDisjoint(ctx, f.patch, g.patch, o.Shells.Separation); e != nil {
				return Solid{}, history, empty, fmt.Errorf("trimmed: tangent shell pair %d,%d: %w", i, j, e)
			}
			proof.SeparatedPairs++
		}
	}
	// At an interior contact the source vertex is always disjoint from
	// the other boundary. Each connected shell remains outside it.
	for i, s := range solids {
		j := i ^ 1
		inside, e := classifySeparatedSplineWitness(ctx, solids[j], splineShellPatchBounds(all[j]), s.vertices[0], o.Shells)
		if e != nil || inside {
			if e == nil {
				e = geom.ErrConvergence
			}
			return Solid{}, history, empty, e
		}
	}
	mapping, e := alignment.MapCoefficients()
	if e != nil {
		return Solid{}, history, empty, e
	}
	for i, axis := range order {
		proof.Point[axis] = new(big.Rat).Set(c.Point[i])
	}
	proof.UV[0] = c.Parameters
	for i, row := range mapping {
		proof.UV[1][i] = new(big.Rat).Set(row[2])
		for j, q := range c.Parameters {
			proof.UV[1][i].Add(proof.UV[1][i], new(big.Rat).Mul(row[j], q))
		}
	}
	proof.StrictOrder = c.Side
	native, history := regularizedTouchResult(a, b, op)
	history.Method = "isolated-graph-tangency"
	return native, history, proof, nil
}
