package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/geom"
)

// CombinedOccurrenceColoredAssembly authenticates disjoint shared-use and
// path-specific whole-body overrides on the selected repeated-group grammar.
// It deliberately rejects overlapping targets instead of inferring precedence.
type CombinedOccurrenceColoredAssembly struct {
	model  SHUOOccurrenceColoredAssembly
	unused []UnusedOccurrenceContext
}

func (m CombinedOccurrenceColoredAssembly) Paths() []NestedColoredPath { return m.model.Paths() }
func (m CombinedOccurrenceColoredAssembly) SharedOverrides() []OccurrenceSurfaceColor {
	return append([]OccurrenceSurfaceColor(nil), m.model.ordinary...)
}
func (m CombinedOccurrenceColoredAssembly) PathOverrides() []SHUOSurfaceColor {
	return m.model.Overrides()
}
func (m CombinedOccurrenceColoredAssembly) UnusedContexts() []UnusedOccurrenceContext {
	return append([]UnusedOccurrenceContext(nil), m.unused...)
}
func ReadCombinedOccurrenceColoredAssembly(ctx context.Context, r io.Reader, l Limits) (CombinedOccurrenceColoredAssembly, error) {
	var zero CombinedOccurrenceColoredAssembly
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	l, e := occurrenceColorSourceLimits(l)
	if e != nil {
		return zero, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return zero, e
	}
	raw, unused, e := combinedOccurrenceProofView(f, l)
	if e != nil {
		return zero, e
	}
	q, e := readSHUOOccurrenceColoredAssembly(ctx, bytes.NewReader(raw), l, true)
	if e != nil {
		return zero, e
	}
	q.model.model.Assembly.Assembly.Source = f
	return CombinedOccurrenceColoredAssembly{q, unused}, nil
}
func (m CombinedOccurrenceColoredAssembly) NativeAssembly(ctx context.Context) (ColoredAssembly, error) {
	raw, e := m.model.source()
	if e != nil {
		return ColoredAssembly{}, e
	}
	q, e := ReadCombinedOccurrenceColoredAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		return ColoredAssembly{}, e
	}
	return q.model.model.model.Assembly, nil
}
func WriteCombinedOccurrenceColoredAssemblySource(ctx context.Context, w io.Writer, m CombinedOccurrenceColoredAssembly) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	raw, e := m.model.source()
	if e != nil {
		return e
	}
	if _, e = ReadCombinedOccurrenceColoredAssembly(ctx, bytes.NewReader(raw), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}
