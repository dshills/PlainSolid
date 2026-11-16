package trimmed

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// BuildProjectiveSplineRoofBoolean builds a regularized material result
// from two complete original graphs above a shared planar base. A Common
// uses the exact rectangular overlap. Fuse also accepts fully contained
// material, and Cut accepts a tool whose footprint covers the entire
// stock or has no positive area in common with it. Material uses proven
// exact height order or one complete open analytic section.
func BuildProjectiveSplineRoofBoolean(ctx context.Context, a, b Face, axes [2]int, baseHeight float64,
	op parametric.BooleanOp, o ProjectiveRoofSolidOptions) (ProjectiveRoofSolid, error) {
	if op == parametric.Intersection {
		return BuildProjectiveSplineRoofOverlapCommon(ctx, a, b, axes, baseHeight, o)
	}
	r, err := buildProjectiveSplineRoofBooleanOnEqualFootprints(ctx, a, b, axes, baseHeight, op, o)
	if err == nil || (!errors.Is(err, geom.ErrConvergence) && !errors.Is(err, geom.ErrDomain)) {
		return r, err
	}
	return buildProjectiveSplineRoofContainedMaterial(ctx, a, b, axes, baseHeight, op, o)
}

func buildProjectiveSplineRoofBooleanOnEqualFootprints(ctx context.Context, a, b Face, axes [2]int, baseHeight float64,
	op parametric.BooleanOp, o ProjectiveRoofSolidOptions) (ProjectiveRoofSolid, error) {
	r, err := BuildOrderedProjectiveSplineRoofSolid(ctx, a, b, axes, baseHeight, op, o)
	if err == nil || !errors.Is(err, geom.ErrConvergence) {
		return r, err
	}
	if err := ctx.Err(); err != nil {
		return ProjectiveRoofSolid{}, err
	}
	if op == parametric.Difference {
		return BuildProjectiveSplineRoofContourDifference(ctx, a, b, axes, baseHeight, o)
	}
	return BuildProjectiveSplineRoofContourSolid(ctx, a, b, axes, baseHeight, op, o)
}

func emptyProjectiveRoofBoolean() ProjectiveRoofSolid {
	return ProjectiveRoofSolid{Roofs: [2]int{-1, -1}, BaseFace: -1, Section: -1, Complete: true, Empty: true}
}

func buildProjectiveSplineRoofContainedMaterial(ctx context.Context, a, b Face, axes [2]int, baseHeight float64,
	op parametric.BooleanOp, o ProjectiveRoofSolidOptions) (r ProjectiveRoofSolid, err error) {
	p, err := proveProjectiveRoofFootprints(ctx, a, b, axes, baseHeight, o)
	if err != nil {
		return r, err
	}
	originalStock := func(winner int) (ProjectiveRoofSolid, error) {
		f := p.owners[winner]
		q, err := BuildOrderedProjectiveSplineRoofSolid(ctx, f, f, axes, baseHeight, parametric.Union, o)
		if err != nil {
			return ProjectiveRoofSolid{}, err
		}
		if winner == 1 {
			q.Roofs[0], q.Roofs[1] = q.Roofs[1], q.Roofs[0]
			for i := range q.Walls {
				q.Walls[i].Owner = winner
			}
		}
		return q, nil
	}
	if p.overlap.NoArea {
		if op == parametric.Difference {
			return originalStock(0)
		}
		return r, geom.ErrConvergence // A single-solid fuse cannot carry disjoint material.
	}
	var covered [2]bool
	for i, s := range p.overlap.Areas {
		d := p.supports[i].Domain()
		covered[i] = s.U.Lower.Cmp(new(big.Rat).SetFloat64(d.UMin)) == 0 && s.U.Upper.Cmp(new(big.Rat).SetFloat64(d.UMax)) == 0 &&
			s.V.Lower.Cmp(new(big.Rat).SetFloat64(d.VMin)) == 0 && s.V.Upper.Cmp(new(big.Rat).SetFloat64(d.VMax)) == 0
	}
	if covered[0] == covered[1] || op == parametric.Difference && !covered[0] {
		return r, geom.ErrConvergence
	}
	order, err := geom.CertifyProjectiveSurfaceHeightOrder(ctx, p.supports[0], p.supports[1], axes, o.Envelope.Imprint.Contacts.Graph.Cover.Atlas)
	if err != nil && (op == parametric.Union || !errors.Is(err, geom.ErrConvergence)) {
		return r, err
	}
	if op == parametric.Union {
		switch {
		case covered[0] && order.Complete && order.Sign <= 0:
			return originalStock(1)
		case covered[1] && order.Complete && order.Sign >= 0:
			return originalStock(0)
		default:
			return r, geom.ErrConvergence
		}
	}
	if order.Complete && order.Sign <= 0 {
		return emptyProjectiveRoofBoolean(), nil
	}
	faces, ledger, err := p.clip()
	if err != nil {
		return r, err
	}
	q, err := buildProjectiveSplineRoofBooleanOnEqualFootprints(ctx, faces[0], faces[1], axes, baseHeight, parametric.Difference, o)
	if err != nil {
		return r, err
	}
	if err := restoreProjectiveRoofCoedges(ctx, &q, ledger); err != nil {
		return r, err
	}
	return q, nil
}
