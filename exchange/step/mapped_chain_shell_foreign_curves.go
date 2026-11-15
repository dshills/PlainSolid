package step

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func orderMappedChainForeignFaces(faces []mappedChainStepFace) ([]mappedChainStepFace, error) {
	var roof, plane []mappedChainStepFace
	for _, f := range faces {
		if f.plane == nil {
			roof = append(roof, f)
		} else {
			plane = append(plane, f)
		}
	}
	if len(roof) != 2 || len(plane) != 5 {
		return nil, geom.ErrConvergence
	}
	count := map[int64]int{}
	for _, f := range roof {
		for _, u := range f.uses {
			count[u.id]++
		}
	}
	for i, f := range roof {
		start := -1
		for j, u := range f.uses {
			prev := f.uses[(j+len(f.uses)-1)%len(f.uses)]
			if count[u.id] == 2 && count[prev.id] == 1 {
				if start >= 0 {
					return nil, geom.ErrConvergence
				}
				start = j
			}
		}
		if start < 0 {
			return nil, geom.ErrConvergence
		}
		roof[i].uses = append(slices.Clone(f.uses[start:]), f.uses[:start]...)
	}
	return append(roof, plane...), nil
}

func (g *mappedChainStepSource) foreignRoofSource(edge mappedChainStepEdge, owner int, reverse bool) (trimmed.SplineUV, error) {
	basis, err := g.ownerBasis(edge.pcurves[owner].curve)
	if err != nil {
		return trimmed.SplineUV{}, err
	}
	lo, hi := basis.Domain()
	return trimmed.SplineUV{Curve: basis, Range: trimmed.UVInterval{Min: lo, Max: hi}, Reversed: reverse}, nil
}

func mappedChainSTEPBezierInTraversal(model *geom.ExactSplineCurve, reverse bool, o geom.SurfacePatchOptions) (*geom.ExactSplineCurve, error) {
	if reverse {
		return model.Reparameterize(0, 1, true, o)
	}
	return model, nil
}

func proveMappedChainSTEPTransfer(ctx context.Context, support geom.Surface, source trimmed.SplineUV, edge mappedChainStepEdge, reverse, asynchronous bool, o geom.SurfacePatchOptions) (trimmed.MappedSplineUV, *big.Rat, error) {
	if len(edge.joins) == 0 {
		model, err := mappedChainSTEPBezierInTraversal(edge.model, reverse, o)
		if err != nil {
			return trimmed.MappedSplineUV{}, nil, err
		}
		if asynchronous {
			return trimmed.ProveMappedSplineAsynchronousTransfer(ctx, support, source, model, o)
		}
		q, err := trimmed.ProveMappedSplineTransfer(ctx, support, source, model, o)
		return q, nil, err
	}
	pieces, joins := edge.pieces, mappedChainModelJoins(edge.joins, reverse)
	if reverse {
		pieces = make([]*geom.ExactSplineCurve, len(edge.pieces))
		for i := range pieces {
			piece := edge.pieces[len(pieces)-1-i]
			var err error
			pieces[i], err = piece.Reparameterize(0, 1, true, o)
			if err != nil {
				return trimmed.MappedSplineUV{}, nil, err
			}
		}
	}
	if len(joins) == 1 {
		q, err := trimmed.ProveMappedSplineSupportKnotTransfer(ctx, support, source, [2]*geom.ExactSplineCurve(pieces), joins[0], o)
		return q, nil, err
	}
	q, err := trimmed.ProveMappedSplineTwoSupportKnotsTransfer(ctx, support, source, [3]*geom.ExactSplineCurve(pieces), [2]*big.Rat(joins), o)
	return q, nil, err
}

func mappedChainModelJoins(joins []*big.Rat, reverse bool) []*big.Rat {
	if !reverse {
		return joins
	}
	out := make([]*big.Rat, len(joins))
	for i := range out {
		out[i] = new(big.Rat).Sub(big.NewRat(1, 1), joins[len(joins)-1-i])
	}
	return out
}

func (g *mappedChainStepSource) edgeCrossings(ctx context.Context, id int64, edge mappedChainStepEdge, reverse bool, interval [2]*big.Rat, mapped [2]trimmed.MappedSplineUV, ownerCuts [2]*big.Rat, o geom.SurfacePatchOptions) ([]MappedChainSTEPSupportCrossing, error) {
	cuts := mappedChainModelJoins(edge.joins, reverse)
	changed := [2]bool{true, true}
	if ownerCuts[0] != nil || ownerCuts[1] != nil {
		if ownerCuts[0] != nil && ownerCuts[1] != nil {
			return nil, fmt.Errorf("asynchronous repair permits one owner-side crossing per original edge: %w", geom.ErrConvergence)
		}
		owner := 0
		if ownerCuts[1] != nil {
			owner = 1
		}
		cuts = []*big.Rat{ownerCuts[owner]}
		companion := 1 - owner
		changed[companion] = false
		model, err := mappedChainSTEPBezierInTraversal(edge.model, reverse, o)
		if err != nil {
			return nil, err
		}
		if err := trimmed.ProveMappedSplineSplitTransfer(ctx, g.faces[companion].surface, mapped[companion], model, cuts[0], o); err != nil {
			return nil, fmt.Errorf("companion owner #%d at original support crossing: %w", g.faces[companion].id, err)
		}
	}
	var crossings []MappedChainSTEPSupportCrossing
	for _, t := range cuts {
		fileParameter := t
		if reverse {
			fileParameter = new(big.Rat).Sub(big.NewRat(1, 1), t)
		}
		original := new(big.Rat).Add(edge.lo, new(big.Rat).Mul(fileParameter, new(big.Rat).Sub(edge.hi, edge.lo)))
		station := new(big.Rat).Add(interval[0], new(big.Rat).Mul(t, new(big.Rat).Sub(interval[1], interval[0])))
		crossing := MappedChainSTEPSupportCrossing{
			EdgeID: id, SourceModelParameter: original, ModelStation: station, OwnersChangingCells: changed,
		}
		for owner, curve := range mapped {
			raw, err := curve.AuthoredParameter(t)
			if err != nil {
				return nil, err
			}
			crossing.RawParameters[owner] = raw
		}
		crossings = append(crossings, crossing)
	}
	return crossings, nil
}

func (g *mappedChainStepSource) proveForeignModelLine(ctx context.Context, id int64, points [2][3]*big.Rat, o geom.SurfacePatchOptions) (*geom.ExactSplineCurve, error) {
	line, err := readMappedChainForeignLine(g.file, id, 3)
	if err != nil {
		return nil, err
	}
	if err := line.proveDirectedSpan(points[0][:], points[1][:]); err != nil {
		return nil, err
	}
	h := make([][4]*big.Rat, 2)
	for i, p := range points {
		h[i] = [4]*big.Rat{p[0], p[1], p[2], big.NewRat(1, 1)}
	}
	return geom.NewExactHomogeneousBezier(ctx, h, o)
}

func (g *mappedChainStepSource) proveForeignClosingUse(ctx context.Context, face mappedChainStepFace, edge mappedChainStepEdge, i int, reverse bool, o geom.SurfacePatchOptions) (trimmed.PCurve, error) {
	var raw [2][2]*big.Rat
	id := edge.pcurves[i].curve
	if _, line := component(g.file.Entities[id], "LINE"); line {
		if face.plane == nil {
			return nil, geom.ErrConvergence
		}
		l, err := readMappedChainForeignLine(g.file, id, 2)
		if err != nil {
			return nil, err
		}
		for i, point := range edge.points {
			raw[i], err = face.plane.originalUV(point)
			if err != nil {
				return nil, err
			}
		}
		if err := l.proveDirectedSpan(raw[0][:], raw[1][:]); err != nil {
			return nil, err
		}
	} else {
		s, err := readMappedChainStepSpline(g.file, id, 2, 1, 2, true)
		if err != nil {
			return nil, err
		}
		if s.weights[0].Cmp(s.weights[1]) != 0 {
			return nil, geom.ErrConvergence
		}
		raw = [2][2]*big.Rat{[2]*big.Rat(s.poles[0]), [2]*big.Rat(s.poles[1])}
	}
	if face.plane != nil {
		for i, uv := range raw {
			for k, q := range uv {
				raw[i][k] = new(big.Rat).Add(q, face.plane.shift[k])
			}
		}
	}
	if reverse {
		raw[0], raw[1] = raw[1], raw[0]
	}
	h := [3][3]*big.Rat{
		{raw[0][0], raw[0][1], big.NewRat(1, 1)},
		{new(big.Rat).Quo(new(big.Rat).Add(raw[0][0], raw[1][0]), big.NewRat(2, 1)),
			new(big.Rat).Quo(new(big.Rat).Add(raw[0][1], raw[1][1]), big.NewRat(2, 1)), big.NewRat(1, 1)},
		{raw[1][0], raw[1][1], big.NewRat(1, 1)},
	}
	c, err := trimmed.NewExactQuadraticUV(h)
	if err != nil {
		return nil, err
	}
	model, err := trimmed.ExactTrimModelBezier(ctx, face.surface, c, reverse, o)
	if err != nil {
		return nil, err
	}
	equal, err := geom.CompareExactCurvesContext(ctx, model, edge.model, geom.ExactCurveComparisonOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if err != nil {
		return nil, err
	}
	if !equal.Complete || !equal.Equal {
		return nil, geom.ErrConvergence
	}
	return c, nil
}
