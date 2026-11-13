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

// SphereBoreRecipe describes the selected north-side off-axis blind bore. The
// source frame is retained separately from its nonrational analytic seam.
// LengthUnit is metres per model unit; no coordinate conversion is performed.
type SphereBoreRecipe struct {
	Name                                                           string
	Origin, XAxis, ZAxis                                           geom.Vec3
	SphereRadius, BoreRadius, Offset, Floor, Tolerance, LengthUnit float64
}
type sphereBoreEnvelope struct {
	Version  int
	Kind     string
	Recipe   SphereBoreRecipe
	Boundary json.RawMessage
}

func sphereBorePayload(ctx context.Context, r SphereBoreRecipe) (sphereBoreEnvelope, parametric.SphereOffAxisBlindBoreResult, error) {
	var empty parametric.SphereOffAxisBlindBoreResult
	if ctx == nil || !utf8.ValidString(r.Name) || len(r.Name) > 4096 || r.LengthUnit <= 0 || math.IsNaN(r.LengthUnit) || math.IsInf(r.LengthUnit, 0) {
		return sphereBoreEnvelope{}, empty, geom.ErrDomain
	}
	f, e := geom.NewFrame(r.Origin, r.XAxis, r.ZAxis)
	if e != nil || f.XAxis() != r.XAxis || f.ZAxis() != r.ZAxis {
		return sphereBoreEnvelope{}, empty, geom.ErrDomain
	}
	solid, e := parametric.SphereOffAxisBlindBore(ctx, f, r.SphereRadius, r.BoreRadius, r.Offset, r.Floor, r.Tolerance)
	if e != nil {
		return sphereBoreEnvelope{}, empty, e
	}
	var b bytes.Buffer
	if e = parametric.WriteJSON(&b, solid.Solid); e != nil {
		return sphereBoreEnvelope{}, empty, e
	}
	return sphereBoreEnvelope{Version: 1, Kind: "analytic-off-axis-sphere-blind-bore", Recipe: r, Boundary: json.RawMessage(bytes.TrimSpace(b.Bytes()))}, solid, nil
}

// WriteSphereBoreRecipeJSON writes a canonical recipe-bound native BRep. The
// analytic intersection is retained by the native charts, never fitted to a
// rational STEP curve. Output is staged and construction must succeed first.
func WriteSphereBoreRecipeJSON(ctx context.Context, w io.Writer, r SphereBoreRecipe) error {
	if w == nil {
		return geom.ErrDomain
	}
	d, _, e := sphereBorePayload(ctx, r)
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

// ReadSphereBoreRecipeJSON reconstructs the analytic recipe and requires exact
// canonical JSON equality of its entire native geometry/topology payload.
// Whitespace is ignored; field ordering, number spelling and unique fields are
// canonical. Tampered source charts, incidence or duplicate fields fail closed.
// maxBytes defaults to 1 MiB and cannot exceed 16 MiB. This is not STEP import.
func ReadSphereBoreRecipeJSON(ctx context.Context, r io.Reader, maxBytes int64) (SphereBoreRecipe, parametric.SphereOffAxisBlindBoreResult, error) {
	fail := func() (SphereBoreRecipe, parametric.SphereOffAxisBlindBoreResult, error) {
		return SphereBoreRecipe{}, parametric.SphereOffAxisBlindBoreResult{}, fmt.Errorf("exchange: invalid or noncanonical sphere bore recipe")
	}
	if r == nil || ctx == nil || maxBytes < 0 || maxBytes > 16<<20 {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return SphereBoreRecipe{}, parametric.SphereOffAxisBlindBoreResult{}, e
	}
	if maxBytes == 0 {
		maxBytes = 1 << 20
	}
	b, e := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if e != nil {
		return SphereBoreRecipe{}, parametric.SphereOffAxisBlindBoreResult{}, e
	}
	if int64(len(b)) > maxBytes {
		return fail()
	}
	var d sphereBoreEnvelope
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if e = dec.Decode(&d); e != nil {
		return fail()
	}
	if d.Version != 1 || d.Kind != "analytic-off-axis-sphere-blind-bore" {
		return fail()
	}
	expected, solid, e := sphereBorePayload(ctx, d.Recipe)
	if e != nil {
		return SphereBoreRecipe{}, parametric.SphereOffAxisBlindBoreResult{}, e
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
