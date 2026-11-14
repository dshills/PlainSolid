package parametric

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/geom"
	"reflect"
)

// The exact support codec has fixed per-chart bounds independent of the outer
// document's potentially larger control/byte limits. Coefficients are canonical
// rational strings; no exponent expansion or float pole reconstruction occurs.
const exactSurfaceMaxBytes = 8 << 20
const exactSurfaceMaxDegree = 16
const exactSurfaceMaxControls = 289
const exactSurfaceMaxBits = 4096

type exactBezierSurfaceData geom.ExactBezierSurfaceRecipe

func (d *exactBezierSurfaceData) UnmarshalJSON(raw []byte) error {
	if len(raw) > exactSurfaceMaxBytes {
		return geom.ErrConvergence
	}
	var head struct {
		Version, DegreeU, DegreeV int
		Domain                    geom.ParameterRectangle
		Controls                  json.RawMessage
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if e := decoder.Decode(&head); e != nil {
		return e
	}
	if head.Version != 1 || head.DegreeU < 1 || head.DegreeU > exactSurfaceMaxDegree || head.DegreeV < 1 || head.DegreeV > exactSurfaceMaxDegree || !head.Domain.Valid() {
		return geom.ErrInvalid
	}
	count := (head.DegreeU + 1) * (head.DegreeV + 1)
	rows := json.NewDecoder(bytes.NewReader(head.Controls))
	token, e := rows.Token()
	if e != nil || token != json.Delim('[') {
		return geom.ErrInvalid
	}
	controls := make([][4]string, 0, count)
	for rows.More() {
		if len(controls) >= count {
			return geom.ErrInvalid
		}
		var row []string
		if e := rows.Decode(&row); e != nil {
			return e
		}
		if len(row) != 4 {
			return geom.ErrInvalid
		}
		var h [4]string
		for j, s := range row {
			if len(s) > exactSurfaceMaxBits+3 {
				return geom.ErrConvergence
			}
			h[j] = s
		}
		controls = append(controls, h)
	}
	token, e = rows.Token()
	if e != nil || token != json.Delim(']') || len(controls) != count {
		return geom.ErrInvalid
	}
	var extra any
	if e := rows.Decode(&extra); e != io.EOF {
		return geom.ErrInvalid
	}
	*d = exactBezierSurfaceData{Version: head.Version, DegreeU: head.DegreeU, DegreeV: head.DegreeV, Domain: head.Domain, Controls: controls}
	return nil
}

func encodeExactBezierSurface(s *geom.ExactBezierSurface) (*exactBezierSurfaceData, error) {
	if s == nil || s.ExactPatch() == nil {
		return nil, geom.ErrInvalid
	}
	p := s.ExactPatch()
	nu, nv := p.ControlShape()
	if nu > exactSurfaceMaxDegree+1 || nv > exactSurfaceMaxDegree+1 {
		return nil, geom.ErrConvergence
	}
	recipe, e := p.ExportExactBezierRecipe()
	if e != nil {
		return nil, e
	}
	if len(recipe.Controls) > exactSurfaceMaxControls {
		return nil, geom.ErrConvergence
	}
	// Re-import uses exactly the same acceptance bounds as the reader, ensuring
	// every emitted exact support can be restored without changing its geometry.
	if _, e = geom.ImportExactBezierSurfaceRecipe(context.Background(), recipe, geom.SurfacePatchOptions{MaxPoles: exactSurfaceMaxControls, MaxCoefficientBits: exactSurfaceMaxBits}); e != nil {
		return nil, e
	}
	data := exactBezierSurfaceData(recipe)
	return &data, nil
}
func (d *exactBezierSurfaceData) decodeExact(budget *int) (geom.Surface, error) {
	if d == nil || budget == nil || len(d.Controls) > *budget || len(d.Controls) > exactSurfaceMaxControls || d.DegreeU > exactSurfaceMaxDegree || d.DegreeV > exactSurfaceMaxDegree {
		return nil, geom.ErrConvergence
	}
	p, e := geom.ImportExactBezierSurfaceRecipe(context.Background(), geom.ExactBezierSurfaceRecipe(*d), geom.SurfacePatchOptions{MaxPoles: exactSurfaceMaxControls, MaxCoefficientBits: exactSurfaceMaxBits})
	if e != nil {
		return nil, e
	}
	s, e := geom.NewExactBezierSurface(p)
	if e != nil {
		return nil, e
	}
	*budget -= len(d.Controls)
	return s, nil
}

func validExactSurfaceEnvelope(d surfaceData) bool {
	d.Kind = ""
	d.ExactBezier = nil
	return reflect.ValueOf(d).IsZero()
}
