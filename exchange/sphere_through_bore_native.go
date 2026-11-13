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

// SphereThroughBoreRecipe describes the selected off-axis through bore. The
// source frame is retained separately from its nonrational analytic seam.
// LengthUnit is metres per model unit; no coordinate conversion is performed.
type SphereThroughBoreRecipe struct {
	Name                                                    string
	Origin, XAxis, ZAxis                                    geom.Vec3
	SphereRadius, BoreRadius, Offset, Tolerance, LengthUnit float64
}
type sphereThroughBoreEnvelope struct {
	Version  int
	Kind     string
	Recipe   SphereThroughBoreRecipe
	Boundary json.RawMessage
}

func sphereThroughBorePayload(ctx context.Context, r SphereThroughBoreRecipe) (sphereThroughBoreEnvelope, parametric.SphereOffAxisThroughBoreResult, error) {
	var empty parametric.SphereOffAxisThroughBoreResult
	if ctx == nil || !utf8.ValidString(r.Name) || len(r.Name) > 4096 || r.LengthUnit <= 0 || math.IsNaN(r.LengthUnit) || math.IsInf(r.LengthUnit, 0) {
		return sphereThroughBoreEnvelope{}, empty, geom.ErrDomain
	}
	f, e := geom.NewFrame(r.Origin, r.XAxis, r.ZAxis)
	if e != nil || f.XAxis() != r.XAxis || f.ZAxis() != r.ZAxis {
		return sphereThroughBoreEnvelope{}, empty, geom.ErrDomain
	}
	solid, e := parametric.SphereOffAxisThroughBore(ctx, f, r.SphereRadius, r.BoreRadius, r.Offset, r.Tolerance)
	if e != nil {
		return sphereThroughBoreEnvelope{}, empty, e
	}
	var b bytes.Buffer
	if e = parametric.WriteJSON(&b, solid.Solid); e != nil {
		return sphereThroughBoreEnvelope{}, empty, e
	}
	return sphereThroughBoreEnvelope{Version: 1, Kind: "analytic-off-axis-sphere-through-bore", Recipe: r, Boundary: json.RawMessage(bytes.TrimSpace(b.Bytes()))}, solid, nil
}

// WriteSphereThroughBoreRecipeJSON writes a canonical recipe-bound native BRep. The
// analytic intersection is retained by the native charts, never fitted to a
// rational STEP curve. Output is staged and construction must succeed first.
func WriteSphereThroughBoreRecipeJSON(ctx context.Context, w io.Writer, r SphereThroughBoreRecipe) error {
	if w == nil {
		return geom.ErrDomain
	}
	d, _, e := sphereThroughBorePayload(ctx, r)
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

// ReadSphereThroughBoreRecipeJSON reconstructs the analytic recipe and requires exact
// canonical JSON equality of its entire native geometry/topology payload.
// Whitespace is ignored; field ordering, number spelling and unique fields are
// canonical. Tampered source charts, incidence or duplicate fields fail closed.
// maxBytes defaults to 1 MiB and cannot exceed 16 MiB. This is not STEP import.
func ReadSphereThroughBoreRecipeJSON(ctx context.Context, r io.Reader, maxBytes int64) (SphereThroughBoreRecipe, parametric.SphereOffAxisThroughBoreResult, error) {
	fail := func() (SphereThroughBoreRecipe, parametric.SphereOffAxisThroughBoreResult, error) {
		return SphereThroughBoreRecipe{}, parametric.SphereOffAxisThroughBoreResult{}, fmt.Errorf("exchange: invalid or noncanonical sphere through-bore recipe")
	}
	if r == nil || ctx == nil || maxBytes < 0 || maxBytes > 16<<20 {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return SphereThroughBoreRecipe{}, parametric.SphereOffAxisThroughBoreResult{}, e
	}
	if maxBytes == 0 {
		maxBytes = 1 << 20
	}
	b, e := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if e != nil {
		return SphereThroughBoreRecipe{}, parametric.SphereOffAxisThroughBoreResult{}, e
	}
	if int64(len(b)) > maxBytes {
		return fail()
	}
	var d sphereThroughBoreEnvelope
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if e = dec.Decode(&d); e != nil {
		return fail()
	}
	if d.Version != 1 || d.Kind != "analytic-off-axis-sphere-through-bore" {
		return fail()
	}
	expected, solid, e := sphereThroughBorePayload(ctx, d.Recipe)
	if e != nil {
		return SphereThroughBoreRecipe{}, parametric.SphereOffAxisThroughBoreResult{}, e
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
