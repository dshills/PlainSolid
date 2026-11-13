package exchange

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"unicode/utf8"
)

// SphereCylinderCommonRecipe describes the selected off-axis finite cylinder intersection. The
// source frame is retained separately from its nonrational analytic seam.
// LengthUnit is metres per model unit; no coordinate conversion is performed.
type SphereCylinderCommonRecipe struct {
	Name                                                                      string
	Origin, XAxis, ZAxis                                                      geom.Vec3
	SphereRadius, CylinderRadius, Offset, Lower, Upper, Tolerance, LengthUnit float64
}
type sphereCylinderCommonEnvelope struct {
	Version  int
	Kind     string
	Recipe   SphereCylinderCommonRecipe
	Boundary json.RawMessage
}

func sphereCylinderCommonPayload(ctx context.Context, r SphereCylinderCommonRecipe) (sphereCylinderCommonEnvelope, parametric.SphereOffAxisCommonResult, error) {
	var empty parametric.SphereOffAxisCommonResult
	if ctx == nil || !utf8.ValidString(r.Name) || len(r.Name) > 4096 || r.LengthUnit <= 0 || math.IsNaN(r.LengthUnit) || math.IsInf(r.LengthUnit, 0) {
		return sphereCylinderCommonEnvelope{}, empty, geom.ErrDomain
	}
	f, e := geom.NewFrame(r.Origin, r.XAxis, r.ZAxis)
	if e != nil || f.XAxis() != r.XAxis || f.ZAxis() != r.ZAxis {
		return sphereCylinderCommonEnvelope{}, empty, geom.ErrDomain
	}
	solid, e := parametric.SphereOffAxisCylinderCommon(ctx, f, r.SphereRadius, r.CylinderRadius, r.Offset, r.Lower, r.Upper, r.Tolerance)
	if e != nil {
		return sphereCylinderCommonEnvelope{}, empty, e
	}
	var b bytes.Buffer
	if e = parametric.WriteJSON(&b, solid.Solid); e != nil {
		return sphereCylinderCommonEnvelope{}, empty, e
	}
	return sphereCylinderCommonEnvelope{Version: 1, Kind: "analytic-off-axis-sphere-cylinder-common", Recipe: r, Boundary: json.RawMessage(bytes.TrimSpace(b.Bytes()))}, solid, nil
}

// WriteSphereCylinderCommonRecipeJSON writes a canonical recipe-bound native BRep. The
// analytic intersection is retained by the native charts, never fitted to a
// rational STEP curve. Output is staged and construction must succeed first.
func WriteSphereCylinderCommonRecipeJSON(ctx context.Context, w io.Writer, r SphereCylinderCommonRecipe) error {
	if w == nil {
		return geom.ErrDomain
	}
	d, _, e := sphereCylinderCommonPayload(ctx, r)
	if e != nil {
		return e
	}
	b, e := json.Marshal(d)
	if e != nil {
		return e
	}
	b = append(b, '\n')
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		e = io.ErrShortWrite
	}
	return e
}

// ReadSphereCylinderCommonRecipeJSON reconstructs the analytic recipe and requires exact
// canonical JSON equality of its entire native geometry/topology payload.
// Whitespace is ignored; field ordering, number spelling and unique fields are
// canonical. Tampered source charts, incidence or duplicate fields fail closed.
// maxBytes defaults to 1 MiB and cannot exceed 16 MiB. This is not STEP import.
func ReadSphereCylinderCommonRecipeJSON(ctx context.Context, r io.Reader, maxBytes int64) (SphereCylinderCommonRecipe, parametric.SphereOffAxisCommonResult, error) {
	fail := func() (SphereCylinderCommonRecipe, parametric.SphereOffAxisCommonResult, error) {
		return SphereCylinderCommonRecipe{}, parametric.SphereOffAxisCommonResult{}, fmt.Errorf("exchange: invalid or noncanonical sphere cylinder-common recipe")
	}
	if r == nil || ctx == nil || maxBytes < 0 || maxBytes > 16<<20 {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return SphereCylinderCommonRecipe{}, parametric.SphereOffAxisCommonResult{}, e
	}
	if maxBytes == 0 {
		maxBytes = 1 << 20
	}
	b, e := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if e != nil {
		return SphereCylinderCommonRecipe{}, parametric.SphereOffAxisCommonResult{}, e
	}
	if int64(len(b)) > maxBytes {
		return fail()
	}
	var d sphereCylinderCommonEnvelope
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if e = dec.Decode(&d); e != nil {
		return fail()
	}
	if d.Version != 1 || d.Kind != "analytic-off-axis-sphere-cylinder-common" {
		return fail()
	}
	expected, solid, e := sphereCylinderCommonPayload(ctx, d.Recipe)
	if e != nil {
		return SphereCylinderCommonRecipe{}, parametric.SphereOffAxisCommonResult{}, e
	}
	raw, e := json.Marshal(expected)
	if e != nil {
		return fail()
	}
	var compact bytes.Buffer
	if e = json.Compact(&compact, b); e != nil || !bytes.Equal(compact.Bytes(), raw) {
		return fail()
	}
	return d.Recipe, solid, nil
}
