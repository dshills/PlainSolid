package parametric

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"plainsolid/geom"
)

const exactTensorMaxControls = 4096

type exactTensorSurfaceData geom.ExactTensorSurfaceRecipe

func (d *exactTensorSurfaceData) UnmarshalJSON(raw []byte) error {
	if len(raw) > exactSurfaceMaxBytes {
		return geom.ErrConvergence
	}
	var head struct {
		Version, DegreeU, DegreeV, CountU, CountV int
		KnotsU, KnotsV, Controls                  json.RawMessage
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if e := decoder.Decode(&head); e != nil {
		return e
	}
	if head.Version != 1 || head.DegreeU < 1 || head.DegreeU > exactSurfaceMaxDegree ||
		head.DegreeV < 1 || head.DegreeV > exactSurfaceMaxDegree ||
		head.CountU <= head.DegreeU || head.CountV <= head.DegreeV {
		return geom.ErrInvalid
	}
	if head.CountU > exactTensorMaxControls/head.CountV {
		return geom.ErrConvergence
	}
	u, e := decodeExactTensorKnots(head.KnotsU, head.CountU+head.DegreeU+1)
	if e != nil {
		return e
	}
	v, e := decodeExactTensorKnots(head.KnotsV, head.CountV+head.DegreeV+1)
	if e != nil {
		return e
	}
	h, e := decodeExactSurfaceControls(head.Controls, head.CountU*head.CountV)
	if e != nil {
		return e
	}
	*d = exactTensorSurfaceData{Version: head.Version, DegreeU: head.DegreeU, DegreeV: head.DegreeV,
		CountU: head.CountU, CountV: head.CountV, KnotsU: u, KnotsV: v, Controls: h}
	return nil
}

func decodeExactTensorKnots(raw []byte, count int) ([]float64, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	token, e := d.Token()
	if e != nil || token != json.Delim('[') {
		return nil, geom.ErrInvalid
	}
	knots := make([]float64, 0, count)
	for d.More() {
		if len(knots) == count {
			return nil, geom.ErrInvalid
		}
		token, e = d.Token()
		number, ok := token.(json.Number)
		if e != nil || !ok {
			return nil, geom.ErrInvalid
		}
		knot, e := number.Float64()
		if e != nil || !finite(knot) {
			return nil, geom.ErrInvalid
		}
		knots = append(knots, knot)
	}
	token, e = d.Token()
	if e != nil || token != json.Delim(']') || len(knots) != count {
		return nil, geom.ErrInvalid
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return nil, geom.ErrInvalid
	}
	return knots, nil
}

func encodeExactTensorSurface(s *geom.ExactTensorSurface) (*exactTensorSurfaceData, error) {
	if s == nil || s.ExactPatch() == nil {
		return nil, geom.ErrInvalid
	}
	o := geom.SurfacePatchOptions{MaxPoles: exactTensorMaxControls, MaxCoefficientBits: exactSurfaceMaxBits}
	ctx := context.Background()
	recipe, _, e := s.ExactPatch().ExportExactTensorRecipeBounded(ctx, exactSurfaceMaxBytes, o)
	if e != nil {
		return nil, e
	}
	if recipe.DegreeU > exactSurfaceMaxDegree || recipe.DegreeV > exactSurfaceMaxDegree {
		return nil, geom.ErrConvergence
	}
	if _, e := geom.ImportExactTensorSurfaceRecipe(ctx, recipe, o); e != nil {
		return nil, e
	}
	d := exactTensorSurfaceData(recipe)
	return &d, nil
}

func (d *exactTensorSurfaceData) decodeExact(budget *int) (geom.Surface, error) {
	if d == nil || budget == nil || len(d.Controls) > *budget || len(d.Controls) > exactTensorMaxControls {
		return nil, geom.ErrConvergence
	}
	ctx := context.Background()
	o := geom.SurfacePatchOptions{MaxPoles: exactTensorMaxControls, MaxCoefficientBits: exactSurfaceMaxBits}
	p, e := geom.ImportExactTensorSurfaceRecipe(ctx, geom.ExactTensorSurfaceRecipe(*d), o)
	if e != nil {
		return nil, e
	}
	s, e := geom.NewExactTensorSurface(ctx, p, o)
	if e != nil {
		return nil, e
	}
	*budget -= len(d.Controls)
	return s, nil
}
