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

func proveMappedChainSTEPTransfer(ctx context.Context, support geom.Surface, source trimmed.SplineUV, edge mappedChainStepEdge, reverse, asynchronous, cellLocal bool, bezierRepairs mappedChainBezierRepairs, maxInferredSupportKnots int, o geom.SurfacePatchOptions) (trimmed.MappedSplineUV, []*big.Rat, error) {
	if len(edge.joins) == 0 {
		model, err := mappedChainSTEPBezierInTraversal(edge.model, reverse, o)
		if err != nil {
			return trimmed.MappedSplineUV{}, nil, err
		}
		if bezierRepairs&mappedChainBezierSharedPair != 0 {
			return trimmed.ProveMappedSplineInferredSupportTransfer(ctx, support, source, model, o)
		}
		if asynchronous || bezierRepairs != 0 {
			prove := trimmed.ProveMappedSplineAsynchronousTransfer
			if cellLocal {
				prove = trimmed.ProveMappedSplineCellLocalAsynchronousTransfer
			}
			q, cut, err := prove(ctx, support, source, model, o)
			if cut == nil {
				return q, nil, err
			}
			return q, []*big.Rat{cut}, err
		}
		q, err := trimmed.ProveMappedSplineTransfer(ctx, support, source, model, o)
		return q, nil, err
	}
	pieces, err := mappedChainSTEPPiecesInTraversal(edge.pieces, reverse, o)
	if err != nil {
		return trimmed.MappedSplineUV{}, nil, err
	}
	joins := mappedChainModelJoins(edge.joins, reverse)
	if len(joins) == 1 {
		if cellLocal {
			return trimmed.ProveMappedSplineCellLocalStoredKnotTransfer(ctx, support, source, [2]*geom.ExactSplineCurve(pieces), joins[0], o)
		}
		if maxInferredSupportKnots != 0 {
			return trimmed.ProveMappedSplineSingleStoredKnotTransfer(ctx, support, source, [2]*geom.ExactSplineCurve(pieces), joins[0], maxInferredSupportKnots, o)
		}
		q, err := trimmed.ProveMappedSplineSupportKnotTransfer(ctx, support, source, [2]*geom.ExactSplineCurve(pieces), joins[0], o)
		return q, nil, err
	}
	q, err := trimmed.ProveMappedSplineTwoSupportKnotsTransfer(ctx, support, source, [3]*geom.ExactSplineCurve(pieces), [2]*big.Rat(joins), o)
	return q, nil, err
}

func mappedChainSTEPPiecesInTraversal(source []*geom.ExactSplineCurve, reverse bool, o geom.SurfacePatchOptions) ([]*geom.ExactSplineCurve, error) {
	if !reverse {
		return source, nil
	}
	pieces := make([]*geom.ExactSplineCurve, len(source))
	for i := range pieces {
		var err error
		pieces[i], err = source[len(pieces)-1-i].Reparameterize(0, 1, true, o)
		if err != nil {
			return nil, err
		}
	}
	return pieces, nil
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

type mappedChainBezierRepairs uint8

const (
	mappedChainBezierAsynchronous mappedChainBezierRepairs = 1 << iota
	mappedChainBezierShared
	mappedChainBezierSharedPair
)

func (g *mappedChainStepSource) edgeCrossings(ctx context.Context, id int64, edge mappedChainStepEdge, reverse bool, interval [2]*big.Rat, mapped [2]trimmed.MappedSplineUV, ownerCuts [2][]*big.Rat, bezierRepairs mappedChainBezierRepairs, allowThree bool, v8Cuts int, o geom.SurfacePatchOptions) ([]MappedChainSTEPSupportCrossing, error) {
	joins := mappedChainModelJoins(edge.joins, reverse)
	type cut struct {
		t      *big.Rat
		owners [2]bool
		stored bool
	}
	var cuts []cut
	for _, join := range joins {
		cuts = append(cuts, cut{join, [2]bool{true, true}, true})
	}
	a, b := len(ownerCuts[0]), len(ownerCuts[1])
	sharedCount := 0
	switch {
	case bezierRepairs&mappedChainBezierSharedPair != 0:
		sharedCount = 2
	case bezierRepairs&mappedChainBezierShared != 0:
		sharedCount = 1
	}
	switch n := a + b; {
	case sharedCount != 0 && a == sharedCount && a == b && len(joins) == 0 &&
		slices.EqualFunc(ownerCuts[0], ownerCuts[1], func(a, b *big.Rat) bool { return a.Cmp(b) == 0 }):
		// Each owner already proved its exact raw restrictions against this
		// Bézier. Equal model stations define the shared partition.
		for _, station := range ownerCuts[0] {
			cuts = append(cuts, cut{t: station, owners: [2]bool{true, true}})
		}
	case sharedCount == 2 && a == 2 && b == 2 && len(joins) == 0 && allowThree:
		for owner, row := range ownerCuts {
			for _, station := range row {
				i := slices.IndexFunc(cuts, func(c cut) bool { return c.t.Cmp(station) == 0 })
				if i < 0 {
					cuts = append(cuts, cut{t: station, owners: [2]bool{owner == 0, owner == 1}})
				} else {
					cuts[i].owners[owner] = true
				}
			}
		}
		if len(cuts) != 3 {
			return nil, fmt.Errorf("the original model Bézier requires one shared support event and one separate event per roof: %w", geom.ErrConvergence)
		}
		slices.SortFunc(cuts, func(a, b cut) int { return a.t.Cmp(b.t) })
		stations := [3]*big.Rat{cuts[0].t, cuts[1].t, cuts[2].t}
		model, err := mappedChainSTEPBezierInTraversal(edge.model, reverse, o)
		if err != nil {
			return nil, err
		}
		for owner, curve := range mapped {
			face := g.faces[owner]
			if err := trimmed.ProveMappedSplineBezierThreeSupportPartition(ctx, face.surface, curve, model, stations, o); err != nil {
				return nil, fmt.Errorf("owner #%d on the mixed original Bézier partition: %w", face.id, err)
			}
		}
	case sharedCount == 2 && n > 0 || sharedCount == 1 && n > 1 && bezierRepairs&mappedChainBezierAsynchronous == 0:
		return nil, fmt.Errorf("both inferred roof support streams must coincide on the original model Bézier: %w", geom.ErrConvergence)
	case n > 1:
		three := allowThree && a == 1 && b == 1
		multiple := n+1 == v8Cuts && min(a, b) >= 1
		singleBezier := bezierRepairs&mappedChainBezierAsynchronous != 0 && len(joins) == 0 && a == 1 && b == 1
		if !singleBezier && (len(joins) != 1 || !three && !multiple) {
			return nil, fmt.Errorf("asynchronous repair permits one owner-side crossing per original edge: %w", geom.ErrConvergence)
		}
		var extra []*big.Rat
		for owner, row := range ownerCuts {
			for _, station := range row {
				if slices.ContainsFunc(extra, func(prior *big.Rat) bool { return prior.Cmp(station) == 0 }) {
					return nil, fmt.Errorf("separate inferred support stations must be distinct: %w", geom.ErrConvergence)
				}
				extra = append(extra, station)
				cuts = append(cuts, cut{t: station, owners: [2]bool{owner == 0, owner == 1}})
			}
		}
		if singleBezier {
			model, err := mappedChainSTEPBezierInTraversal(edge.model, reverse, o)
			if err != nil {
				return nil, err
			}
			slices.SortFunc(extra, (*big.Rat).Cmp)
			for owner, curve := range mapped {
				face := g.faces[owner]
				if err := trimmed.ProveMappedSplineBezierSupportPartition(ctx, face.surface, curve, model, [2]*big.Rat(extra), o); err != nil {
					return nil, fmt.Errorf("owner #%d on the paired original Bézier partition: %w", face.id, err)
				}
			}
			break
		}
		pieces, err := mappedChainSTEPPiecesInTraversal(edge.pieces, reverse, o)
		if err != nil {
			return nil, err
		}
		for owner, curve := range mapped {
			face := g.faces[owner]
			if err := trimmed.ProveMappedSplineStoredKnotPartition(ctx, face.surface, curve, [2]*geom.ExactSplineCurve(pieces), joins[0], extra, o); err != nil {
				partition := "three-cut"
				if multiple {
					partition = fmt.Sprintf("%d-cut", v8Cuts)
				}
				return nil, fmt.Errorf("owner #%d on the common %s partition: %w", face.id, partition, err)
			}
		}
	case n == 1:
		owner := 0
		if b != 0 {
			owner = 1
		}
		extra := ownerCuts[owner][0]
		companion := 1 - owner
		support := g.faces[companion].surface
		var err error
		switch len(joins) {
		case 0:
			model, e := mappedChainSTEPBezierInTraversal(edge.model, reverse, o)
			if e != nil {
				return nil, e
			}
			err = trimmed.ProveMappedSplineSplitTransfer(ctx, support, mapped[companion], model, extra, o)
		case 1:
			pieces, e := mappedChainSTEPPiecesInTraversal(edge.pieces, reverse, o)
			if e != nil {
				return nil, e
			}
			err = trimmed.ProveMappedSplineMixedSplitTransfer(ctx, support, mapped[companion], [2]*geom.ExactSplineCurve(pieces), joins[0], extra, o)
		default:
			return nil, geom.ErrConvergence
		}
		if err != nil {
			return nil, fmt.Errorf("companion owner #%d at original support crossing: %w", g.faces[companion].id, err)
		}
		cuts = append(cuts, cut{t: extra, owners: [2]bool{owner == 0, owner == 1}})
	}
	slices.SortFunc(cuts, func(a, b cut) int { return a.t.Cmp(b.t) })
	var crossings []MappedChainSTEPSupportCrossing
	for _, cut := range cuts {
		t := cut.t
		fileParameter := t
		if reverse {
			fileParameter = new(big.Rat).Sub(big.NewRat(1, 1), t)
		}
		original := new(big.Rat).Add(edge.lo, new(big.Rat).Mul(fileParameter, new(big.Rat).Sub(edge.hi, edge.lo)))
		station := new(big.Rat).Add(interval[0], new(big.Rat).Mul(t, new(big.Rat).Sub(interval[1], interval[0])))
		crossing := MappedChainSTEPSupportCrossing{
			EdgeID: id, SourceModelParameter: original, ModelStation: station, OwnersChangingCells: cut.owners, StoredModelKnot: cut.stored,
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
