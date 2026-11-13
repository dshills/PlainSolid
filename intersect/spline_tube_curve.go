package intersect

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"slices"
	"sync"

	"plainsolid/geom"
)

// TubeCurveOptions controls exact tube evaluation and floating representative
// verification. ResidualTolerance defaults to 1e-7 in model units.
type TubeCurveOptions struct {
	Patches           geom.SurfacePatchOptions
	Evaluation        geom.SurfaceTubeEvaluationOptions
	Junction          geom.SurfaceTubeJunctionOptions
	ResidualTolerance float64
}

// TubeSegment is one certified local branch expressed as two paired p-curves.
// Exact root enclosures are available through Evaluate; TrimA and TrimB
// return bounded floating representatives of the same contact.
type TubeSegment struct {
	a, b     *geom.BSplineSurface
	pa, pb   *geom.SurfacePatch
	proof    geom.VerifiedSurfaceTube
	options  TubeCurveOptions
	axis     int
	lo, hi   float64
	reversed bool
	ends     [2]geom.SurfaceTubeEvaluation
	samples  *tubeSampleCache
	recipe   TubeRecipe
	index    int
}

// A fixed-size cache shares repeated exact evaluations between the two chart
// uses of one model edge. Tessellation, validation and integration often ask
// for the same normalized parameter. Callers cannot grow memory by sampling.
type tubeSampleCache struct {
	mu    sync.Mutex
	slots [16]tubeSample
}

type tubeSample struct {
	bits uint64
	data geom.SurfaceTubeEvaluation
	ok   bool
}

func tubeSampleIndex(bits uint64) int {
	bits ^= bits >> 33
	bits *= 0xff51afd7ed558ccd
	bits ^= bits >> 33
	return int(bits & 15)
}

func (c *tubeSampleCache) get(t float64) (geom.SurfaceTubeEvaluation, bool) {
	bits := math.Float64bits(t)
	c.mu.Lock()
	x := c.slots[tubeSampleIndex(bits)]
	c.mu.Unlock()
	return x.data, x.ok && x.bits == bits
}

func (c *tubeSampleCache) put(t float64, value geom.SurfaceTubeEvaluation) {
	bits := math.Float64bits(t)
	c.mu.Lock()
	c.slots[tubeSampleIndex(bits)] = tubeSample{bits, value, true}
	c.mu.Unlock()
}

// TubeTrim implements a differentiable p-curve with normalized parameter.
type TubeTrim struct {
	segment TubeSegment
	second  bool
}

func (s TubeSegment) TrimA() TubeTrim { return TubeTrim{segment: s} }
func (s TubeSegment) TrimB() TubeTrim { return TubeTrim{segment: s, second: true} }

// TubeRecipe is an immutable handle to a verified chain's construction inputs.
// It can be used as a key to store one recipe for several p-curves.
type TubeRecipe struct{ data *TubeRecipeData }

// TubeRecipeData contains geometry and four-parameter boxes, not proof
// results. RebuildTubeCurves rechecks branch existence and endpoint identity.
// Oriented, when present, supplies one axis and direction per cell and takes
// precedence over FreeAxis. Closed additionally joins the last to the first.
type TubeRecipeData struct {
	A, B           Patch
	Cells          []TubeRecipeCell
	FreeAxis       int
	Options        TubeCurveOptions
	Oriented       []TubeDirection
	Closed         bool
	ExactEndpoints []TubeExactEndpoint
}

// TubeExactEndpoint selects an open recipe endpoint and gives exact represented
// [uA,vA,uB,vB] parameters, verified during reconstruction.
type TubeExactEndpoint struct {
	Segment, End int
	Parameters   [4]float64
}

// TubeDirection selects the independent parameter and traversal direction.
type TubeDirection struct {
	FreeAxis int
	Reversed bool
}

type TubeRecipeCell struct {
	DomainA, DomainB geom.ParameterRectangle
	BoundaryAnchor   *TubeBoundaryAnchor `json:",omitempty"`
}

func (r TubeRecipe) Data() TubeRecipeData {
	if r.data == nil {
		return TubeRecipeData{}
	}
	x := *r.data
	x.Cells = append([]TubeRecipeCell(nil), x.Cells...)
	for i := range x.Cells {
		x.Cells[i].BoundaryAnchor = cloneTubeBoundaryAnchor(x.Cells[i].BoundaryAnchor)
	}
	x.Oriented = append([]TubeDirection(nil), x.Oriented...)
	x.ExactEndpoints = append([]TubeExactEndpoint(nil), x.ExactEndpoints...)
	return x
}

// SplineBranchCurves rebuilds the traversal returned by JoinSplineTubeBranches
// as paired p-curves with matching endpoints, including across axis changes.
func SplineBranchCurves(ctx context.Context, a, b Patch, seeds SplineSeedReport, branch SplineTubeBranch, o TubeCurveOptions) ([]TubeSegment, error) {
	if ctx == nil || len(branch.Cells) == 0 || len(branch.Cells) > 4096 {
		return nil, geom.ErrInvalid
	}
	data := TubeRecipeData{A: a, B: b, Closed: branch.Closed, Options: o}
	for _, cell := range branch.Cells {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if cell.Cell < 0 || cell.Cell >= len(seeds.Attempts) {
			return nil, geom.ErrDomain
		}
		c := seeds.Attempts[cell.Cell]
		if c.Tube == nil || c.Tube.FreeAxis != cell.FreeAxis || !c.Tube.ProvenBranch {
			return nil, ErrUnresolved
		}
		data.Cells = append(data.Cells, TubeRecipeCell{c.Cell.DomainA, c.Cell.DomainB, cloneTubeBoundaryAnchor(c.BoundaryAnchor)})
		data.Oriented = append(data.Oriented, TubeDirection{cell.FreeAxis, cell.Reversed})
	}
	return RebuildTubeCurves(ctx, data)
}

// Recipe returns the construction recipe, segment index and second-chart flag.
func (p TubeTrim) Recipe() (TubeRecipe, int, bool) {
	return p.segment.recipe, p.segment.index, p.second
}

// CertifiesPairedTube reports whether p and q refer to the same certified
// root segment and their respective original spline supports.
func (p TubeTrim) CertifiesPairedTube(q TubeTrim, a, b *geom.BSplineSurface) bool {
	pa, i, secondA := p.Recipe()
	pb, j, secondB := q.Recipe()
	if pa.data == nil || pb.data == nil || a == nil || b == nil || i != j {
		return false
	}
	x, y := pa.data, pb.data
	if pa != pb && (x.FreeAxis != y.FreeAxis || x.Closed != y.Closed || x.Options != y.Options ||
		x.A.Min != y.A.Min || x.A.Max != y.A.Max || x.B.Min != y.B.Min || x.B.Max != y.B.Max ||
		!slices.Equal(x.Cells, y.Cells) || !slices.Equal(x.Oriented, y.Oriented) || !slices.Equal(x.ExactEndpoints, y.ExactEndpoints) ||
		!geom.SameSplineSurface(p.segment.a, q.segment.a) || !geom.SameSplineSurface(p.segment.b, q.segment.b)) {
		return false
	}
	sa, sb := p.segment.a, q.segment.a
	if secondA {
		sa = p.segment.b
	}
	if secondB {
		sb = q.segment.b
	}
	return geom.SameSplineSurface(a, sa) && geom.SameSplineSurface(b, sb)
}

// BoundsUV encloses the entire p-curve in its certified tube rectangle.
func (p TubeTrim) BoundsUV() (geom.UVBox, error) {
	s := p.segment
	if s.pa == nil || s.pb == nil {
		return geom.UVBox{}, geom.ErrInvalid
	}
	d := s.pa.Domain()
	if p.second {
		d = s.pb.Domain()
	}
	return geom.UVBox{Min: geom.Vec2{X: d.UMin, Y: d.VMin}, Max: geom.Vec2{X: d.UMax, Y: d.VMax}}, nil
}

// EncloseUV bounds the entire exact p-curve over a closed normalized
// subinterval. The boolean reports whether a conservative enclosure is
// available, even when the requested refinement has not converged.
func (p TubeTrim) EncloseUV(ctx context.Context, start, end float64, o geom.SurfaceTubeIntervalOptions) (geom.UVBox, bool, error) {
	s := p.segment
	if ctx == nil || s.pa == nil || !finite(start) || !finite(end) || start < 0 || end > 1 || start >= end {
		return geom.UVBox{}, false, geom.ErrInvalid
	}
	lo, hi := s.lo, s.hi
	a, b := lo, hi
	if s.reversed {
		a, b = b, a
	}
	x, y := exactNormalizedTubeParameter(a, b, start), exactNormalizedTubeParameter(a, b, end)
	if x.Cmp(y) > 0 {
		x, y = y, x
	}
	left, right := outwardSplineParameter(x, true), outwardSplineParameter(y, false)
	q, e := s.proof.EncloseInterval(ctx, max(lo, left), min(hi, right), o)
	if p.second {
		return q.ParametersB, q.HasBounds, e
	}
	return q.ParametersA, q.HasBounds, e
}

func exactNormalizedTubeParameter(a, b, t float64) *big.Rat {
	lo, hi := new(big.Rat).SetFloat64(a), new(big.Rat).SetFloat64(b)
	return hi.Sub(hi, lo).Mul(hi, new(big.Rat).SetFloat64(t)).Add(hi, lo)
}

func outwardSplineParameter(x *big.Rat, lower bool) float64 {
	r, _ := geom.RationalOutwardRange(x)
	if lower {
		return r.Min
	}
	return r.Max
}

// Support returns the original immutable spline of this trim's chart.
func (p TubeTrim) Support() *geom.BSplineSurface {
	if p.second {
		return p.segment.b
	}
	return p.segment.a
}

// TubeCurves builds paired p-curves for an increasing same-axis chain.
// It independently rechecks every junction because the reports are mutable.
// Adjacent endpoint boxes are intersected, so shared representatives agree
// bit-for-bit in both UV charts.
func TubeCurves(ctx context.Context, a, b Patch, seeds SplineSeedReport, chain TubeChain, o TubeCurveOptions) ([]TubeSegment, error) {
	if ctx == nil || len(chain.Cells) == 0 || len(chain.Cells) > 4096 {
		return nil, geom.ErrInvalid
	}
	data := TubeRecipeData{A: a, B: b, FreeAxis: chain.FreeAxis, Options: o}
	for _, i := range chain.Cells {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if i < 0 || i >= len(seeds.Attempts) {
			return nil, geom.ErrDomain
		}
		c := seeds.Attempts[i]
		if c.Tube == nil || c.Tube.FreeAxis != chain.FreeAxis || !c.Tube.ProvenBranch {
			return nil, ErrUnresolved
		}
		data.Cells = append(data.Cells, TubeRecipeCell{c.Cell.DomainA, c.Cell.DomainB, cloneTubeBoundaryAnchor(c.BoundaryAnchor)})
	}
	return RebuildTubeCurves(ctx, data)
}

// RebuildTubeCurves reconstructs paired trims without trusting serialized proof
// claims. Endpoint evaluation revalidates each entire branch tube.
func RebuildTubeCurves(ctx context.Context, data TubeRecipeData) ([]TubeSegment, error) {
	a, b, axis, o := data.A, data.B, data.FreeAxis, data.Options
	if ctx == nil || !a.valid() || !b.valid() || len(data.Cells) == 0 || len(data.Cells) > 4096 || axis < 0 || axis > 3 || len(data.ExactEndpoints) > 2 ||
		len(data.Oriented) != 0 && len(data.Oriented) != len(data.Cells) ||
		data.Closed && (len(data.Cells) < 2 || len(data.Oriented) == 0) {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.ResidualTolerance == 0 {
		o.ResidualTolerance = 1e-7
	}
	if !finite(o.ResidualTolerance) || o.ResidualTolerance <= 0 {
		return nil, geom.ErrInvalid
	}
	sa, ok := a.Surface.(*geom.BSplineSurface)
	if !ok {
		return nil, geom.ErrInvalid
	}
	sb, ok := b.Surface.(*geom.BSplineSurface)
	if !ok {
		return nil, geom.ErrInvalid
	}
	pa, e := geom.NewSurfacePatch(sa, o.Patches)
	if e != nil {
		return nil, e
	}
	pb, e := geom.NewSurfacePatch(sb, o.Patches)
	if e != nil {
		return nil, e
	}
	domain := func(p Patch) geom.ParameterRectangle {
		return geom.ParameterRectangle{UMin: p.Min.X, UMax: p.Max.X, VMin: p.Min.Y, VMax: p.Max.Y}
	}
	pa, e = pa.Trim(domain(a))
	if e != nil {
		return nil, e
	}
	pb, e = pb.Trim(domain(b))
	if e != nil {
		return nil, e
	}
	data.Cells = append([]TubeRecipeCell(nil), data.Cells...)
	for i := range data.Cells {
		data.Cells[i].BoundaryAnchor = cloneTubeBoundaryAnchor(data.Cells[i].BoundaryAnchor)
	}
	data.Oriented = append([]TubeDirection(nil), data.Oriented...)
	data.ExactEndpoints = append([]TubeExactEndpoint(nil), data.ExactEndpoints...)
	data.Options = o
	recipe := TubeRecipe{&data}
	out := make([]TubeSegment, 0, len(data.Cells))
	projection := tubeProjectionLookup(ctx, pa, pb, o.Junction)
	join := func(i, j int) error {
		x, y := &out[i], &out[j]
		p, q := data.Cells[i], data.Cells[j]
		xend, yend := 1, 0
		if x.reversed {
			xend = 0
		}
		if y.reversed {
			yend = 1
		}
		ok := false
		if x.axis == y.axis && xend != yend {
			opt := o.Junction
			opt.FreeAxis = x.axis
			proof, e := geom.CertifySurfaceTubeJunction(pa, pb, p.DomainA, p.DomainB, q.DomainA, q.DomainB, opt)
			if e != nil {
				return e
			}
			ok = proof.Connected
		}
		if !ok && len(data.Oriented) > 0 {
			opt := geom.SurfaceTubeEndpointOptions{LeftFreeAxis: x.axis, RightFreeAxis: y.axis, LeftEnd: xend, RightEnd: yend,
				MaxOperations: o.Junction.MaxOperations, MaxCoefficientBits: o.Junction.MaxCoefficientBits}
			proof, e := certifyTubeEndpointJunction(ctx, pa, pb, p.DomainA, p.DomainB, q.DomainA, q.DomainB, opt, x.ends[1], y.ends[0], x.proof, y.proof, projection)
			if e != nil {
				return e
			}
			ok = proof.Connected
		}
		if !ok {
			return ErrUnresolved
		}
		point, e := canonicalTubePoint(x.ends[1], y.ends[0])
		if e == nil {
			x.ends[1], y.ends[0] = point, point
		}
		return e
	}
	for i, c := range data.Cells {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		ta, e := pa.Trim(c.DomainA)
		if e != nil {
			return nil, e
		}
		tb, e := pb.Trim(c.DomainB)
		if e != nil {
			return nil, e
		}
		d0, d1 := ta.Domain(), tb.Domain()
		d := [4][2]float64{{d0.UMin, d0.UMax}, {d0.VMin, d0.VMax}, {d1.UMin, d1.UMax}, {d1.VMin, d1.VMax}}
		reversed := false
		if len(data.Oriented) != 0 {
			axis, reversed = data.Oriented[i].FreeAxis, data.Oriented[i].Reversed
		}
		if axis < 0 || axis > 3 {
			return nil, geom.ErrInvalid
		}
		lo, hi := d[axis][0], d[axis][1]
		minLimit := func(a, b int) int {
			if a == 0 {
				return b
			}
			if b == 0 {
				return a
			}
			return min(a, b)
		}
		tube, e := verifySplineTubeCell(ctx, ta, tb, geom.SurfacePairTubeOptions{FreeAxis: axis,
			MaxOperations:      minLimit(o.Evaluation.MaxOperations, o.Junction.MaxOperations),
			MaxCoefficientBits: minLimit(o.Evaluation.MaxCoefficientBits, o.Junction.MaxCoefficientBits)}, c.BoundaryAnchor)
		if e != nil {
			return nil, e
		}
		if len(data.Oriented) != 0 && reversed != (tube.FreeTangentSign() < 0) {
			return nil, ErrUnresolved
		}
		seg := TubeSegment{a: sa, b: sb, pa: ta, pb: tb, proof: tube, options: o, axis: axis, lo: lo, hi: hi, reversed: reversed,
			samples: &tubeSampleCache{}, recipe: recipe, index: i}
		if reversed {
			lo, hi = hi, lo
		}
		for j, p := range [2]float64{lo, hi} {
			q, e := seg.evaluate(p)
			if e != nil {
				return nil, e
			}
			seg.ends[j] = q
		}
		out = append(out, seg)
		if i > 0 {
			if e := join(i-1, i); e != nil {
				return nil, e
			}
		}
	}
	if data.Closed {
		if e := join(len(out)-1, 0); e != nil {
			return nil, e
		}
	}
	seen := map[[2]int]bool{}
	for _, h := range data.ExactEndpoints {
		id := [2]int{h.Segment, h.End}
		if !validTubeRecipeEndpoint(data, h.Segment, h.End) || seen[id] {
			return nil, geom.ErrInvalid
		}
		seen[id] = true
		var hint [4]*big.Rat
		for i, f := range h.Parameters {
			if !finite(f) {
				return nil, geom.ErrInvalid
			}
			hint[i] = new(big.Rat).SetFloat64(f)
		}
		s := out[h.Segment]
		q, e := s.CertifyEndpoint(ctx, h.End, hint, geom.TubeEndpointOptions{MaxOperations: o.Junction.MaxOperations, MaxCoefficientBits: o.Junction.MaxCoefficientBits})
		if e != nil || !q.Proven {
			if e == nil {
				e = ErrUnresolved
			}
			return nil, e
		}
		out[h.Segment] = s.withCertifiedEndpoint(h.End, q)
	}
	return out, nil
}

func canonicalTubePoint(a, b geom.SurfaceTubeEvaluation) (geom.SurfaceTubeEvaluation, error) {
	uv := func(a, b geom.UVBox) geom.UVBox {
		return geom.UVBox{Min: geom.Vec2{X: max(a.Min.X, b.Min.X), Y: max(a.Min.Y, b.Min.Y)}, Max: geom.Vec2{X: min(a.Max.X, b.Max.X), Y: min(a.Max.Y, b.Max.Y)}}
	}
	x, y := uv(a.ParametersA, b.ParametersA), uv(a.ParametersB, b.ParametersB)
	box, ok := a.Bounds.Intersect(b.Bounds)
	if !ok || x.Min.X > x.Max.X || x.Min.Y > x.Max.Y || y.Min.X > y.Max.X || y.Min.Y > y.Max.Y {
		return a, ErrUnresolved
	}
	a.ParametersA, a.ParametersB, a.Bounds = x, y, box
	a.Point = box.Min.Scale(.5).Add(box.Max.Scale(.5))
	return a, nil
}

func (s TubeSegment) evaluate(parameter float64) (geom.SurfaceTubeEvaluation, error) {
	o := s.options.Evaluation
	o.FreeAxis, o.Parameter = s.axis, parameter
	return s.proof.Evaluate(o)
}

// Evaluate obtains certified chart and model-space enclosures at normalized t.
func (s TubeSegment) Evaluate(t float64) (geom.SurfaceTubeEvaluation, error) {
	if s.pa == nil || !finite(t) || t < 0 || t > 1 {
		return geom.SurfaceTubeEvaluation{}, geom.ErrDomain
	}
	if t == 0 || t == 1 {
		return s.ends[int(t)], nil
	}
	if s.samples != nil {
		if q, ok := s.samples.get(t); ok {
			return q, nil
		}
	}
	lo, hi := s.lo, s.hi
	if s.reversed {
		lo, hi = hi, lo
	}
	q, e := s.evaluate(min(s.hi, max(s.lo, (1-t)*lo+t*hi)))
	if e == nil && s.samples != nil {
		s.samples.put(t, q)
	}
	return q, e
}

func (s TubeSegment) representative(t float64) (Parameters, error) {
	q, e := s.Evaluate(t)
	if e != nil {
		return Parameters{}, e
	}
	p := Parameters{midUV(q.ParametersA), midUV(q.ParametersB)}
	a, e := s.a.Eval(p.A.X, p.A.Y)
	if e != nil {
		return Parameters{}, e
	}
	b, e := s.b.Eval(p.B.X, p.B.Y)
	if e != nil {
		return Parameters{}, e
	}
	r := s.options.ResidualTolerance
	w := q.Bounds.Size().Length()
	if a.Distance(b) > r || a.Distance(q.Point)+w/2 > r || b.Distance(q.Point)+w/2 > r {
		return Parameters{}, fmt.Errorf("%w: paired tube residual exceeds tolerance", ErrUnresolved)
	}
	return p, nil
}

// EvaluatePair returns verified floating chart representatives for both
// supports at the same normalized tube parameter.
func (s TubeSegment) EvaluatePair(t float64) (Parameters, error) {
	return s.representative(t)
}

func (p TubeTrim) EvalUV(t float64) (geom.Vec2, error) {
	q, e := p.segment.representative(t)
	if p.second {
		return q.B, e
	}
	return q.A, e
}

// EvaluateBounds returns the exact UV root enclosure at normalized t.
func (p TubeTrim) EvaluateBounds(t float64) (geom.UVBox, error) {
	q, e := p.segment.Evaluate(t)
	if p.second {
		return q.ParametersB, e
	}
	return q.ParametersA, e
}

// DerivativeUV evaluates the numerical tangent using the certified nonsingular
// implicit system. Its parameter is the segment's normalized coordinate.
func (p TubeTrim) DerivativeUV(t float64) (geom.Vec2, error) {
	s := p.segment
	q, e := s.representative(t)
	if e != nil {
		return geom.Vec2{}, e
	}
	au, av, e := s.a.Partials(q.A.X, q.A.Y)
	if e != nil {
		return geom.Vec2{}, e
	}
	bu, bv, e := s.b.Partials(q.B.X, q.B.Y)
	if e != nil {
		return geom.Vec2{}, e
	}
	cols := [4]geom.Vec3{au, av, bu.Scale(-1), bv.Scale(-1)}
	scale := 0.
	for _, c := range cols {
		scale = max(scale, c.Length())
	}
	if !finite(scale) || scale == 0 {
		return geom.Vec2{}, ErrSingular
	}
	for j, c := range cols {
		cols[j] = geom.Vec3{X: c.X / scale, Y: c.Y / scale, Z: c.Z / scale}
	}
	idx := [3]int{}
	k := 0
	for i := range 4 {
		if i != s.axis {
			idx[k] = i
			k++
		}
	}
	a, b, c := cols[idx[0]], cols[idx[1]], cols[idx[2]]
	d := a.Dot(b.Cross(c))
	if !finite(d) || math.Abs(d) < 1e-15 {
		return geom.Vec2{}, ErrSingular
	}
	w := s.hi - s.lo
	if s.reversed {
		w = -w
	}
	v := cols[s.axis].Scale(-w)
	r := [4]float64{}
	r[s.axis], r[idx[0]], r[idx[1]], r[idx[2]] = w, v.Dot(b.Cross(c))/d, a.Dot(v.Cross(c))/d, a.Dot(b.Cross(v))/d
	i := 0
	if p.second {
		i = 2
	}
	if !finite(r[i]) || !finite(r[i+1]) {
		return geom.Vec2{}, ErrSingular
	}
	return geom.Vec2{X: r[i], Y: r[i+1]}, nil
}
