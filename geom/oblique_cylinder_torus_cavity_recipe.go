package geom

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"
)

// This recipe persists exact analytic source, not a generic native BRep. No
// numeric geom.Frame, sampled mesh or stored verification flag is trusted.
type obliqueCavityRecipe struct {
	Kind    string       `json:"kind"`
	Version int          `json:"version"`
	Scalars [5]string    `json:"scalars"`
	Frame   [3][4]string `json:"frame"`
}

const obliqueCavityRecipeBytes = 64 << 10

func obliqueCavityRecipeRat(s string) (*big.Rat, error) {
	if len(s) == 0 || len(s) > 2500 {
		return nil, ErrConvergence
	}
	q, ok := new(big.Rat).SetString(s)
	if !ok || q.RatString() != s {
		return nil, ErrInvalid
	}
	if q.Num().BitLen() > 4096 || q.Denom().BitLen() > 4096 {
		return nil, ErrConvergence
	}
	return q, nil
}

// WriteObliqueCylinderTorusCavityRecipe emits deterministic canonical rational
// source data, bounded to64KiB/4096-bit scalars. The immutable source was already
// proved on construction; no derived bounds/moments are serialized as evidence.
// Geometry/preflight failure writes nothing; ordinary writer errors propagate.
func WriteObliqueCylinderTorusCavityRecipe(w io.Writer, s ObliqueCylinderTorusCavityMaterial) error {
	if w == nil || s.source.R == nil {
		return ErrInvalid
	}
	p := obliqueCavityRecipe{Kind: "exact-oblique-cylinder-torus-cavity", Version: 1}
	for i, q := range []*big.Rat{s.source.R, s.source.r, s.source.c, s.source.lower, s.source.upper} {
		p.Scalars[i] = q.RatString()
		if _, e := obliqueCavityRecipeRat(p.Scalars[i]); e != nil {
			return e
		}
	}
	for i := range p.Frame {
		for j := range p.Frame[i] {
			p.Frame[i][j] = s.pose[i][j].RatString()
			if _, e := obliqueCavityRecipeRat(p.Frame[i][j]); e != nil {
				return e
			}
		}
	}
	data, e := json.Marshal(p)
	if e != nil {
		return e
	}
	data = append(data, '\n')
	if len(data) > obliqueCavityRecipeBytes {
		return ErrConvergence
	}
	n, e := w.Write(data)
	if e == nil && n != len(data) {
		return io.ErrShortWrite
	}
	return e
}

// ReadObliqueCylinderTorusCavityRecipe reconstructs and re-proves exact proper
// rotation and strict containment. Point/line/bounds/moments subsequently derive
// from this same immutable source. Unknown fields, noncanonical rationals,
// trailing data, unsupported versions or excessive input reject atomically.
// Parsing is64KiB/4096-bit bounded; geometric proof uses caller arithmetic limits.
func ReadObliqueCylinderTorusCavityRecipe(ctx context.Context, r io.Reader, o SurfacePatchOptions) (ObliqueCylinderTorusCavityMaterial, error) {
	var zero ObliqueCylinderTorusCavityMaterial
	if ctx == nil || r == nil {
		return zero, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	data, e := io.ReadAll(io.LimitReader(r, obliqueCavityRecipeBytes+1))
	if e != nil {
		return zero, e
	}
	if len(data) > obliqueCavityRecipeBytes {
		return zero, ErrConvergence
	}
	var p obliqueCavityRecipe
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if e = d.Decode(&p); e != nil {
		return zero, e
	}
	var tail any
	if e = d.Decode(&tail); e != io.EOF {
		return zero, ErrInvalid
	}
	if p.Kind != "exact-oblique-cylinder-torus-cavity" || p.Version != 1 {
		return zero, ErrInvalid
	}
	var scalars [5]float64
	for i, v := range p.Scalars {
		q, e := obliqueCavityRecipeRat(v)
		if e != nil {
			return zero, e
		}
		scalars[i], e = RepresentedExactScalar(q)
		if e != nil {
			return zero, e
		}
	}
	var m [3][4]*big.Rat
	for i := range m {
		for j := range m[i] {
			m[i][j], e = obliqueCavityRecipeRat(p.Frame[i][j])
			if e != nil {
				return zero, e
			}
		}
	}
	return NewObliqueCylinderTorusCavityMaterial(ctx, scalars[0], scalars[1], scalars[2], scalars[3], scalars[4], m, o)
}
