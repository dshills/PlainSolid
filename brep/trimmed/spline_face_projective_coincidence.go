package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func certifyProjectiveSplineFaceWires(ctx context.Context, faces [2]int, a, b *geom.SurfacePatch,
	loops [2][][]Use, reversed bool, o SplineFaceCoincidenceOptions) (SplineFaceCoincidence, error) {
	fail := func(err error) (SplineFaceCoincidence, error) { return SplineFaceCoincidence{}, err }
	if o.MatchHolePermutation {
		return fail(geom.ErrDomain)
	}
	for i, loop := range loops[0] {
		if len(loop) == 0 || len(loop) != len(loops[1][i]) {
			return fail(geom.ErrDomain)
		}
	}
	work := 0
	options := func() geom.SurfacePatchOptions {
		p := o.Patches
		p.MaxOperations = o.MaxOperations - work
		if o.Patches.MaxOperations > 0 {
			p.MaxOperations = min(p.MaxOperations, o.Patches.MaxOperations)
		}
		return p
	}
	proof, err := geom.CertifyProjectiveSurfaceGraphOverlap(ctx, a, b, *o.ProjectiveAxes, options())
	if err != nil {
		return fail(err)
	}
	if !proof.Coincident {
		return fail(geom.ErrConvergence)
	}
	work += proof.ExactOperations
	if work >= o.MaxOperations {
		return fail(parametric.ErrBudget)
	}
	mapUV, used, err := proof.OriginalUVMapWithWork(options())
	if err != nil {
		return fail(err)
	}
	work += used
	r := SplineFaceCoincidence{Faces: faces, ProjectiveMap: &proof, OppositeOrientation: (proof.Orientation < 0) != reversed}
	for loopID := range loops[0] {
		type coedge struct {
			curve   *geom.ExactSplineCurve
			reverse bool
		}
		var coedges [2][]coedge
		for owner, owners := range loops {
			for _, u := range owners[loopID] {
				if work >= o.MaxOperations {
					return fail(parametric.ErrBudget)
				}
				c, reverse, err := coincidentUseCurve(ctx, u, options())
				if err != nil {
					return fail(err)
				}
				work += c.ExactOperations()
				if work >= o.MaxOperations {
					return fail(parametric.ErrBudget)
				}
				inside, used, err := c.UVControlHullInsideRational(ctx, proof.Areas[owner], options())
				work += used
				if err != nil {
					return fail(err)
				}
				if !inside {
					return fail(geom.ErrDomain)
				}
				if owner == 0 {
					if work >= o.MaxOperations {
						return fail(parametric.ErrBudget)
					}
					c, err = c.TransformProjectiveUV(ctx, mapUV, options())
					if err != nil {
						return fail(err)
					}
					work += c.ExactOperations()
				}
				coedges[owner] = append(coedges[owner], coedge{c, reverse})
			}
		}
		count := len(coedges[0])
		matched := false
		for start := range count {
			uses := make([]CoincidentSplineUse, 0, count)
			for i, use := range coedges[0] {
				j := (start + proof.Orientation*i + count) % count
				other := coedges[1][j]
				if work >= o.MaxOperations {
					return fail(parametric.ErrBudget)
				}
				reverse := (use.reverse != other.reverse) != (proof.Orientation < 0)
				identity, err := geom.CompareExactCurvesContext(ctx, use.curve, other.curve, geom.ExactCurveComparisonOptions{
					NormalizeDomains: true, ReverseB: reverse, MaxOperations: options().MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits})
				work += identity.ExactOperations
				if err != nil {
					return fail(err)
				}
				if !identity.Complete || !identity.Equal {
					break
				}
				uses = append(uses, CoincidentSplineUse{LoopA: loopID, UseA: i, EdgeA: loops[0][loopID][i].Edge,
					LoopB: loopID, UseB: j, EdgeB: loops[1][loopID][j].Edge, Reversed: proof.Orientation < 0,
					RangeA: [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}, RangeB: coincidentTraversalRange(proof.Orientation)})
			}
			if len(uses) == count {
				matched = true
				r.Uses = append(r.Uses, uses...)
				break
			}
		}
		if !matched {
			return fail(geom.ErrConvergence)
		}
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	r.Complete, r.ExactOperations = true, work
	return r, nil
}
