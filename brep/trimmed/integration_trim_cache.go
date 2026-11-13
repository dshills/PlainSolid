package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
	"plainsolid/intersect"
)

// All caches are private to one synchronous integration call. Capacity limits
// affect only speed; misses retain the original evaluator and error semantics.
const massTrimCacheLimit = 16384
const massAffineProofLimit = 256

type massTrimKey struct {
	id int
	t  float64
}
type massTrimSample struct {
	point, derivative       geom.Vec2
	hasPoint, hasDerivative bool
}
type massTrimMemo struct {
	values map[massTrimKey]massTrimSample
}
type massMemoTrim struct {
	source differentiableTrim
	cache  *massTrimMemo
	id     int
}

func (c massMemoTrim) EvalUV(t float64) (geom.Vec2, error) {
	key := massTrimKey{c.id, t}
	v, exists := c.cache.values[key]
	if v.hasPoint {
		return v.point, nil
	}
	p, e := c.source.EvalUV(t)
	if e == nil && (exists || len(c.cache.values) < massTrimCacheLimit) {
		v.point, v.hasPoint = p, true
		c.cache.values[key] = v
	}
	return p, e
}
func (c massMemoTrim) DerivativeUV(t float64) (geom.Vec2, error) {
	key := massTrimKey{c.id, t}
	v, exists := c.cache.values[key]
	if v.hasDerivative {
		return v.derivative, nil
	}
	p, e := c.source.DerivativeUV(t)
	if e == nil && (exists || len(c.cache.values) < massTrimCacheLimit) {
		v.derivative, v.hasDerivative = p, true
		c.cache.values[key] = v
	}
	return p, e
}

type massProofKey struct {
	recipe  intersect.TubeRecipe
	segment int
}
type massAffineProofs struct {
	left   int
	values map[massProofKey]geom.TubeAffineLineCertificate
}

func (p *massAffineProofs) zeroStrip(ctx context.Context, u Use) (bool, error) {
	trim, reversed, ok := trimTube(u.Curve)
	if !ok {
		return false, nil
	}
	recipe, index, second := trim.Recipe()
	key := massProofKey{recipe, index}
	cert, seen := p.values[key]
	if !seen {
		if p.left <= 0 || len(p.values) >= massAffineProofLimit {
			return false, nil
		}
		var e error
		cert, e = trim.CertifyAffineChord(ctx, geom.TubeAffineLineOptions{MaxOperations: p.left})
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if e != nil {
			cert.Proven = false
			p.left = 0
		} else {
			p.left -= max(1, cert.ExactOperations)
		}
		p.values[key] = cert
	}
	if !cert.Proven {
		return false, nil
	}
	offset := 0
	if second {
		offset = 2
	}
	a, b := cert.Start, cert.End
	if reversed {
		a, b = b, a
	}
	// The original face chord must equal the proved parameter endpoints exactly.
	// A tolerance-close chord has a nonzero strip and must still be integrated.
	for k, x := range []float64{u.UVStart.X, u.UVStart.Y, u.UVEnd.X, u.UVEnd.Y} {
		endpoint := a
		if k >= 2 {
			endpoint = b
		}
		q := new(big.Rat).SetFloat64(x)
		if q == nil || endpoint[offset+k%2].Cmp(q) != 0 {
			return false, nil
		}
	}
	return true, nil
}
