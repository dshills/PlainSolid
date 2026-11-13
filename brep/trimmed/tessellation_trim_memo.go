package trimmed

import (
	"math"
	"plainsolid/geom"
)

// A memo belongs to one synchronous Tessellate invocation, across all levels
// and fold retries. Saturation changes only speed; no evaluator errors or
// nonfinite results are retained. Identity is a copied oriented source use,
// never merely an edge ID (two seam branches can share the same edge).
const tessellationTrimMemoLimit = 16384

type tessellationTrimKey struct {
	use       int
	parameter uint64
}
type tessellationTrimMemo struct {
	values       map[tessellationTrimKey]geom.Vec2
	limit        int
	hits, misses int
}
type tessellationMemoCurve struct {
	source PCurve
	memo   *tessellationTrimMemo
	id     int
}

func (c tessellationMemoCurve) EvalUV(t float64) (geom.Vec2, error) {
	key := tessellationTrimKey{c.id, math.Float64bits(t)}
	if p, ok := c.memo.values[key]; ok {
		c.memo.hits++
		return p, nil
	}
	c.memo.misses++
	p, e := c.source.EvalUV(t)
	if e == nil && uvFinite(p) && len(c.memo.values) < c.memo.limit {
		if c.memo.values == nil {
			c.memo.values = make(map[tessellationTrimKey]geom.Vec2)
		}
		c.memo.values[key] = p
	}
	return p, e
}

func (s Solid) withTessellationTrimMemo(memo *tessellationTrimMemo) Solid {
	if memo == nil {
		return s
	}
	curved := false
	for _, f := range s.faces {
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				curved = curved || u.Curve != nil
			}
		}
	}
	if !curved {
		return s
	}
	next := 0
	wrap := func(u Use) Use {
		if u.Curve != nil {
			u.Curve = tessellationMemoCurve{u.Curve, memo, next}
			next++
		}
		return u
	}
	// Seeds contain copies of uses made during construction. Give each seed
	// side its own identity; never infer curve equality from endpoints or type.
	// Its descendants preserve that identity throughout adaptive refinement.
	s.seeds = append([][]seed(nil), s.seeds...)
	for i, face := range s.seeds {
		s.seeds[i] = append([]seed(nil), face...)
		for j := range s.seeds[i] {
			for k := range s.seeds[i][j].side {
				u := &s.seeds[i][j].side[k]
				u.use = wrap(u.use)
			}
		}
	}
	s.faces = append([]Face(nil), s.faces...)
	for i := range s.faces {
		f := &s.faces[i]
		f.Outer = append([]Use(nil), f.Outer...)
		for j := range f.Outer {
			f.Outer[j] = wrap(f.Outer[j])
		}
		f.Holes = append([][]Use(nil), f.Holes...)
		for j := range f.Holes {
			f.Holes[j] = append([]Use(nil), f.Holes[j]...)
			for k := range f.Holes[j] {
				f.Holes[j][k] = wrap(f.Holes[j][k])
			}
		}
	}
	return s
}
