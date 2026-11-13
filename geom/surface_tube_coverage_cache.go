package geom

import (
	"context"
	"math/big"
	"strconv"
	"strings"
	"sync"
)

// SurfacePairTubeCoverageCache lazily reuses successful local tube proofs for
// one immutable ordered surface pair. It does not cache contact exclusion or
// accept caller certificates. Each cell is charged the original logical proof
// cost; insufficient budgets rerun the original path to preserve diagnostics.
// Concurrent calls are supported; simultaneous misses may prove independently.
type SurfacePairTubeCoverageCache struct {
	a, b       *SurfacePatch
	mu         sync.Mutex
	entries    map[string]tubeCoverageEntry
	maxEntries int
	stats      SurfaceTubeCoverageCacheStats
}

// SurfaceTubeCoverageCacheStats counts local proof reuse, not acceptance of
// candidate coverage. Misses includes failures and insufficient-work replays.
type SurfaceTubeCoverageCacheStats struct{ Hits, Misses, Stored, SavedOperations int }
type tubeCoverageEntry struct {
	certificate SurfaceTubeCertificate
	work        int
}

// NewSurfacePairTubeCoverageCache binds cache ownership to a,b. maxEntries zero
// means 4096; full caches keep validating without retaining additional proofs.
func NewSurfacePairTubeCoverageCache(a, b *SurfacePatch, maxEntries int) (*SurfacePairTubeCoverageCache, error) {
	if a == nil || b == nil || len(a.net) == 0 || len(b.net) == 0 || maxEntries < 0 || maxEntries > 4096 {
		return nil, ErrInvalid
	}
	if maxEntries == 0 {
		maxEntries = 4096
	}
	return &SurfacePairTubeCoverageCache{a: a, b: b, maxEntries: maxEntries, entries: make(map[string]tubeCoverageEntry)}, nil
}

// CertifyCell has the one-shot function's proof, work, bit and Pending semantics.
// Hints and rational anchors are copied before use; later mutations cannot
// modify cached authority. Callers must not mutate inputs during a call.
func (c *SurfacePairTubeCoverageCache) CertifyCell(ctx context.Context, candidate SurfacePairParameterBox, tubes []SurfacePairTubeRegion, o SurfaceTubeUnionCoverageOptions) (SurfaceTubeUnionCoverage, error) {
	if c == nil || c.a == nil || c.b == nil {
		return SurfaceTubeUnionCoverage{}, ErrInvalid
	}
	owned := append([]SurfacePairTubeRegion(nil), tubes...)
	for i := range owned {
		if owned[i].Boundary != nil {
			x := *owned[i].Boundary
			for j, v := range x.Anchor {
				if v != nil {
					x.Anchor[j] = new(big.Rat).Set(v)
				}
			}
			owned[i].Boundary = &x
		}
	}
	return certifySurfacePairCellInTubes(ctx, c.a, c.b, candidate, owned, o, c)
}
func (c *SurfacePairTubeCoverageCache) Stats() SurfaceTubeCoverageCacheStats {
	if c == nil {
		return SurfaceTubeCoverageCacheStats{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stats
}
func (c *SurfacePairTubeCoverageCache) lookup(key string, remaining int) (tubeCoverageEntry, bool) {
	if c == nil || key == "" {
		return tubeCoverageEntry{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if ok && e.work < remaining {
		c.stats.Hits++
		c.stats.SavedOperations += e.work
		return e, true
	}
	c.stats.Misses++
	return tubeCoverageEntry{}, false
}
func (c *SurfacePairTubeCoverageCache) remember(key string, e tubeCoverageEntry) {
	if c == nil || key == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.entries[key]; ok || len(c.entries) >= c.maxEntries {
		return
	}
	c.entries[key] = e
	c.stats.Stored++
}
func tubeCoverageKey(t SurfacePairTubeRegion, bits int) string {
	if t.Boundary != nil {
		for _, v := range t.Boundary.Anchor {
			if v != nil && (v.Num().BitLen() > bits || v.Denom().BitLen() > bits) {
				return ""
			}
		}
	}
	var b strings.Builder
	for _, v := range []float64{t.Box.A.UMin, t.Box.A.UMax, t.Box.A.VMin, t.Box.A.VMax, t.Box.B.UMin, t.Box.B.UMax, t.Box.B.VMin, t.Box.B.VMax} {
		b.WriteString(strconv.FormatFloat(v, 'x', -1, 64))
		b.WriteByte('/')
	}
	b.WriteString(strconv.Itoa(t.FreeAxis))
	b.WriteByte('/')
	b.WriteString(strconv.Itoa(bits))
	if t.Boundary != nil {
		b.WriteString("/boundary/")
		b.WriteString(strconv.FormatBool(t.Boundary.AtUpper))
		for _, v := range t.Boundary.Anchor {
			b.WriteByte('/')
			if v == nil {
				b.WriteString("nil")
			} else {
				b.WriteString(v.RatString())
			}
		}
	}
	return b.String()
}
