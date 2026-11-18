//go:build research

package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ExactStraightSewOptions also bounds discovery comparisons between boundary
// segments. MaxEdges in Sew bounds both the source and subdivided edge arrays.
type ExactStraightSewOptions struct {
	Sew          ExactSewOptions
	MaxPairTests int
}

// ExactSewnSegment maps a positive interval of an original edge to a sewn
// edge. Lower and Upper are exact fractions of its original parameter range.
type ExactSewnSegment struct {
	ExactSewnEdge
	Lower, Upper *big.Rat
}

// ExactSubdivisionSewReport maps every original vertex and model edge. Segments
// for an original edge are ordered by its original increasing parameter.
// InsertedVertices counts source slots added before canonical sewing.
type ExactSubdivisionSewReport struct {
	Vertices, FlippedFaces                   []int
	Segments                                 [][]ExactSewnSegment
	PairTests, Comparisons, InsertedVertices int
}

// ExactStraightSewReport is the subdivision report for straight borders.
type ExactStraightSewReport = ExactSubdivisionSewReport

// SewExactStraightSubdivisions discovers unmatched straight source borders
// with different segmentations. It subdivides only single-use borders with
// exact affine model and UV parameters and positive-length collinear overlap.
// Each new UV and model endpoint must be exactly representable. The original
// trims are proved before splitting, and the resulting graph is independently
// checked by exact sewing. Point contact alone introduces no subdivision.
func SewExactStraightSubdivisions(ctx context.Context, vertices []geom.Vec3, edges []parametric.Edge, faces []Face, o ExactStraightSewOptions) (Solid, ExactStraightSewReport, error) {
	var empty ExactStraightSewReport
	if o.MaxPairTests == 0 {
		o.MaxPairTests = 131072
	}
	if o.MaxPairTests < 1 || o.MaxPairTests > 1048576 {
		return Solid{}, empty, geom.ErrInvalid
	}
	source := Solid{vertices: vertices, edges: edges, faces: faces}
	settings, err := exactSewSource(ctx, source, o.Sew)
	if err != nil {
		return Solid{}, empty, err
	}
	borders, err := affineSewBorders(ctx, source, settings.Curve)
	if err != nil {
		return Solid{}, empty, err
	}
	partitions, tested, err := straightSewPartitions(ctx, borders, len(edges), settings.MaxEdges, o.MaxPairTests)
	if err != nil {
		return Solid{}, empty, err
	}
	return sewSubdividedBorders(ctx, source, settings, partitions, tested, func(edge int, parameters []*big.Rat) ([]geom.Vec3, []geom.Curve, error) {
		return sampleStraightSewBorder(source, edge, parameters)
	})
}

func sewSubdividedBorders(ctx context.Context, source Solid, settings ExactSewOptions, partitions [][]*big.Rat, tested int, sample sewBorderSampler) (Solid, ExactSubdivisionSewReport, error) {
	var empty ExactSubdivisionSewReport
	split, intervals, err := splitSewBorders(ctx, source, partitions, sample)
	if err != nil {
		return Solid{}, empty, err
	}
	if len(split.edges) != len(source.edges) {
		if err = validateExactSewSource(ctx, split, settings); err != nil {
			return Solid{}, empty, err
		}
	}
	solid, report, err := finishExactSew(ctx, split, settings)
	if err != nil {
		return Solid{}, empty, err
	}
	for _, pieces := range intervals {
		for i := range pieces {
			pieces[i].ExactSewnEdge = report.Edges[pieces[i].Edge]
		}
	}
	return solid, ExactSubdivisionSewReport{
		Vertices: slices.Clone(report.Vertices[:len(source.vertices)]), FlippedFaces: report.FlippedFaces, Segments: intervals,
		PairTests: tested, Comparisons: report.Comparisons, InsertedVertices: len(split.vertices) - len(source.vertices),
	}, nil
}

type affineSewBorder struct {
	edge       int
	start, end [3]float64
}

func affineSewBorders(ctx context.Context, s Solid, o geom.SurfacePatchOptions) ([]affineSewBorder, error) {
	uses := make([]int, len(s.edges))
	straight := make([]bool, len(s.edges))
	for _, face := range s.faces {
		for _, loop := range append([][]Use{face.Outer}, face.Holes...) {
			for _, use := range loop {
				uses[use.Edge]++
				straight[use.Edge] = use.Curve == nil
			}
		}
	}
	var result []affineSewBorder
	for i, edge := range s.edges {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if uses[i] != 1 || !straight[i] {
			continue
		}
		p := [2]geom.Vec3{s.vertices[edge.Start], s.vertices[edge.End]}
		actual, rev, err := exactSewCurve(ctx, edge, p, o)
		if err != nil {
			return nil, fmt.Errorf("trimmed: source border %d: %w", i, err)
		}
		line, err := straightSewCurve(p)
		if err != nil {
			return nil, err
		}
		exact, err := geom.NewExactSplineCurve(line, o)
		if err != nil {
			return nil, err
		}
		check, err := geom.CompareExactCurvesContext(ctx, actual, exact, geom.ExactCurveComparisonOptions{
			NormalizeDomains: true, ReverseB: rev, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
		})
		if err != nil {
			return nil, err
		}
		if check.Complete && check.Equal {
			result = append(result, affineSewBorder{i, [3]float64{p[0].X, p[0].Y, p[0].Z}, [3]float64{p[1].X, p[1].Y, p[1].Z}})
		}
	}
	return result, nil
}

func straightSewCurve(endpoints [2]geom.Vec3) (*geom.BSpline, error) {
	return geom.NewBSpline(1, endpoints[:], []float64{0, 0, 1, 1}, nil)
}

func straightSewPartitions(ctx context.Context, borders []affineSewBorder, edgeCount, edgeLimit, pairLimit int) ([][]*big.Rat, int, error) {
	cuts := make([]map[string]*big.Rat, edgeCount)
	zero, one := new(big.Rat), big.NewRat(1, 1)
	tested, n := 0, edgeCount
	add := func(edge int, t *big.Rat) {
		if t.Sign() <= 0 || t.Cmp(one) >= 0 {
			return
		}
		if cuts[edge] == nil {
			cuts[edge] = make(map[string]*big.Rat)
		}
		key := t.RatString()
		if cuts[edge][key] == nil {
			cuts[edge][key] = t
			n++
		}
	}
	for i, a := range borders {
		for _, b := range borders[i+1:] {
			if err := ctx.Err(); err != nil {
				return nil, 0, err
			}
			if tested == pairLimit {
				return nil, 0, geom.ErrConvergence
			}
			tested++
			overlap := true
			for axis := range a.start {
				lo := max(min(a.start[axis], a.end[axis]), min(b.start[axis], b.end[axis]))
				hi := min(max(a.start[axis], a.end[axis]), max(b.start[axis], b.end[axis]))
				overlap = overlap && lo <= hi
			}
			if !overlap {
				continue
			}
			b0, b1 := a.coordinate(b.start), a.coordinate(b.end)
			if b0 == nil || b1 == nil {
				continue
			}
			low, high := b0, b1
			if low.Cmp(high) > 0 {
				low, high = high, low
			}
			if low.Cmp(one) >= 0 || high.Cmp(zero) <= 0 {
				continue
			}
			add(a.edge, b0)
			add(a.edge, b1)
			length := new(big.Rat).Sub(b1, b0)
			add(b.edge, new(big.Rat).Quo(new(big.Rat).Neg(b0), length))
			add(b.edge, new(big.Rat).Quo(new(big.Rat).Sub(one, b0), length))
			if n > edgeLimit {
				return nil, 0, geom.ErrConvergence
			}
		}
	}
	out := make([][]*big.Rat, edgeCount)
	for i, cut := range cuts {
		for _, t := range cut {
			out[i] = append(out[i], t)
		}
		slices.SortFunc(out[i], (*big.Rat).Cmp)
	}
	return out, tested, nil
}

func (a affineSewBorder) coordinate(p [3]float64) *big.Rat {
	axis := 0
	for a.start[axis] == a.end[axis] {
		axis++
	}
	delta := func(x, y float64) *big.Rat { return new(big.Rat).Sub(ratInset(x), ratInset(y)) }
	t := new(big.Rat).Quo(delta(p[axis], a.start[axis]), delta(a.end[axis], a.start[axis]))
	for i := range a.start {
		d := delta(p[i], a.start[i])
		if d.Cmp(new(big.Rat).Mul(delta(a.end[i], a.start[i]), t)) != 0 {
			return nil
		}
	}
	return t
}

// A sampler gets the ascending partition including zero and one, returning
// exact source points and normalized curves in the original traversal.
type sewBorderSampler func(edge int, parameters []*big.Rat) ([]geom.Vec3, []geom.Curve, error)

func splitSewBorders(ctx context.Context, s Solid, cuts [][]*big.Rat, sample sewBorderSampler) (Solid, [][]ExactSewnSegment, error) {
	var out Solid
	out.vertices, out.faces = s.Vertices(), s.Faces()
	pieces := make([][]ExactSewnSegment, len(s.edges))
	for i, e := range s.edges {
		if err := ctx.Err(); err != nil {
			return Solid{}, nil, err
		}
		zero, one := new(big.Rat), big.NewRat(1, 1)
		if len(cuts[i]) == 0 {
			pieces[i] = []ExactSewnSegment{{ExactSewnEdge{Edge: len(out.edges)}, zero, one}}
			out.edges = append(out.edges, e)
			continue
		}
		parameters := append([]*big.Rat{zero}, cuts[i]...)
		parameters = append(parameters, one)
		points, curves, err := sample(i, parameters)
		if err != nil {
			return Solid{}, nil, err
		}
		previous := e.Start
		for j, curve := range curves {
			next := e.End
			if j+1 < len(curves) {
				next = len(out.vertices)
				out.vertices = append(out.vertices, points[j+1])
			}
			pieces[i] = append(pieces[i], ExactSewnSegment{ExactSewnEdge{Edge: len(out.edges)}, parameters[j], parameters[j+1]})
			out.edges = append(out.edges, parametric.Edge{Start: previous, End: next, Curve: curve, Range: parametric.Interval{Max: 1}})
			previous = next
		}
	}
	for i := range out.faces {
		f := &out.faces[i]
		var err error
		f.Outer, err = rebuildStraightSewLoop(ctx, f.Outer, pieces)
		if err != nil {
			return Solid{}, nil, err
		}
		for j := range f.Holes {
			f.Holes[j], err = rebuildStraightSewLoop(ctx, f.Holes[j], pieces)
			if err != nil {
				return Solid{}, nil, err
			}
		}
	}
	return out, pieces, nil
}

func sampleStraightSewBorder(s Solid, edge int, parameters []*big.Rat) ([]geom.Vec3, []geom.Curve, error) {
	e := s.edges[edge]
	start, end := s.vertices[e.Start], s.vertices[e.End]
	points, curves := make([]geom.Vec3, len(parameters)), make([]geom.Curve, len(parameters)-1)
	points[0], points[len(points)-1] = start, end
	for i := 1; i+1 < len(points); i++ {
		v, err := straightSewMix(parameters[i], [3]float64{start.X, start.Y, start.Z}, [3]float64{end.X, end.Y, end.Z})
		if err != nil {
			return nil, nil, err
		}
		points[i] = geom.Vec3{X: v[0], Y: v[1], Z: v[2]}
	}
	for i := range curves {
		c, err := straightSewCurve([2]geom.Vec3{points[i], points[i+1]})
		if err != nil {
			return nil, nil, err
		}
		curves[i] = c
	}
	return points, curves, nil
}

func straightSewMix(t *big.Rat, a, b [3]float64) (out [3]float64, err error) {
	for i := range out {
		x, exact := exactRectEdgeMix(t, a[i], b[i])
		if !exact {
			return out, geom.ErrUnrepresentable
		}
		out[i] = x
	}
	return out, nil
}

func rebuildStraightSewLoop(ctx context.Context, loop []Use, pieces [][]ExactSewnSegment) ([]Use, error) {
	var out []Use
	one := big.NewRat(1, 1)
	for _, u := range loop {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		segments := pieces[u.Edge]
		if len(segments) == 1 {
			u.Edge = segments[0].Edge
			out = append(out, u)
			continue
		}
		before := u.UVStart
		for i := range segments {
			segment := segments[i]
			hi := segment.Upper
			if u.Reversed {
				segment = segments[len(segments)-1-i]
				hi = new(big.Rat).Sub(one, segment.Lower)
			}
			x, xok := exactRectEdgeMix(hi, u.UVStart.X, u.UVEnd.X)
			y, yok := exactRectEdgeMix(hi, u.UVStart.Y, u.UVEnd.Y)
			point := geom.Vec2{X: x, Y: y}
			if !xok || !yok || before == point {
				return nil, geom.ErrUnrepresentable
			}
			out = append(out, Use{Edge: segment.Edge, Reversed: u.Reversed, UVStart: before, UVEnd: point})
			before = point
		}
	}
	return out, nil
}
