package step

import (
	"context"
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

func (g *mappedChainStepSource) proveForeignRoofUse(ctx context.Context, edgeID int64, edge mappedChainStepEdge, owner int, o geom.SurfacePatchOptions) (trimmed.MappedSplineUV, error) {
	var zero trimmed.MappedSplineUV
	basis, err := g.ownerBasis(edge.pcurves[owner].curve)
	if err != nil {
		return zero, err
	}
	reverse := !g.uses[edgeID][owner].sense
	model := edge.model
	if reverse {
		model, err = model.Reparameterize(0, 1, true, o)
		if err != nil {
			return zero, err
		}
	}
	lo, hi := basis.Domain()
	source := trimmed.SplineUV{Curve: basis, Range: trimmed.UVInterval{Min: lo, Max: hi}, Reversed: reverse}
	return trimmed.ProveMappedSplineTransfer(ctx, g.faces[owner].surface, source, model, o)
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
