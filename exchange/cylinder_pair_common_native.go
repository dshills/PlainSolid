package exchange

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"unicode/utf8"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// CylinderPairCommonRecipe retains both original raw cylinder axes and finite
// parameter windows. Axes are NOT normalized and Min/Max remain raw parameters.
// LengthUnit is metres per model unit; no coordinate conversion is performed.
// The native dispatcher accepts its documented bounded construction families,
// not every arrangement supported by the exact point/line material query.
type CylinderPairCommonRecipe struct {
	Name                  string
	A, B                  geom.RawCylinder
	Tolerance, LengthUnit float64
}
type cylinderPairCommonEnvelope struct {
	Version                                 int
	Kind                                    string
	Recipe                                  CylinderPairCommonRecipe
	Construction                            parametric.CylinderCommonConstruction
	SideFaces, LowerCapFaces, UpperCapFaces [2][]int
	Boundary                                json.RawMessage
}

func cylinderPairCommonPayload(ctx context.Context, r CylinderPairCommonRecipe) (cylinderPairCommonEnvelope, parametric.FiniteCylinderPairCommonResult, error) {
	var empty parametric.FiniteCylinderPairCommonResult
	if ctx == nil || !utf8.ValidString(r.Name) || len(r.Name) > 4096 || r.LengthUnit <= 0 || math.IsNaN(r.LengthUnit) || math.IsInf(r.LengthUnit, 0) {
		return cylinderPairCommonEnvelope{}, empty, geom.ErrDomain
	}
	if e := ctx.Err(); e != nil {
		return cylinderPairCommonEnvelope{}, empty, e
	}
	solid, e := parametric.FiniteCylinderPairCommon(ctx, r.A, r.B, r.Tolerance)
	if e != nil {
		return cylinderPairCommonEnvelope{}, empty, e
	}
	var native bytes.Buffer
	if e = parametric.WriteJSON(&native, solid.Solid); e != nil {
		return cylinderPairCommonEnvelope{}, empty, e
	}
	envelope := cylinderPairCommonEnvelope{Version: 1, Kind: "analytic-finite-cylinder-pair-common", Recipe: r, Construction: solid.Construction, SideFaces: solid.SideFaces, LowerCapFaces: solid.LowerCapFaces, UpperCapFaces: solid.UpperCapFaces, Boundary: json.RawMessage(bytes.TrimSpace(native.Bytes()))}
	return envelope, solid, nil
}

// WriteCylinderPairCommonRecipeJSON stages a canonical recipe, construction
// discriminator, complete input-ordered face ancestry and native boundary.
// It publishes nothing on construction/encoding failure. Writer failures can
// of course occur after a partial underlying I/O write.
func WriteCylinderPairCommonRecipeJSON(ctx context.Context, w io.Writer, r CylinderPairCommonRecipe) error {
	if w == nil {
		return geom.ErrDomain
	}
	envelope, _, e := cylinderPairCommonPayload(ctx, r)
	if e != nil {
		return e
	}
	encoded, e := json.Marshal(envelope)
	if e != nil {
		return e
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	encoded = append(encoded, '\n')
	n, e := w.Write(encoded)
	if e == nil && n != len(encoded) {
		return io.ErrShortWrite
	}
	return e
}

// ReadCylinderPairCommonRecipeJSON rebuilds from the original raw operands and
// requires exact canonical equality of the entire envelope, including ancestry
// and native geometry. Whitespace is ignored; duplicate/unknown fields, changed
// number spellings, extra documents and inconsistent payloads fail closed.
// This proves recipe/boundary consistency, not authenticity of an external
// source. maxBytes defaults to 1 MiB and is capped at 16 MiB. No STEP conversion is
// involved and unsupported native arrangements remain explicit errors.
func ReadCylinderPairCommonRecipeJSON(ctx context.Context, r io.Reader, maxBytes int64) (CylinderPairCommonRecipe, parametric.FiniteCylinderPairCommonResult, error) {
	fail := func(e error) (CylinderPairCommonRecipe, parametric.FiniteCylinderPairCommonResult, error) {
		if e == nil {
			e = fmt.Errorf("exchange: invalid or noncanonical finite cylinder-common recipe")
		}
		return CylinderPairCommonRecipe{}, parametric.FiniteCylinderPairCommonResult{}, e
	}
	if ctx == nil || r == nil || maxBytes < 0 || maxBytes > 16<<20 {
		return fail(nil)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	if maxBytes == 0 {
		maxBytes = 1 << 20
	}
	encoded, e := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if e != nil {
		return fail(e)
	}
	if int64(len(encoded)) > maxBytes {
		return fail(nil)
	}
	var envelope cylinderPairCommonEnvelope
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(&envelope); e != nil {
		return fail(nil)
	}
	if envelope.Version != 1 || envelope.Kind != "analytic-finite-cylinder-pair-common" {
		return fail(nil)
	}
	expected, solid, e := cylinderPairCommonPayload(ctx, envelope.Recipe)
	if e != nil {
		return fail(e)
	}
	canonical, e := json.Marshal(expected)
	if e != nil {
		return fail(e)
	}
	var compact bytes.Buffer
	if e = json.Compact(&compact, encoded); e != nil || !bytes.Equal(compact.Bytes(), canonical) {
		return fail(nil)
	}
	if e = ctx.Err(); e != nil {
		return fail(e)
	}
	return envelope.Recipe, solid, nil
}
