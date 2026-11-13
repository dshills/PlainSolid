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

// SphereCylinderPartialCommonRecipe describes the selected off-axis finite cylinder intersection with an upper lens cap. The
// source frame is retained separately from its nonrational analytic seam.
// Exact cap-event radicals and arc ownership are retained in local recipe
// coordinates; native azimuth charts retain their represented angle evaluation.
// LengthUnit is metres per model unit; no coordinate conversion is performed.
type SphereCylinderPartialCommonRecipe struct {
	Name                                                                      string
	Origin, XAxis, ZAxis                                                      geom.Vec3
	SphereRadius, CylinderRadius, Offset, Lower, Upper, Tolerance, LengthUnit float64
}
type sphereCylinderPartialCommonEnvelope struct {
	Version     int
	Kind        string
	Recipe      SphereCylinderPartialCommonRecipe
	Boundary    json.RawMessage
	CapMaterial json.RawMessage
}

func sphereCylinderPartialCommonPayload(ctx context.Context, r SphereCylinderPartialCommonRecipe) (sphereCylinderPartialCommonEnvelope, parametric.SphereOffAxisPartialCommonResult, error) {
	var empty parametric.SphereOffAxisPartialCommonResult
	if ctx == nil || !utf8.ValidString(r.Name) || len(r.Name) > 4096 || r.LengthUnit <= 0 || math.IsNaN(r.LengthUnit) || math.IsInf(r.LengthUnit, 0) {
		return sphereCylinderPartialCommonEnvelope{}, empty, geom.ErrDomain
	}
	f, e := geom.NewFrame(r.Origin, r.XAxis, r.ZAxis)
	if e != nil || f.XAxis() != r.XAxis || f.ZAxis() != r.ZAxis {
		return sphereCylinderPartialCommonEnvelope{}, empty, geom.ErrDomain
	}
	solid, e := parametric.SphereOffAxisCylinderPartialCommon(ctx, f, r.SphereRadius, r.CylinderRadius, r.Offset, r.Lower, r.Upper, r.Tolerance)
	if e != nil {
		return sphereCylinderPartialCommonEnvelope{}, empty, e
	}
	var b bytes.Buffer
	if e = parametric.WriteJSON(&b, solid.Solid); e != nil {
		return sphereCylinderPartialCommonEnvelope{}, empty, e
	}
	capData, e := json.Marshal(solid.UpperCapMaterial)
	if e != nil {
		return sphereCylinderPartialCommonEnvelope{}, empty, e
	}
	return sphereCylinderPartialCommonEnvelope{Version: 1, Kind: "analytic-off-axis-sphere-cylinder-partial-common", Recipe: r, CapMaterial: capData, Boundary: json.RawMessage(bytes.TrimSpace(b.Bytes()))}, solid, nil
}

// WriteSphereCylinderPartialCommonRecipeJSON writes a canonical recipe-bound native BRep. The
// analytic intersection is retained by the native charts, never fitted to a
// rational STEP curve. Output is staged and construction must succeed first.
func WriteSphereCylinderPartialCommonRecipeJSON(ctx context.Context, w io.Writer, r SphereCylinderPartialCommonRecipe) error {
	if w == nil {
		return geom.ErrDomain
	}
	d, _, e := sphereCylinderPartialCommonPayload(ctx, r)
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

// ReadSphereCylinderPartialCommonRecipeJSON reconstructs the analytic recipe and requires exact
// canonical JSON equality of its entire native geometry/topology payload.
// Whitespace is ignored; field ordering, number spelling and unique fields are
// canonical. Tampered source charts, incidence or duplicate fields fail closed.
// maxBytes defaults to 1 MiB and cannot exceed 16 MiB. This is not STEP import.
func ReadSphereCylinderPartialCommonRecipeJSON(ctx context.Context, r io.Reader, maxBytes int64) (SphereCylinderPartialCommonRecipe, parametric.SphereOffAxisPartialCommonResult, error) {
	fail := func() (SphereCylinderPartialCommonRecipe, parametric.SphereOffAxisPartialCommonResult, error) {
		return SphereCylinderPartialCommonRecipe{}, parametric.SphereOffAxisPartialCommonResult{}, fmt.Errorf("exchange: invalid or noncanonical sphere cylinder-common recipe")
	}
	if r == nil || ctx == nil || maxBytes < 0 || maxBytes > 16<<20 {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return SphereCylinderPartialCommonRecipe{}, parametric.SphereOffAxisPartialCommonResult{}, e
	}
	if maxBytes == 0 {
		maxBytes = 1 << 20
	}
	b, e := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if e != nil {
		return SphereCylinderPartialCommonRecipe{}, parametric.SphereOffAxisPartialCommonResult{}, e
	}
	if int64(len(b)) > maxBytes {
		return fail()
	}
	var d sphereCylinderPartialCommonEnvelope
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if e = dec.Decode(&d); e != nil {
		return fail()
	}
	if d.Version != 1 || d.Kind != "analytic-off-axis-sphere-cylinder-partial-common" {
		return fail()
	}
	expected, solid, e := sphereCylinderPartialCommonPayload(ctx, d.Recipe)
	if e != nil {
		return SphereCylinderPartialCommonRecipe{}, parametric.SphereOffAxisPartialCommonResult{}, e
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
