package intersect

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"

	"plainsolid/geom"
)

// TubeBoundaryAnchor proposes an exact point in both charts at one free
// endpoint. Parameters is the legacy binary64 representation. Use
// NewTubeBoundaryAnchor when a dependent coordinate requires a rational.
// Every proof rechecks the original support equality and inward continuation.
type TubeBoundaryAnchor struct {
	AtUpper    bool
	Parameters [4]float64
	rational   [4]string
}

const tubeAnchorMaxBits = 16384

// NewTubeBoundaryAnchor copies four exact coordinates. Binary64 coordinates
// retain the legacy representation, including its native JSON fields.
func NewTubeBoundaryAnchor(atUpper bool, parameters [4]*big.Rat) (*TubeBoundaryAnchor, error) {
	r := &TubeBoundaryAnchor{AtUpper: atUpper}
	var represented [4]float64
	allRepresented := true
	for i, q := range parameters {
		if q == nil {
			return nil, geom.ErrInvalid
		}
		if q.Num().BitLen() > tubeAnchorMaxBits || q.Denom().BitLen() > tubeAnchorMaxBits {
			return nil, geom.ErrConvergence
		}
		x, exact := q.Float64()
		represented[i] = x
		allRepresented = allRepresented && exact && finite(x)
	}
	if allRepresented {
		r.Parameters = represented
	} else {
		for i, q := range parameters {
			r.rational[i] = q.RatString()
		}
	}
	return r, nil
}

func cloneTubeBoundaryAnchor(a *TubeBoundaryAnchor) *TubeBoundaryAnchor {
	if a == nil {
		return nil
	}
	v := *a
	return &v
}

// ExactOptions gives a caller-owned proposal in original UV units. It does
// not certify that the point lies on either original support.
func (a *TubeBoundaryAnchor) ExactOptions(o geom.SurfacePairTubeOptions) (geom.SurfaceTubeBoundaryOptions, error) {
	if a == nil {
		return geom.SurfaceTubeBoundaryOptions{}, geom.ErrInvalid
	}
	r := geom.SurfaceTubeBoundaryOptions{SurfacePairTubeOptions: o, AtUpper: a.AtUpper}
	if a.rational != ([4]string{}) {
		if a.Parameters != ([4]float64{}) {
			return r, geom.ErrInvalid
		}
		bits := o.MaxCoefficientBits
		if bits == 0 {
			bits = tubeAnchorMaxBits
		}
		for i, text := range a.rational {
			q, e := readTubeAnchorRational(text, min(bits, tubeAnchorMaxBits))
			if e != nil {
				return r, e
			}
			r.Anchor[i] = q
		}
		return r, nil
	}
	for i, v := range a.Parameters {
		if !finite(v) {
			return geom.SurfaceTubeBoundaryOptions{}, geom.ErrInvalid
		}
		r.Anchor[i] = new(big.Rat).SetFloat64(v)
	}
	return r, nil
}

func readTubeAnchorRational(text string, bits int) (*big.Rat, error) {
	if bits < 64 {
		return nil, geom.ErrInvalid
	}
	if len(text) > bits+3 {
		return nil, geom.ErrConvergence
	}
	for _, ch := range text {
		if ch != '-' && ch != '/' && (ch < '0' || ch > '9') {
			return nil, geom.ErrInvalid
		}
	}
	q, ok := new(big.Rat).SetString(text)
	if !ok || q.RatString() != text {
		return nil, geom.ErrInvalid
	}
	if q.Num().BitLen() > bits || q.Denom().BitLen() > bits {
		return nil, geom.ErrConvergence
	}
	return q, nil
}

func (a TubeBoundaryAnchor) MarshalJSON() ([]byte, error) {
	if a.rational == ([4]string{}) {
		return json.Marshal(struct {
			AtUpper    bool
			Parameters [4]float64
		}{a.AtUpper, a.Parameters})
	}
	if _, e := a.ExactOptions(geom.SurfacePairTubeOptions{}); e != nil {
		return nil, e
	}
	return json.Marshal(struct {
		AtUpper            bool
		RationalParameters [4]string
	}{a.AtUpper, a.rational})
}

func (a *TubeBoundaryAnchor) UnmarshalJSON(raw []byte) error {
	if len(raw) > 4*(tubeAnchorMaxBits+3)+1024 {
		return geom.ErrConvergence
	}
	var wire struct {
		AtUpper                        bool
		Parameters, RationalParameters json.RawMessage
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(&wire); e != nil {
		return e
	}
	if (wire.Parameters == nil) == (wire.RationalParameters == nil) {
		return geom.ErrInvalid
	}
	r := TubeBoundaryAnchor{AtUpper: wire.AtUpper}
	if wire.Parameters != nil {
		var p []*float64
		if e := json.Unmarshal(wire.Parameters, &p); e != nil || len(p) != 4 {
			return geom.ErrInvalid
		}
		for i, x := range p {
			if x == nil || !finite(*x) {
				return geom.ErrInvalid
			}
			r.Parameters[i] = *x
		}
	} else {
		var p []string
		if e := json.Unmarshal(wire.RationalParameters, &p); e != nil || len(p) != 4 {
			return geom.ErrInvalid
		}
		copy(r.rational[:], p)
		if r.rational == ([4]string{}) {
			return geom.ErrInvalid
		}
		if _, e := r.ExactOptions(geom.SurfacePairTubeOptions{}); e != nil {
			return e
		}
	}
	*a = r
	return nil
}

func verifySplineTubeCell(ctx context.Context, a, b *geom.SurfacePatch, o geom.SurfacePairTubeOptions, anchor *TubeBoundaryAnchor) (geom.VerifiedSurfaceTube, error) {
	if anchor == nil {
		return geom.NewVerifiedSurfaceTube(a, b, o)
	}
	x, e := anchor.ExactOptions(o)
	if e != nil {
		return geom.VerifiedSurfaceTube{}, e
	}
	return geom.NewBoundaryAnchoredSurfaceTube(ctx, a, b, x)
}

func certifySplineTubeCell(ctx context.Context, a, b *geom.SurfacePatch, o geom.SurfacePairTubeOptions, anchor *TubeBoundaryAnchor) (geom.SurfaceTubeCertificate, error) {
	if anchor == nil {
		return geom.CertifySurfacePairTube(a, b, o)
	}
	x, e := anchor.ExactOptions(o)
	if e != nil {
		return geom.SurfaceTubeCertificate{}, e
	}
	return geom.CertifyBoundaryAnchoredSurfacePairTube(ctx, a, b, x)
}
