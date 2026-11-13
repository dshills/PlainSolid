package trimvalidate

import (
	"context"
	"errors"
	"math/big"
)

// CertifiedLoops binds immutable exact trim topology to repeated location
// queries. Construction proves the loops once; each query has its own work
// and subdivision counters with the same configured limits. It is safe for
// concurrent read-only queries. The zero value is invalid.
type CertifiedLoops struct {
	rings   [][]curve
	edges   []curve
	options Options
	forest  bool
}

// CertifyLoops proves positive outer winding, simple disjoint rings and
// strictly contained, nonnested holes. A failed or unresolved proof returns
// nil and the same diagnostic as ValidateLoops.
func CertifyLoops(ctx context.Context, outer Loop, holes []Loop, o Options) (*CertifiedLoops, Report, error) {
	var loops *CertifiedLoops
	r, _, e := validateLoops(ctx, outer, holes, nil, o, &loops, nil)
	if e != nil || r.Status != Certified {
		return nil, r, e
	}
	return loops, r, nil
}

// Locate classifies a rational UV point against the certified rings.
func (c *CertifiedLoops) Locate(ctx context.Context, uv [2]*big.Rat) (Location, Report, error) {
	if uv[0] == nil || uv[1] == nil {
		return Unknown, emptyLoopReport(), ErrOptions
	}
	p := point(uv)
	return c.locate(ctx, &p)
}

// LocateBox proves a single material location over the entire closed box.
// An enclosure that may meet a trim boundary returns Unknown.
func (c *CertifiedLoops) LocateBox(ctx context.Context, lo, hi [2]*big.Rat) (Location, Report, error) {
	for i := range lo {
		if lo[i] == nil || hi[i] == nil || lo[i].Cmp(hi[i]) > 0 {
			return Unknown, emptyLoopReport(), ErrOptions
		}
	}
	return c.locate(ctx, nil, rationalQueryBox{lo: lo, hi: hi})
}

func emptyLoopReport() Report { return Report{LoopA: -1, EdgeA: -1, LoopB: -1, EdgeB: -1} }

func (c *CertifiedLoops) locate(ctx context.Context, p *point, boxes ...rationalQueryBox) (Location, Report, error) {
	r := emptyLoopReport()
	if ctx == nil || c == nil || len(c.rings) == 0 && !c.forest {
		return Unknown, r, ErrOptions
	}
	if e := ctx.Err(); e != nil {
		return Unknown, r, e
	}
	b := &budget{ctx: ctx, o: c.options}
	loc, status, reason := locateCertifiedLoops(c.rings, c.edges, p, &r, b, c.forest, boxes...)
	r.Status, r.Reason, r.Work, r.Subdivisions = status, reason, b.work, b.subdivisions
	if b.err != nil {
		r.Status = Unresolved
		if !errors.Is(b.err, ErrLimit) {
			return Unknown, r, b.err
		}
		return Unknown, r, nil
	}
	return loc, r, nil
}
