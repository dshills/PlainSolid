package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ExactSplineSewOptions bounds both endpoint preimage discovery and complete
// curve equality checks. Pair tests default to 32768 and aggregate polynomial
// work to 4000000 operations. Point limits also apply to each preimage search.
type ExactSplineSewOptions struct {
	Sew                            ExactSewOptions
	Point                          geom.ExactCurveSegmentOptions
	MaxPairTests, MaxDiscoveryWork int
	// MultiSpan admits original model borders with interior knots when the
	// common interval can be proved on the original parametrizations.
	MultiSpan bool
}

// ExactSplineSewReport includes the polynomial work performed to discover
// exact overlap intervals, in addition to the original topology mapping.
type ExactSplineSewReport struct {
	ExactSubdivisionSewReport
	DiscoveryWork int
}

// SewExactSplineSubdivisions joins differently split single-Bezier model edges
// with affine UV coedges on exact spline supports. It searches for unique
// rational endpoint preimages, then compares the complete interval between
// two shared endpoint pairs using exact normalized rational curves. A shared
// point or matching endpoints alone cannot authorize a split. The native
// result retains the original supports and exact normalized source intervals.
// MultiSpan allows comparisons across different original knot partitions.
// The new UV and model endpoints must be exactly representable.
func SewExactSplineSubdivisions(ctx context.Context, vertices []geom.Vec3, edges []parametric.Edge, faces []Face, o ExactSplineSewOptions) (Solid, ExactSplineSewReport, error) {
	var empty ExactSplineSewReport
	if o.MaxPairTests == 0 {
		o.MaxPairTests = 32768
	}
	if o.MaxDiscoveryWork == 0 {
		o.MaxDiscoveryWork = 4000000
	}
	if o.MaxPairTests < 1 || o.MaxPairTests > 262144 || o.MaxDiscoveryWork < 1 || o.MaxDiscoveryWork > 128000000 || o.Point.MaxOperations < 0 {
		return Solid{}, empty, geom.ErrInvalid
	}
	source := Solid{vertices: vertices, edges: edges, faces: faces}
	settings, err := exactSewSource(ctx, source, o.Sew)
	if err != nil {
		return Solid{}, empty, err
	}
	owners, borders, err := splineSewBorders(ctx, source, settings.Curve, o.MultiSpan)
	if err != nil {
		return Solid{}, empty, err
	}
	discovery := splineSewDiscovery{ctx: ctx, options: o, curveOptions: settings.Curve}
	cuts, err := discovery.partitions(borders, len(edges), settings.MaxEdges)
	if err != nil {
		return Solid{}, empty, err
	}
	solid, report, err := sewSubdividedBorders(ctx, source, settings, cuts, discovery.pairs, func(edge int, parameters []*big.Rat) ([]geom.Vec3, []geom.Curve, error) {
		return sampleSplineSewBorder(ctx, source, edge, owners[edge], parameters, settings.Curve)
	})
	if err != nil {
		return Solid{}, empty, err
	}
	return solid, ExactSplineSewReport{report, discovery.work}, nil
}

type splineSewOwner struct {
	face  int
	use   Use
	count int
}

type splineSewBorder struct {
	edge      int
	curve     *geom.ExactSplineCurve
	reversed  bool
	ends      [2]geom.Vec3
	bounds    [3][2]*big.Rat
	preimage  map[geom.Vec3]splineSewPreimage
	multispan bool
}

type splineSewPreimage struct {
	t          *big.Rat
	unresolved bool
}

func splineSewBorders(ctx context.Context, s Solid, o geom.SurfacePatchOptions, multispan bool) ([]splineSewOwner, []splineSewBorder, error) {
	owners := make([]splineSewOwner, len(s.edges))
	for fi, f := range s.faces {
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				owners[u.Edge] = splineSewOwner{fi, u, owners[u.Edge].count + 1}
			}
		}
	}
	var borders []splineSewBorder
	for i, e := range s.edges {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		over := owners[i]
		base, _ := parametric.SurfaceMapping(s.faces[over.face].Surface)
		if over.count != 1 || over.use.Curve != nil || !supportsExactSplinePatch(base) {
			continue
		}
		ends := [2]geom.Vec3{s.vertices[e.Start], s.vertices[e.End]}
		curve, rev, err := exactSewCurve(ctx, e, ends, o)
		if err != nil {
			return nil, nil, fmt.Errorf("trimmed: spline border %d: %w", i, err)
		}
		net, _, err := curve.BezierControlNet(ctx, o)
		b := splineSewBorder{edge: i, curve: curve, reversed: rev, ends: ends, preimage: make(map[geom.Vec3]splineSewPreimage)}
		if errors.Is(err, geom.ErrConvergence) {
			if multispan {
				box, e := curve.ProjectionBounds()
				if e != nil {
					return nil, nil, e
				}
				lower, upper := [3]float64{box.Min.X, box.Min.Y, box.Min.Z}, [3]float64{box.Max.X, box.Max.Y, box.Max.Z}
				for axis := range b.bounds {
					b.bounds[axis] = [2]*big.Rat{ratInset(lower[axis]), ratInset(upper[axis])}
				}
				b.multispan = true
				borders = append(borders, b)
			}
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		for _, h := range net.Control {
			for axis := range b.bounds {
				x := new(big.Rat).Quo(h[axis], h[3])
				box := &b.bounds[axis]
				if box[0] == nil || x.Cmp(box[0]) < 0 {
					box[0] = x
				}
				if box[1] == nil || x.Cmp(box[1]) > 0 {
					box[1] = x
				}
			}
		}
		borders = append(borders, b)
	}
	return owners, borders, nil
}

type splineSewDiscovery struct {
	ctx          context.Context
	options      ExactSplineSewOptions
	curveOptions geom.SurfacePatchOptions
	pairs, work  int
}

func (d *splineSewDiscovery) partitions(borders []splineSewBorder, edgeCount, edgeLimit int) ([][]*big.Rat, error) {
	cuts := make([][]*big.Rat, edgeCount)
	one := big.NewRat(1, 1)
	for i := range borders {
		a := &borders[i]
		for j := i + 1; j < len(borders); j++ {
			if err := d.ctx.Err(); err != nil {
				return nil, err
			}
			if d.pairs == d.options.MaxPairTests {
				return nil, geom.ErrConvergence
			}
			d.pairs++
			b := &borders[j]
			if a.ends == b.ends || a.ends == ([2]geom.Vec3{b.ends[1], b.ends[0]}) {
				// Fully matching borders are handled by ordinary exact sewing.
				continue
			}
			interval, err := d.overlap(a, b)
			if err != nil {
				return nil, err
			}
			for owner, bounds := range interval {
				edge := [2]int{a.edge, b.edge}[owner]
				for _, t := range bounds {
					if t == nil || t.Sign() <= 0 || t.Cmp(one) >= 0 || slices.ContainsFunc(cuts[edge], func(q *big.Rat) bool { return q.Cmp(t) == 0 }) {
						continue
					}
					if edgeCount == edgeLimit {
						return nil, geom.ErrConvergence
					}
					cuts[edge] = append(cuts[edge], t)
					edgeCount++
				}
			}
		}
	}
	for _, c := range cuts {
		slices.SortFunc(c, (*big.Rat).Cmp)
	}
	return cuts, nil
}

func (d *splineSewDiscovery) overlap(a, b *splineSewBorder) ([2][2]*big.Rat, error) {
	var empty [2][2]*big.Rat
	var anchors [][2]*big.Rat
	for side, own := range [2]*splineSewBorder{a, b} {
		other := [2]*splineSewBorder{b, a}[side]
		for end, p := range own.ends {
			position, err := d.atPoint(other, p)
			if err != nil {
				return empty, err
			}
			if position.unresolved {
				return empty, nil
			}
			if position.t == nil {
				continue
			}
			var key [2]*big.Rat
			key[side], key[1-side] = big.NewRat(int64(end), 1), position.t
			if !slices.ContainsFunc(anchors, func(x [2]*big.Rat) bool { return x[0].Cmp(key[0]) == 0 && x[1].Cmp(key[1]) == 0 }) {
				anchors = append(anchors, key)
			}
		}
	}
	if len(anchors) != 2 || anchors[0][0].Cmp(anchors[1][0]) == 0 || anchors[0][1].Cmp(anchors[1][1]) == 0 {
		return empty, nil
	}
	var parts [2]*geom.ExactSplineCurve
	var pieces [2]geom.SurfaceAffineCurvePieces
	multi := a.multispan || b.multispan
	for side, c := range [2]*splineSewBorder{a, b} {
		left, right := c.absolute(anchors[0][side]), c.absolute(anchors[1][side])
		limit := d.remaining(d.curveOptions.MaxOperations)
		if limit == 0 {
			return empty, geom.ErrConvergence
		}
		o := geom.TubeAffineLineOptions{MaxOperations: limit, MaxCoefficientBits: d.curveOptions.MaxCoefficientBits, MaxPoles: d.curveOptions.MaxPoles}
		if multi {
			part, err := c.curve.RationalSegmentPieces(d.ctx, left, right, geom.SurfaceAffineCurveOptions{TubeAffineLineOptions: o})
			if err != nil {
				return empty, err
			}
			pieces[side] = part
			d.work += part.ExactOperations
		} else {
			part, err := c.curve.RationalSegment(d.ctx, left, right, o)
			if err != nil {
				return empty, err
			}
			parts[side] = part
			d.work += part.ExactOperations()
		}
	}
	limit := d.remaining(d.curveOptions.MaxOperations)
	if limit == 0 {
		return empty, geom.ErrConvergence
	}
	o := geom.ExactCurveComparisonOptions{NormalizeDomains: true, MaxOperations: limit, MaxCoefficientBits: d.curveOptions.MaxCoefficientBits}
	var check geom.ExactCurveComparison
	var err error
	if multi {
		check, err = geom.CompareAffineParameterCurvePieces(d.ctx, pieces[0], pieces[1], o)
	} else {
		check, err = geom.CompareExactCurvesContext(d.ctx, parts[0], parts[1], o)
	}
	if err != nil {
		return empty, err
	}
	d.work += check.ExactOperations
	if !check.Complete || !check.Equal {
		return empty, nil
	}
	return [2][2]*big.Rat{{anchors[0][0], anchors[1][0]}, {anchors[0][1], anchors[1][1]}}, nil
}

func (d *splineSewDiscovery) remaining(max int) int {
	left := d.options.MaxDiscoveryWork - d.work
	if max > 0 && max < left {
		left = max
	}
	return left
}

func (d *splineSewDiscovery) atPoint(b *splineSewBorder, p geom.Vec3) (splineSewPreimage, error) {
	var empty splineSewPreimage
	if saved, ok := b.preimage[p]; ok {
		return saved, nil
	}
	q := [3]*big.Rat{ratInset(p.X), ratInset(p.Y), ratInset(p.Z)}
	for axis, x := range q {
		box := b.bounds[axis]
		if x.Cmp(box[0]) < 0 || x.Cmp(box[1]) > 0 {
			b.preimage[p] = empty
			return empty, nil
		}
	}
	o := d.options.Point
	o.MaxOperations = d.remaining(o.MaxOperations)
	if o.MaxOperations == 0 {
		return empty, geom.ErrConvergence
	}
	proof, err := geom.FindExactCurvePointParameters(d.ctx, b.curve, q, o)
	if err != nil {
		return empty, err
	}
	d.work += proof.Operations
	result := splineSewPreimage{}
	switch {
	case !proof.Complete || len(proof.ConstantSpans) > 0:
		result.unresolved = true
	case len(proof.Roots) > 0:
		if root := proof.Roots[0].Parameter; root != nil {
			for _, other := range proof.Roots[1:] {
				// A shared knot is reported by both incident spans.
				// Distinct exact roots still make the preimage ambiguous.
				if other.Parameter == nil || root.Cmp(other.Parameter) != 0 {
					result.unresolved = true
					break
				}
			}
			if !result.unresolved {
				origin, end := b.domain()
				result.t = new(big.Rat).Quo(new(big.Rat).Sub(root, origin), new(big.Rat).Sub(end, origin))
			}
		} else {
			result.unresolved = true
		}
	}
	b.preimage[p] = result
	return result, nil
}

func (b *splineSewBorder) domain() (*big.Rat, *big.Rat) {
	lo, hi := b.curve.Domain()
	if b.reversed {
		lo, hi = hi, lo
	}
	return ratInset(lo), ratInset(hi)
}

func (b *splineSewBorder) absolute(t *big.Rat) *big.Rat {
	lo, hi := b.domain()
	width := new(big.Rat).Sub(hi, lo)
	return width.Add(lo, width.Mul(width, t))
}

func sampleSplineSewBorder(ctx context.Context, s Solid, edge int, owner splineSewOwner, parameters []*big.Rat, o geom.SurfacePatchOptions) ([]geom.Vec3, []geom.Curve, error) {
	f := s.faces[owner.face]
	patch, err := exactFaceSupportPatch(f, o)
	if err != nil {
		return nil, nil, err
	}
	points, curves := make([]geom.Vec3, len(parameters)), make([]geom.Curve, len(parameters)-1)
	var prior geom.Vec2
	start, end := owner.use.UVStart, owner.use.UVEnd
	if owner.use.Reversed {
		start, end = end, start
	}
	for i, t := range parameters {
		x, xok := exactRectEdgeMix(t, start.X, end.X)
		y, yok := exactRectEdgeMix(t, start.Y, end.Y)
		if !xok || !yok {
			return nil, nil, geom.ErrUnrepresentable
		}
		uv := geom.Vec2{X: x, Y: y}
		if i == 0 || i == len(parameters)-1 {
			original := s.edges[edge].Start
			if i != 0 {
				original = s.edges[edge].End
			}
			points[i] = s.vertices[original]
		} else {
			p, err := patch.EvaluateRational(ctx, ratInset(x), ratInset(y), geom.SurfacePatchEvaluationOptions{
				MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
			})
			if err != nil {
				return nil, nil, err
			}
			points[i], err = geom.RepresentedExactPoint(p.Point)
			if err != nil {
				return nil, nil, err
			}
		}
		if i > 0 {
			curves[i-1] = surfaceTrimCurve{surface: f.Surface, a: prior, b: uv}
		}
		prior = uv
	}
	return points, curves, nil
}
