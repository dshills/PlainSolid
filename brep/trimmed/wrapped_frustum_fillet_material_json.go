//go:build research

package trimmed

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"plainsolid/geom"
)

type filletedFrustumMaterialRecipe struct {
	source       Solid
	lower, upper float64
	cuts         []FilletedFrustumBoreCut
}
type nativeFilletedFrustumMaterial struct {
	Version int                      `json:"version"`
	Kind    string                   `json:"kind"`
	Stock   json.RawMessage          `json:"stock"`
	Lower   float64                  `json:"lowerFillet"`
	Upper   float64                  `json:"upperFillet"`
	Cuts    []FilletedFrustumBoreCut `json:"cuts"`
	Binding string                   `json:"bindingSHA256"`
}

// FilletedFrustumMaterialJSONLimits bounds bytes, ordinary native-stock parsing
// and fresh geometry proof independently. Limits are never read from the file.
type FilletedFrustumMaterialJSONLimits struct {
	MaxBytes int64
	Stock    JSONLimits
	Proof    geom.SurfacePatchOptions
}

func filletedFrustumMaterialBinding(ctx context.Context, q FilletedFrustumMaterial) (nativeFilletedFrustumMaterial, error) {
	if ctx == nil || q.recipe == nil || q.slope == nil {
		return nativeFilletedFrustumMaterial{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nativeFilletedFrustumMaterial{}, e
	}
	var stock, body bytes.Buffer
	if e := WriteJSON(&stock, q.recipe.source); e != nil {
		return nativeFilletedFrustumMaterial{}, e
	}
	if e := WriteJSON(&body, q.solid); e != nil {
		return nativeFilletedFrustumMaterial{}, e
	}
	envelope := nativeFilletedFrustumMaterial{Version: 1, Kind: "exact-filleted-frustum-material", Stock: stock.Bytes(), Lower: q.recipe.lower, Upper: q.recipe.upper, Cuts: append([]FilletedFrustumBoreCut(nil), q.recipe.cuts...)}
	// Both original occurrence IDs/inputs and resulting native entity ordering
	// are bound. The hash is a consistency check, not a digital signature.
	canonical, e := json.Marshal(struct {
		Recipe nativeFilletedFrustumMaterial
		Body   json.RawMessage
	}{envelope, body.Bytes()})
	if e != nil {
		return nativeFilletedFrustumMaterial{}, e
	}
	digest := sha256.Sum256(canonical)
	envelope.Binding = hex.EncodeToString(digest[:])
	if e := ctx.Err(); e != nil {
		return nativeFilletedFrustumMaterial{}, e
	}
	return envelope, nil
}

// WriteFilletedFrustumMaterialJSON stores only original construction inputs
// and a deterministic consistency binding to the proved native result. It does
// not serialize trusted classification caches, rounded supports or root results.
// Ordinary trimmed Solid JSON is unchanged and can still export q.Solid().
func WriteFilletedFrustumMaterialJSON(ctx context.Context, w io.Writer, q FilletedFrustumMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	envelope, e := filletedFrustumMaterialBinding(ctx, q)
	if e != nil {
		return e
	}
	raw, e := json.Marshal(envelope)
	if e != nil {
		return e
	}
	raw = append(raw, '\n')
	if e := ctx.Err(); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}

// ReadFilletedFrustumMaterialJSON strictly decodes a bounded recipe, re-proves
// the original stock, both fillets, all cuts and native edge/face ownership,
// then verifies the binding before exposing material or oblique-line queries.
// A changed valid shape with the old binding is rejected, as is invalid source
// geometry. Re-authoring a valid recipe AND its binding is not authentication
// failure; callers needing origin authentication must sign their own envelope.
func ReadFilletedFrustumMaterialJSON(ctx context.Context, r io.Reader, o FilletedFrustumMaterialJSONLimits) (FilletedFrustumMaterial, error) {
	fail := func(e error) (FilletedFrustumMaterial, error) { return FilletedFrustumMaterial{}, e }
	if ctx == nil || r == nil || o.MaxBytes < 0 || o.MaxBytes > 32<<20 {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	if o.MaxBytes == 0 {
		o.MaxBytes = 4 << 20
	}
	raw, e := io.ReadAll(io.LimitReader(r, o.MaxBytes+1))
	if e != nil {
		return fail(e)
	}
	if int64(len(raw)) > o.MaxBytes {
		return fail(geom.ErrConvergence)
	}
	if e = ctx.Err(); e != nil {
		return fail(e)
	}
	var stored nativeFilletedFrustumMaterial
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(&stored); e != nil {
		return fail(e)
	}
	var extra any
	if e = decoder.Decode(&extra); e != io.EOF {
		if e == nil {
			e = geom.ErrInvalid
		}
		return fail(e)
	}
	if stored.Version != 1 || stored.Kind != "exact-filleted-frustum-material" || len(stored.Cuts) < 1 || len(stored.Cuts) > 16 || len(stored.Binding) != 64 {
		return fail(geom.ErrInvalid)
	}
	if _, e = hex.DecodeString(stored.Binding); e != nil {
		return fail(geom.ErrInvalid)
	}
	if o.Stock.MaxBytes == 0 || o.Stock.MaxBytes > o.MaxBytes {
		o.Stock.MaxBytes = o.MaxBytes
	}
	source, e := ReadJSONContext(ctx, bytes.NewReader(stored.Stock), o.Stock)
	if e != nil {
		return fail(e)
	}
	q, e := PrepareFilletedFrustumMaterial(ctx, source, stored.Lower, stored.Upper, stored.Cuts, o.Proof)
	if e != nil {
		return fail(e)
	}
	fresh, e := filletedFrustumMaterialBinding(ctx, q)
	if e != nil {
		return fail(e)
	}
	if fresh.Binding != stored.Binding {
		return fail(fmt.Errorf("filleted frustum material: native recipe binding differs: %w", geom.ErrInvalid))
	}
	return q, nil
}
