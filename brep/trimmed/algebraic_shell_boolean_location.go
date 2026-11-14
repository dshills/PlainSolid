package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// LocateMaterial locates a represented model point in the qualified Boolean
// from the original operand boundaries. Absent or buried operand boundaries
// do not count as result boundaries. A point proved on both operands lies on
// a regular transverse section: that pairing was certified by construction.
// A point near an unsupported contact or unresolved ray returns an error.
func (s AlgebraicShellBoolean) LocateMaterial(ctx context.Context, p geom.Vec3, o SplineShellBooleanOptions) (parametric.Location, error) {
	if ctx == nil || s.source == nil || !p.IsFinite() || o.MaxRays < 0 || o.MaxRays > 12 {
		return parametric.Outside, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return parametric.Outside, e
	}
	if o.MaxRays == 0 {
		o.MaxRays = 12
	}
	if len(s.faces) == 0 {
		return parametric.Outside, nil
	}
	var loc [2]parametric.Location
	for i, operand := range [2]Solid{s.source.a, s.source.b} {
		v, e := locateOriginalBooleanOperand(ctx, operand, s.source.bounds[i], p, o)
		if e != nil {
			return parametric.Outside, fmt.Errorf("trimmed: locate Boolean operand %d: %w", i, e)
		}
		loc[i] = v
		if v == parametric.Outside && (s.op == parametric.Intersection || s.op == parametric.Difference && i == 0) {
			return parametric.Outside, nil
		}
		if s.op == parametric.Union && v == parametric.Inside {
			return parametric.Inside, nil
		}
	}
	a, b := loc[0], loc[1]
	if s.op == parametric.Difference {
		switch b {
		case parametric.Outside:
			b = parametric.Inside
		case parametric.Inside:
			b = parametric.Outside
		}
	}
	if s.op == parametric.Union {
		if a == parametric.Boundary || b == parametric.Boundary {
			return parametric.Boundary, nil
		}
		return parametric.Outside, nil
	}
	if a == parametric.Outside || b == parametric.Outside {
		return parametric.Outside, nil
	}
	if a == parametric.Boundary || b == parametric.Boundary {
		return parametric.Boundary, nil
	}
	return parametric.Inside, nil
}

func locateOriginalBooleanOperand(ctx context.Context, s Solid, bounds geom.Box, p geom.Vec3, o SplineShellBooleanOptions) (parametric.Location, error) {
	if len(s.faces) == 0 || !bounds.Contains(p) {
		return parametric.Outside, nil
	}
	for _, x := range s.vertices {
		if x == p {
			return parametric.Boundary, nil
		}
	}
	query := [3]*big.Rat{ratInset(p.X), ratInset(p.Y), ratInset(p.Z)}
	for i := range s.faces {
		q, e := s.LocateRationalOnSplineFace(ctx, i, query, SplineFacePointOptions{Patch: o.Embedding.Patches, Trim: o.Ray.Trim})
		if e != nil && !errors.Is(e, geom.ErrConvergence) {
			return parametric.Outside, e
		}
		if e == nil && (q.Location == SplineFacePointInterior || q.Location == SplineFacePointOnTrim) {
			return parametric.Boundary, nil
		}
	}
	inside, e := classifySeparatedSplineWitness(ctx, s, bounds, p, o)
	if inside {
		return parametric.Inside, e
	}
	return parametric.Outside, e
}
