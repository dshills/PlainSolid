package parametric

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

// SphericalLensDisplayJSONOptions bounds the read and rebuild. MaxBytes
// defaults to 64 KiB and may not exceed 1 MiB. Arithmetic applies to the
// source rim and display approximation proofs performed on reload.
type SphericalLensDisplayJSONOptions struct {
	MaxBytes   int64
	Arithmetic geom.SurfacePatchOptions
}

type sphericalLensDisplayRecipe struct {
	Balls                                       [2]Ball
	Distance, SourceTolerance, DisplayTolerance float64
	PrecisionBits                               int `json:",omitempty"`
}

type sphericalLensDisplayNative struct {
	Version            int
	Kind, SourceSHA256 string
	OriginalCenter     [3]string
	RadiusSquared      string
	Latitude           [2][2]string
	ModelDeviation     string
	Source             sphericalLensDisplayRecipe
}

// WriteJSON stores the original ball recipe and exact display certificate.
// It writes no rounded cap controls, tessellation, or mesh reference. The
// reader reconstructs all source geometry and independently re-proves the
// reported evidence using the caller's arithmetic limits.
func (r CertifiedSphericalLensDilation) WriteJSON(ctx context.Context, w io.Writer, o SphericalLensDisplayJSONOptions) error {
	if ctx == nil || w == nil || r.proof.ModelDeviation == nil {
		return geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	limit, err := sphericalLensDisplayJSONLimit(o.MaxBytes)
	if err != nil {
		return err
	}
	balls, _, err := r.source.sphereOffsetOwners(Intersection, false)
	if err != nil {
		return err
	}
	recipe := sphericalLensDisplayRecipe{balls, r.distance, r.source.solid.Tolerance(), r.display.Tolerance(), r.bits}
	record, err := sphericalLensDisplayNativeRecord(recipe, r.proof)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if int64(len(raw)) > limit {
		return ErrBudget
	}
	if _, err := ReadCertifiedSphericalLensDilationJSON(ctx, bytes.NewReader(raw), o); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	n, err := w.Write(raw)
	if err == nil && n != len(raw) {
		err = io.ErrShortWrite
	}
	return err
}

// ReadCertifiedSphericalLensDilationJSON rebuilds the exact sphere material,
// recertifies the model deviation, and reconstructs the display shell. Source
// or witness edits cannot inherit the recorded certificate.
func ReadCertifiedSphericalLensDilationJSON(ctx context.Context, r io.Reader, o SphericalLensDisplayJSONOptions) (CertifiedSphericalLensDilation, error) {
	var zero CertifiedSphericalLensDilation
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	limit, err := sphericalLensDisplayJSONLimit(o.MaxBytes)
	if err != nil {
		return zero, err
	}
	raw, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return zero, err
	}
	if int64(len(raw)) > limit {
		return zero, ErrBudget
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var record sphericalLensDisplayNative
	if err := dec.Decode(&record); err != nil {
		return zero, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF || record.Version != 1 || record.Kind != "certified-spherical-lens-dilation" {
		return zero, geom.ErrInvalid
	}
	s := record.Source
	digest, err := sphericalLensDisplaySourceDigest(s)
	if err != nil || record.SourceSHA256 != digest {
		return zero, fmt.Errorf("%w: spherical lens display source digest", geom.ErrInvalid)
	}
	original, err := NewSpherePairSet(ctx, s.Balls[0], s.Balls[1], Intersection, s.SourceTolerance)
	if err != nil {
		return zero, err
	}
	result, err := original.ProveSphericalLensDilationDisplay(ctx, s.Distance, s.DisplayTolerance, geom.SphereLensMeasureOptions{PrecisionBits: s.PrecisionBits, Arithmetic: o.Arithmetic})
	if err != nil {
		return zero, err
	}
	derived, err := sphericalLensDisplayNativeRecord(s, result.proof)
	if err != nil {
		return zero, err
	}
	if derived != record {
		return zero, fmt.Errorf("%w: spherical lens display witness changed", geom.ErrInvalid)
	}
	return result, nil
}

func sphericalLensDisplayJSONLimit(n int64) (int64, error) {
	if n < 0 || n > 1<<20 {
		return 0, geom.ErrInvalid
	}
	if n == 0 {
		n = 1 << 16
	}
	return n, nil
}

func sphericalLensDisplaySourceDigest(recipe sphericalLensDisplayRecipe) (string, error) {
	raw, err := json.Marshal(recipe)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func sphericalLensDisplayNativeRecord(recipe sphericalLensDisplayRecipe, proof SphericalLensDisplayProof) (sphericalLensDisplayNative, error) {
	digest, err := sphericalLensDisplaySourceDigest(recipe)
	if err != nil {
		return sphericalLensDisplayNative{}, err
	}
	r := sphericalLensDisplayNative{Version: 1, Kind: "certified-spherical-lens-dilation", SourceSHA256: digest, Source: recipe,
		RadiusSquared: proof.OriginalRadiusSquared.RatString(), ModelDeviation: proof.ModelDeviation.RatString()}
	for i, q := range proof.OriginalCenter {
		r.OriginalCenter[i] = q.RatString()
	}
	for i, v := range proof.OriginalLatitudes {
		r.Latitude[i] = [2]string{v.Lower.RatString(), v.Upper.RatString()}
	}
	return r, nil
}
