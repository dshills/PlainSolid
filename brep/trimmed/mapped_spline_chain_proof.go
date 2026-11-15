package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

// MappedSplineChain is an immutable proof-only, piecewise normalized trace.
// Unlike MultiSpanMappedSplineUV, its geometry need not be one global Bezier.
// It deliberately does not implement PCurve: native topology and face filling
// must not silently replace the original piecewise geometry by one polynomial.
type MappedSplineChain struct {
	parts       []MappedSplineUV
	stations    []*big.Rat
	independent bool
}

// NewMappedSplineChain re-proves two through eight contiguous restrictions of
// one whole original spline, independent positive maps, and exact C0 UV joins.
// Every map constructor retains o's independent work limit (at most eight).
// Model station coefficients are bounded to 4096 bits. No C1 promise is made.
func NewMappedSplineChain(ctx context.Context, parts []MappedSplineUV, stations []*big.Rat, o geom.SurfacePatchOptions) (MappedSplineChain, error) {
	return newMappedSplineChain(ctx, parts, stations, o, false)
}

// NewMappedSplineEdgeChain proves a chain of independently authored whole
// Bézier spline coedges. Their charts and endpoint positions remain exact.
// Each original basis and raw interval remains attached to its own coedge.
func NewMappedSplineEdgeChain(ctx context.Context, parts []MappedSplineUV, stations []*big.Rat, o geom.SurfacePatchOptions) (MappedSplineChain, error) {
	return newMappedSplineChain(ctx, parts, stations, o, true)
}

func newMappedSplineChain(ctx context.Context, parts []MappedSplineUV, stations []*big.Rat, o geom.SurfacePatchOptions, independent bool) (MappedSplineChain, error) {
	var zero MappedSplineChain
	if ctx == nil || len(parts) < 2 || len(parts) > 8 || len(stations) != len(parts)-1 {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	prior := new(big.Rat)
	for _, s := range stations {
		if s == nil {
			return zero, geom.ErrInvalid
		}
		if s.Num().BitLen() > 4096 || s.Denom().BitLen() > 4096 {
			return zero, geom.ErrConvergence
		}
		if s.Cmp(prior) <= 0 || s.Cmp(big.NewRat(1, 1)) >= 0 {
			return zero, geom.ErrInvalid
		}
		prior = s
	}
	out := MappedSplineChain{independent: independent}
	for i, p := range parts {
		if p.source.Curve == nil {
			return zero, geom.ErrInvalid
		}
		for _, k := range p.source.Curve.Knots() {
			if k > p.source.Range.Min && k < p.source.Range.Max {
				return zero, geom.ErrConvergence
			}
		}
		q, e := NewMappedSplineUV(ctx, p.source, p.ParameterWeight(), o)
		if e != nil {
			return zero, e
		}
		if independent {
			lo, hi := q.source.Curve.Domain()
			if q.source.Range != (UVInterval{lo, hi}) {
				return zero, geom.ErrConvergence
			}
		}
		if i > 0 {
			a := out.parts[i-1]
			if a.curve.ends[1] != q.curve.ends[0] {
				return zero, geom.ErrConvergence
			}
			if !independent {
				if !sameAuthoredBezierSpline(a.source.Curve, q.source.Curve) || a.source.Reversed != q.source.Reversed || a.source.ExactEndpoints != q.source.ExactEndpoints {
					return zero, geom.ErrConvergence
				}
				if (!q.source.Reversed && a.source.Range.Max != q.source.Range.Min) || (q.source.Reversed && a.source.Range.Min != q.source.Range.Max) {
					return zero, geom.ErrConvergence
				}
			}
		}
		out.parts = append(out.parts, q)
	}
	if !independent {
		first, last := out.parts[0].source, out.parts[len(parts)-1].source
		lo, hi := first.Curve.Domain()
		if (!first.Reversed && (first.Range.Min != lo || last.Range.Max != hi)) || (first.Reversed && (first.Range.Max != hi || last.Range.Min != lo)) {
			return zero, geom.ErrConvergence
		}
	}
	for _, s := range stations {
		out.stations = append(out.stations, new(big.Rat).Set(s))
	}
	return out, nil
}
func (c MappedSplineChain) Sources() []SplineUV {
	out := make([]SplineUV, len(c.parts))
	for i, p := range c.parts {
		out[i] = p.Source()
	}
	return out
}
func (c MappedSplineChain) ModelStations() []*big.Rat {
	out := make([]*big.Rat, len(c.stations))
	for i, p := range c.stations {
		out[i] = new(big.Rat).Set(p)
	}
	return out
}
func (c MappedSplineChain) locate(t *big.Rat) (int, *big.Rat, error) {
	if len(c.parts) < 2 || t == nil {
		return 0, nil, geom.ErrInvalid
	}
	if t.Num().BitLen() > 4096 || t.Denom().BitLen() > 4096 {
		return 0, nil, geom.ErrConvergence
	}
	if t.Sign() < 0 || t.Cmp(big.NewRat(1, 1)) > 0 {
		return 0, nil, geom.ErrDomain
	}
	lo := new(big.Rat)
	for i := range c.parts {
		hi := big.NewRat(1, 1)
		if i < len(c.stations) {
			hi = c.stations[i]
		}
		if t.Cmp(hi) <= 0 {
			return i, new(big.Rat).Quo(new(big.Rat).Sub(t, lo), new(big.Rat).Sub(hi, lo)), nil
		}
		lo = hi
	}
	return 0, nil, geom.ErrDomain
}

// AuthoredParameter returns the raw parameter on the incoming original coedge
// at a joint. Independently authored adjacent coedges may have distinct raw
// parameters for that same point.
func (c MappedSplineChain) AuthoredParameter(t *big.Rat) (*big.Rat, error) {
	i, u, e := c.locate(t)
	if e != nil {
		return nil, e
	}
	return c.parts[i].AuthoredParameter(u)
}
func (c MappedSplineChain) authoredCellParameter(a, b, t *big.Rat) (int, *big.Rat, error) {
	i, lo, hi, err := c.cellParameters(a, b)
	if err != nil {
		return 0, nil, err
	}
	u := new(big.Rat).Add(lo, new(big.Rat).Mul(t, new(big.Rat).Sub(hi, lo)))
	raw, err := c.parts[i].AuthoredParameter(u)
	return i, raw, err
}

func (c MappedSplineChain) cell(a, b *big.Rat) (ExactBezierUV, error) {
	i, lo, hi, e := c.cellParameters(a, b)
	if e != nil {
		return ExactBezierUV{}, e
	}
	return restrictMappedProofCurve(c.parts[i].curve, lo, hi)
}

func (c MappedSplineChain) cellParameters(a, b *big.Rat) (int, *big.Rat, *big.Rat, error) {
	mid := new(big.Rat).Quo(new(big.Rat).Add(a, b), big.NewRat(2, 1))
	i, _, e := c.locate(mid)
	if e != nil {
		return 0, nil, nil, e
	}
	lo := new(big.Rat)
	if i > 0 {
		lo = c.stations[i-1]
	}
	hi := big.NewRat(1, 1)
	if i < len(c.stations) {
		hi = c.stations[i]
	}
	if a.Cmp(lo) < 0 || b.Cmp(hi) > 0 {
		return 0, nil, nil, geom.ErrConvergence
	}
	w := new(big.Rat).Sub(hi, lo)
	return i, new(big.Rat).Quo(new(big.Rat).Sub(a, lo), w), new(big.Rat).Quo(new(big.Rat).Sub(b, lo), w), nil
}

// MappedSplineChainAgreement proves continuous complete model correspondence;
// it is not an embedding, closed shell, native persistence or face-fill proof.
// Source ownership is the ordered pair of immutable input chains/supports.
type MappedSplineChainAgreement struct {
	cuts         []*big.Rat
	points       [][3]*big.Rat
	sources      [2]MappedSplineChain
	supports     [2]geom.Surface
	knotRepair   mappedChainKnotRepair
	knotCutCount int
	jointChanges [][2]bool
}

type mappedChainKnotRepair uint8

// Nonzero identifiers map to native boundary recipe versions 3 through 12.
const (
	mappedChainNoKnotRepair mappedChainKnotRepair = iota
	mappedChainSingleKnotRepair
	mappedChainSynchronizedKnotsRepair
	mappedChainAsynchronousKnotsRepair
	mappedChainMixedKnotsRepair
	mappedChainThreeKnotsRepair
	mappedChainMultipleKnotsRepair
	mappedChainCellLocalKnotsRepair
	mappedChainSameCellAsynchronousKnotsRepair
	mappedChainJoinedKnotsRepair
	mappedChainAsymmetricJoinedKnotsRepair
)

// OriginalJointSupportChanges reports the proved original joint masks in
// traversal order. Each bit reports whether that source roof changes its
// support cell there. The joined support proofs populate this record.
func (r MappedSplineChainAgreement) OriginalJointSupportChanges() [][2]bool {
	return slices.Clone(r.jointChanges)
}

func (r MappedSplineChainAgreement) ModelCuts() []*big.Rat {
	out := make([]*big.Rat, len(r.cuts))
	for i, p := range r.cuts {
		out[i] = new(big.Rat).Set(p)
	}
	return out
}
func (r MappedSplineChainAgreement) ModelPoints() [][3]*big.Rat {
	out := make([][3]*big.Rat, len(r.points))
	for i, p := range r.points {
		for k := range p {
			out[i][k] = new(big.Rat).Set(p[k])
		}
	}
	return out
}
func (r MappedSplineChainAgreement) AuthoredParameters(owner int) ([]*big.Rat, error) {
	if owner < 0 || owner > 1 || len(r.cuts) == 0 {
		return nil, geom.ErrInvalid
	}
	out := make([]*big.Rat, len(r.cuts))
	for i, t := range r.cuts {
		p, e := r.sources[owner].AuthoredParameter(t)
		if e != nil {
			return nil, e
		}
		out[i] = p
	}
	return out, nil
}

// CertifyMappedSplineChainAgreement compares every cell of the common exact
// station union (at most 15 cells), using full homogeneous composition equality.
// Each cell retains o's proof budget; total work is bounded by 15 such proofs
// plus at most 16 exact endpoint evaluations. Internal restriction endpoints
// must be representable under the existing exact Bezier descriptor contract.
// Both supports must be directly supported exact spline charts (no placement).
func CertifyMappedSplineChainAgreement(ctx context.Context, a geom.Surface, ca MappedSplineChain, b geom.Surface, cb MappedSplineChain, o edgecheck.Options) (MappedSplineChainAgreement, error) {
	return certifyMappedSplineChainAgreement(ctx, a, ca, b, cb, o, mappedChainNoKnotRepair)
}

// CertifyMappedSplineChainSupportKnotAgreement adds one shared model station
// at an isolated transverse crossing inside the original chain station union.
// Each roof may have at most one interior support knot across its two axes.
// Bounded rational reconstruction must recover the exact crossing. Both
// raw owner restrictions and the model point must fit the native source
// contracts. Both restrictions must fit
// wholly in original support knot cells. The resulting partition has at most
// 15 cells and retains the original curves and charts.
func CertifyMappedSplineChainSupportKnotAgreement(ctx context.Context, a geom.Surface, ca MappedSplineChain, b geom.Surface, cb MappedSplineChain, o edgecheck.Options) (MappedSplineChainAgreement, error) {
	return certifyMappedSplineChainAgreement(ctx, a, ca, b, cb, o, mappedChainSingleKnotRepair)
}

// CertifyMappedSplineChainTwoSupportKnotsAgreement proves two synchronized
// transverse model crossings inside the same original common chain cell.
// Each roof must contribute both events on its own original support knots.
// The complete original curve restrictions and exact native representations
// are proved for the three resulting cells. The common partition is bounded
// to 15 cells including the two new cuts.
func CertifyMappedSplineChainTwoSupportKnotsAgreement(ctx context.Context, a geom.Surface, ca MappedSplineChain, b geom.Surface, cb MappedSplineChain, o edgecheck.Options) (MappedSplineChainAgreement, error) {
	return certifyMappedSplineChainAgreement(ctx, a, ca, b, cb, o, mappedChainSynchronizedKnotsRepair)
}

// CertifyMappedSplineChainAsynchronousSupportKnotsAgreement adds one shared
// model cut inside each of two original common chain cells. Each original
// roof has one transverse support knot crossing in a different cell. The
// other roof remains in its original span at that cut. Both exact raw curves
// and their model lifts are preserved on every resulting restriction.
func CertifyMappedSplineChainAsynchronousSupportKnotsAgreement(ctx context.Context, a geom.Surface, ca MappedSplineChain, b geom.Surface, cb MappedSplineChain, o edgecheck.Options) (MappedSplineChainAgreement, error) {
	return certifyMappedSplineChainAgreement(ctx, a, ca, b, cb, o, mappedChainAsynchronousKnotsRepair)
}

// CertifyMappedSplineChainSameCellAsynchronousSupportKnotsAgreement proves
// two distinct owner-only crossings in one original common chain cell. Each
// roof changes its own support once and the opposite roof remains in its
// current support cell across that station.
func CertifyMappedSplineChainSameCellAsynchronousSupportKnotsAgreement(ctx context.Context, a geom.Surface, ca MappedSplineChain, b geom.Surface, cb MappedSplineChain, o edgecheck.Options) (MappedSplineChainAgreement, error) {
	return certifyMappedSplineChainAgreement(ctx, a, ca, b, cb, o, mappedChainSameCellAsynchronousKnotsRepair)
}

// CertifyMappedSplineChainMixedSupportKnotsAgreement proves two distinct exact
// support crossings in one original common chain cell. At one crossing both
// owners change their original support cells. At the other only the owner
// with two original support knots changes cells. Both raw source restrictions
// and their complete model equality are certified in every resulting cell.
func CertifyMappedSplineChainMixedSupportKnotsAgreement(ctx context.Context, a geom.Surface, ca MappedSplineChain, b geom.Surface, cb MappedSplineChain, o edgecheck.Options) (MappedSplineChainAgreement, error) {
	return certifyMappedSplineChainAgreement(ctx, a, ca, b, cb, o, mappedChainMixedKnotsRepair)
}

// CertifyMappedSplineChainThreeSupportKnotsAgreement proves three exact
// transverse stations inside one original common chain cell. Each owner
// crosses two original support knots. One station is shared and each
// owner changes cells alone at one of the other stations. All four
// resulting restrictions retain both original raw parameter maps.
func CertifyMappedSplineChainThreeSupportKnotsAgreement(ctx context.Context, a geom.Surface, ca MappedSplineChain, b geom.Surface, cb MappedSplineChain, o edgecheck.Options) (MappedSplineChainAgreement, error) {
	return certifyMappedSplineChainAgreement(ctx, a, ca, b, cb, o, mappedChainThreeKnotsRepair)
}

// CertifyMappedSplineChainFourSupportKnotsAgreement proves four exact stations
// inside one original common cell. One station is shared and the two roofs
// contribute one and two separate crossings of their original support knots.
// Each of the five restrictions retains both original raw parameter maps.
func CertifyMappedSplineChainFourSupportKnotsAgreement(ctx context.Context, a geom.Surface, ca MappedSplineChain, b geom.Surface, cb MappedSplineChain, o edgecheck.Options) (MappedSplineChainAgreement, error) {
	proof, err := CertifyMappedSplineChainMultipleSupportKnotsAgreement(ctx, a, ca, b, cb, o)
	if err == nil && proof.knotCutCount != 4 {
		return MappedSplineChainAgreement{}, geom.ErrConvergence
	}
	return proof, err
}

// CertifyMappedSplineChainMultipleSupportKnotsAgreement proves four through
// six distinct exact crossings inside one original common cell. Each owner
// has two through four original support knots and contributes at least one
// owner-only crossing. Exactly one station is shared. The count is rederived
// from the original support charts during native v8 reload.
func CertifyMappedSplineChainMultipleSupportKnotsAgreement(ctx context.Context, a geom.Surface, ca MappedSplineChain, b geom.Surface, cb MappedSplineChain, o edgecheck.Options) (MappedSplineChainAgreement, error) {
	return certifyMappedSplineChainAgreement(ctx, a, ca, b, cb, o, mappedChainMultipleKnotsRepair)
}

// CertifyMappedSplineChainCellLocalSupportKnotsAgreement proves two original
// common cells with two transverse crossings each. Each cell has one shared
// support event and one event from a single owner. The single-owner events
// belong to opposite roofs. A third original common cell can contribute one
// more shared crossing. The separated arrangement has one shared-plus-single
// cell and an opposite single-owner event beyond an event-free original cell.
// Every authored roof knot contributes exactly once.
func CertifyMappedSplineChainCellLocalSupportKnotsAgreement(ctx context.Context, a geom.Surface, ca MappedSplineChain, b geom.Surface, cb MappedSplineChain, o edgecheck.Options) (MappedSplineChainAgreement, error) {
	return certifyMappedSplineChainAgreement(ctx, a, ca, b, cb, o, mappedChainCellLocalKnotsRepair)
}

// CertifyMappedSplineChainJoinedSupportKnotsAgreement proves two or three
// adjacent original coedges on each roof. Both roofs must cross a support
// line transversely at each original joint. Exactly one roof crosses support
// inside each coedge, and both roofs must contribute interior events. Only
// interior events add model cuts.
func CertifyMappedSplineChainJoinedSupportKnotsAgreement(ctx context.Context, a geom.Surface, ca MappedSplineChain, b geom.Surface, cb MappedSplineChain, o edgecheck.Options) (MappedSplineChainAgreement, error) {
	return certifyMappedSplineChainAgreement(ctx, a, ca, b, cb, o, mappedChainJoinedKnotsRepair)
}

// CertifyMappedSplineChainAsymmetricJoinedSupportKnotsAgreement proves
// three original coedges with one shared and one unilateral support joint.
// The original roofs have two and four support knots, respectively. Exactly
// one roof changes support inside each coedge, and both contribute interior
// events. The other roof's original span extends across the unilateral joint.
func CertifyMappedSplineChainAsymmetricJoinedSupportKnotsAgreement(ctx context.Context, a geom.Surface, ca MappedSplineChain, b geom.Surface, cb MappedSplineChain, o edgecheck.Options) (MappedSplineChainAgreement, error) {
	return certifyMappedSplineChainAgreement(ctx, a, ca, b, cb, o, mappedChainAsymmetricJoinedKnotsRepair)
}

func certifyMappedSplineChainAgreement(ctx context.Context, a geom.Surface, ca MappedSplineChain, b geom.Surface, cb MappedSplineChain, o edgecheck.Options, repair mappedChainKnotRepair) (MappedSplineChainAgreement, error) {
	var zero MappedSplineChainAgreement
	if ctx == nil || len(ca.parts) < 2 || len(cb.parts) < 2 {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	cuts := []*big.Rat{new(big.Rat), big.NewRat(1, 1)}
	for _, c := range []MappedSplineChain{ca, cb} {
		for _, s := range c.stations {
			for i, t := range cuts {
				if s.Cmp(t) == 0 {
					break
				}
				if s.Cmp(t) < 0 {
					cuts = append(cuts, nil)
					copy(cuts[i+1:], cuts[i:])
					cuts[i] = new(big.Rat).Set(s)
					break
				}
			}
		}
	}
	if o.MaxIntervals > 0 && len(cuts)-1 > o.MaxIntervals {
		return zero, geom.ErrConvergence
	}
	limits := geom.SurfacePatchOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits}
	pa, e := exactSplineSupportPatch(a, limits)
	if e != nil {
		return zero, e
	}
	pb, e := exactSplineSupportPatch(b, limits)
	if e != nil {
		return zero, e
	}
	out := MappedSplineChainAgreement{sources: [2]MappedSplineChain{ca, cb}, supports: [2]geom.Surface{a, b}, knotRepair: repair}
	var repairedCuts []bool
	if repair != mappedChainNoKnotRepair {
		originalCount := len(cuts)
		patches := [2]*geom.SurfacePatch{pa, pb}
		switch repair {
		case mappedChainJoinedKnotsRepair, mappedChainAsymmetricJoinedKnotsRepair:
			partition, err := mappedSplineJoinedSupportKnotCuts(ctx, out.sources, patches, cuts, repair, o)
			if err != nil {
				return zero, err
			}
			cuts, repairedCuts, out.jointChanges = partition.cuts, partition.inserted, partition.jointChanges
		default:
			cuts, repairedCuts, e = mappedSplineSupportKnotCuts(ctx, out.sources, patches, cuts, repair, o)
			if e != nil {
				return zero, e
			}
		}
		if len(cuts) > 16 || o.MaxIntervals > 0 && len(cuts)-1 > o.MaxIntervals {
			return zero, geom.ErrConvergence
		}
		out.knotCutCount = len(cuts) - originalCount
	}
	out.cuts = cuts
	for i := 0; i+1 < len(cuts); i++ {
		qa, e := ca.cell(cuts[i], cuts[i+1])
		if e != nil {
			return zero, e
		}
		qb, e := cb.cell(cuts[i], cuts[i+1])
		if e != nil {
			return zero, e
		}
		da, e := qa.descriptor(ctx, limits)
		if e != nil {
			return zero, e
		}
		db, e := qb.descriptor(ctx, limits)
		if e != nil {
			return zero, e
		}
		if repair != mappedChainNoKnotRepair {
			for owner, d := range [2]*geom.ExactSplineCurve{da, db} {
				patch := [2]*geom.SurfacePatch{pa, pb}[owner]
				if _, err := trimModelSupportCell(ctx, patch, d, limits); err != nil {
					return zero, fmt.Errorf("roof %d proved cell %d: %w", owner, i, err)
				}
				if repairedCuts[i] || repairedCuts[i+1] {
					c := out.sources[owner]
					part, lo, hi, err := c.cellParameters(cuts[i], cuts[i+1])
					if err != nil {
						return zero, err
					}
					if _, err := restrictMappedSource(ctx, c.parts[part], lo, hi, limits); err != nil {
						return zero, fmt.Errorf("roof %d proved cell %d original restriction: %w", owner, i, err)
					}
				}
			}
		}
		equal, e := compareTrimCompositionsOnPlacedSplineSupports(ctx, a, da, b, db, o)
		if e != nil {
			return zero, e
		}
		if !equal {
			return zero, geom.ErrConvergence
		}
		for endpoint := 0; endpoint < 2; endpoint++ {
			if i > 0 && endpoint == 0 {
				continue
			}
			ua, ub := qa.ends[endpoint], qb.ends[endpoint]
			xa, e := pa.EvaluateRational(ctx, ratInset(ua.X), ratInset(ua.Y), geom.SurfacePatchEvaluationOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits})
			if e != nil {
				return zero, e
			}
			xb, e := pb.EvaluateRational(ctx, ratInset(ub.X), ratInset(ub.Y), geom.SurfacePatchEvaluationOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits})
			if e != nil {
				return zero, e
			}
			var point [3]*big.Rat
			for k := range point {
				if xa.Point[k].Cmp(xb.Point[k]) != 0 {
					return zero, geom.ErrConvergence
				}
				point[k] = new(big.Rat).Set(xa.Point[k])
			}
			if repair != mappedChainNoKnotRepair && repairedCuts[i+1] && endpoint == 1 {
				if _, e := geom.RepresentedExactPoint(point); e != nil {
					return zero, e
				}
			}
			out.points = append(out.points, point)
		}
	}
	return out, nil
}
