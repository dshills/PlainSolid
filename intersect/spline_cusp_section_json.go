package intersect

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// CuspSectionPersistenceOptions bounds the recipe before surface recovery and
// re-proves the entire section. Defaults: 2MiB, 4096 controls per source.
// Patch recovery stages and the cumulative contact proof have separate limits.
type CuspSectionPersistenceOptions struct {
	MaxBytes, MaxControlPoints int
	Patch                      geom.SurfacePatchOptions
	Proof                      geom.GraphPointContactOptions
}
type cuspSectionOwnerRecipe struct {
	Surface  json.RawMessage
	Min, Max geom.Vec2
}
type cuspSectionRecipe struct {
	Version int
	Kind    string
	Owners  [2]cuspSectionOwnerRecipe
	Center  [2]string
	K, Tau  string
}

func cuspPersistenceOptions(o CuspSectionPersistenceOptions) (CuspSectionPersistenceOptions, error) {
	if o.MaxBytes == 0 {
		o.MaxBytes = 2 << 20
	}
	if o.MaxControlPoints == 0 {
		o.MaxControlPoints = 4096
	}
	if o.MaxBytes < 1 || o.MaxBytes > 16<<20 || o.MaxControlPoints < 4 || o.MaxControlPoints > 65536 {
		return o, geom.ErrInvalid
	}
	if o.Patch.MaxPoles == 0 || o.Patch.MaxPoles > o.MaxControlPoints {
		o.Patch.MaxPoles = o.MaxControlPoints
	}
	return o, nil
}

// MarshalCuspSection re-proves original source ownership and the complete cusp
// before publishing bytes. Only authoring inputs are stored, never proof flags,
// derived points, sampled curves or rounded terminal coordinates.
func MarshalCuspSection(ctx context.Context, s *SplineCuspSection, o CuspSectionPersistenceOptions) ([]byte, error) {
	if ctx == nil || !s.Complete() {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	o, e := cuspPersistenceOptions(o)
	if e != nil {
		return nil, e
	}
	_, e = CertifySplinePairCuspSection(ctx, s.owners[0], s.owners[1], s.center[0], s.center[1], s.k, s.tau, o.Patch, o.Proof)
	if e != nil {
		return nil, e
	}
	q := cuspSectionRecipe{Version: 1, Kind: "spline-cusp-section", Center: [2]string{s.center[0].RatString(), s.center[1].RatString()}, K: s.k.RatString(), Tau: s.tau.RatString()}
	for i, p := range s.owners {
		raw, e := parametric.EncodeSurface(p.Surface)
		if e != nil {
			return nil, e
		}
		if len(raw) > o.MaxBytes {
			return nil, geom.ErrConvergence
		}
		q.Owners[i] = cuspSectionOwnerRecipe{Surface: raw, Min: p.Min, Max: p.Max}
	}
	raw, e := json.Marshal(q)
	if e != nil {
		return nil, e
	}
	if len(raw) > o.MaxBytes {
		return nil, geom.ErrConvergence
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	return raw, nil
}

// UnmarshalCuspSection bounds input, restores original exact supports and
// rebuilds the complete immutable certificate. Unsupported versions, unknown
// fields, omitted sources and any failed proof publish no result.
func UnmarshalCuspSection(ctx context.Context, raw []byte, o CuspSectionPersistenceOptions) (*SplineCuspSection, error) {
	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	o, e := cuspPersistenceOptions(o)
	if e != nil {
		return nil, e
	}
	if len(raw) == 0 {
		return nil, geom.ErrInvalid
	}
	if len(raw) > o.MaxBytes {
		return nil, geom.ErrConvergence
	}
	var q cuspSectionRecipe
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e = d.Decode(&q); e != nil {
		return nil, e
	}
	if e = d.Decode(new(any)); e != io.EOF {
		return nil, geom.ErrInvalid
	}
	if q.Version != 1 || q.Kind != "spline-cusp-section" {
		return nil, geom.ErrInvalid
	}
	var values [4]*big.Rat
	for i, text := range [4]string{q.Center[0], q.Center[1], q.K, q.Tau} {
		if len(text) == 0 || len(text) > 10000 {
			return nil, geom.ErrInvalid
		}
		for j, ch := range text {
			if (ch < '0' || ch > '9') && ch != '/' && !(ch == '-' && j == 0) {
				return nil, geom.ErrInvalid
			}
		}
		v, ok := new(big.Rat).SetString(text)
		if !ok || v.RatString() != text {
			return nil, geom.ErrInvalid
		}
		bits := o.Proof.MaxCoefficientBits
		if bits == 0 {
			bits = 16384
		}
		if v.Num().BitLen() > bits || v.Denom().BitLen() > bits {
			return nil, geom.ErrConvergence
		}
		values[i] = v
	}
	var owners [2]Patch
	for i, p := range q.Owners {
		if e = ctx.Err(); e != nil {
			return nil, e
		}
		if len(p.Surface) == 0 {
			return nil, geom.ErrInvalid
		}
		surface, e := parametric.DecodeSurface(p.Surface, o.MaxControlPoints)
		if e != nil {
			return nil, e
		}
		owners[i] = Patch{Surface: surface, Min: p.Min, Max: p.Max}
	}
	return CertifySplinePairCuspSection(ctx, owners[0], owners[1], values[0], values[1], values[2], values[3], o.Patch, o.Proof)
}
