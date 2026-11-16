package parametric

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"reflect"

	"plainsolid/geom"
)

const exactPeriodicMaxControls = 4096

type exactPeriodicAxisData struct {
	Degree, Count int
	Periodic      bool
	Knots         []string
}

type exactPeriodicSplineData struct {
	Version  int
	U        exactPeriodicAxisData
	V        *exactPeriodicAxisData `json:",omitempty"`
	Controls [][4]string
}

func (d *exactPeriodicSplineData) UnmarshalJSON(raw []byte) error {
	if len(raw) > exactSurfaceMaxBytes {
		return geom.ErrConvergence
	}
	var head struct {
		Version        int
		U, V, Controls json.RawMessage
	}
	if err := decodeExactPeriodicJSON(raw, &head); err != nil {
		return err
	}
	if head.Version != 1 {
		return geom.ErrInvalid
	}
	u, err := decodeExactPeriodicAxis(head.U)
	if err != nil {
		return err
	}
	count := u.Count
	var v *exactPeriodicAxisData
	if head.V != nil {
		other, err := decodeExactPeriodicAxis(head.V)
		if err != nil {
			return err
		}
		if count > exactPeriodicMaxControls/other.Count {
			return geom.ErrConvergence
		}
		count *= other.Count
		v = &other
	}
	if !u.Periodic && (v == nil || !v.Periodic) {
		return geom.ErrInvalid
	}
	h, err := decodeExactSurfaceControls(head.Controls, count)
	if err != nil {
		return err
	}
	*d = exactPeriodicSplineData{head.Version, u, v, h}
	return nil
}

func decodeExactPeriodicJSON(raw []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return geom.ErrInvalid
	}
	return nil
}

func decodeExactPeriodicAxis(raw []byte) (exactPeriodicAxisData, error) {
	var a exactPeriodicAxisData
	var h struct {
		Degree, Count int
		Periodic      bool
		Knots         json.RawMessage
	}
	if err := decodeExactPeriodicJSON(raw, &h); err != nil {
		return a, err
	}
	if h.Degree < 1 || h.Degree > exactSurfaceMaxDegree || h.Count <= h.Degree {
		return a, geom.ErrInvalid
	}
	if h.Count > exactPeriodicMaxControls {
		return a, geom.ErrConvergence
	}
	n := h.Count + 1
	if !h.Periodic {
		n += h.Degree
	}
	d := json.NewDecoder(bytes.NewReader(h.Knots))
	start, err := d.Token()
	if err != nil || start != json.Delim('[') {
		return a, geom.ErrInvalid
	}
	knots := make([]string, 0, n)
	for d.More() {
		if len(knots) == n {
			return a, geom.ErrInvalid
		}
		var value string
		if err := d.Decode(&value); err != nil {
			return a, err
		}
		if len(value) > exactSurfaceMaxBits+3 {
			return a, geom.ErrConvergence
		}
		knots = append(knots, value)
	}
	end, err := d.Token()
	if err != nil || end != json.Delim(']') || len(knots) != n {
		return a, geom.ErrInvalid
	}
	return exactPeriodicAxisData{h.Degree, h.Count, h.Periodic, knots}, nil
}

func exactPeriodicTextValue(text string) (*big.Rat, error) {
	if len(text) > exactSurfaceMaxBits+3 {
		return nil, geom.ErrConvergence
	}
	for i := range len(text) {
		c := text[i]
		if c != '/' && c != '-' && (c < '0' || c > '9') {
			return nil, geom.ErrInvalid
		}
	}
	v, ok := new(big.Rat).SetString(text)
	if !ok || v.RatString() != text {
		return nil, geom.ErrInvalid
	}
	if v.Num().BitLen() > exactSurfaceMaxBits || v.Denom().BitLen() > exactSurfaceMaxBits {
		return nil, geom.ErrConvergence
	}
	return v, nil
}

func (d exactPeriodicAxisData) decode() (geom.ExactTensorSplineAxis, error) {
	a := geom.ExactTensorSplineAxis{Degree: d.Degree, Count: d.Count, Periodic: d.Periodic, Knots: make([]*big.Rat, len(d.Knots))}
	for i, value := range d.Knots {
		var err error
		a.Knots[i], err = exactPeriodicTextValue(value)
		if err != nil {
			return geom.ExactTensorSplineAxis{}, err
		}
	}
	return a, nil
}

func (d *exactPeriodicSplineData) decode(budget *int) (curve geom.Curve, surface geom.Surface, err error) {
	if len(d.Controls) > *budget {
		return nil, nil, geom.ErrConvergence
	}
	u, err := d.U.decode()
	if err != nil {
		return nil, nil, err
	}
	h := make([][4]*big.Rat, len(d.Controls))
	for i, row := range d.Controls {
		for j, text := range row {
			h[i][j], err = exactPeriodicTextValue(text)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	opts := geom.SurfacePatchOptions{MaxPoles: exactPeriodicMaxControls, MaxCoefficientBits: exactSurfaceMaxBits}
	if d.V == nil {
		curve, err = geom.NewExactPeriodicSplineCurve(context.Background(), u.Degree, u.Knots, h, opts)
	} else {
		v, decodeErr := d.V.decode()
		if decodeErr != nil {
			return nil, nil, decodeErr
		}
		surface, err = geom.NewExactPeriodicSplineSurface(context.Background(), u, v, h, opts)
	}
	if err != nil {
		return nil, nil, err
	}
	*budget -= len(h)
	return curve, surface, nil
}

func encodeExactPeriodicSpline(u geom.ExactTensorSplineAxis, v *geom.ExactTensorSplineAxis, h [][4]*big.Rat) (*exactPeriodicSplineData, error) {
	if u.Degree > exactSurfaceMaxDegree || v != nil && v.Degree > exactSurfaceMaxDegree {
		return nil, geom.ErrConvergence
	}
	size := 256
	text := func(q *big.Rat) (string, error) {
		n, d := q.Num().BitLen(), q.Denom().BitLen()
		if n > exactSurfaceMaxBits || d > exactSurfaceMaxBits {
			return "", geom.ErrConvergence
		}
		size += n/3 + d/3 + 12 // Outward bound for canonical decimal digits and JSON syntax.
		if size > exactSurfaceMaxBytes {
			return "", geom.ErrConvergence
		}
		return q.RatString(), nil
	}
	axis := func(a geom.ExactTensorSplineAxis) (exactPeriodicAxisData, error) {
		out := exactPeriodicAxisData{a.Degree, a.Count, a.Periodic, make([]string, len(a.Knots))}
		for i, knot := range a.Knots {
			var err error
			out.Knots[i], err = text(knot)
			if err != nil {
				return exactPeriodicAxisData{}, err
			}
		}
		return out, nil
	}
	ua, err := axis(u)
	if err != nil {
		return nil, err
	}
	r := &exactPeriodicSplineData{Version: 1, U: ua, Controls: make([][4]string, len(h))}
	if v != nil {
		va, err := axis(*v)
		if err != nil {
			return nil, err
		}
		r.V = &va
	}
	for i, row := range h {
		for j, q := range row {
			r.Controls[i][j], err = text(q)
			if err != nil {
				return nil, err
			}
		}
	}
	// A period is derived from two knots. Its bit length can exceed
	// either input, so apply the exact read budget before emitting bytes.
	budget := exactPeriodicMaxControls
	if _, _, err := r.decode(&budget); err != nil {
		return nil, err
	}
	return r, nil
}

func validExactPeriodicCurveEnvelope(d curveData) bool {
	d.Kind, d.ExactPeriodic = "", nil
	return reflect.ValueOf(d).IsZero()
}
