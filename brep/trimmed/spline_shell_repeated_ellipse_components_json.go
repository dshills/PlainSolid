//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"

	"plainsolid/geom"
)

const (
	nativeTangentEllipseFuse   = "trimmed-closed-ellipse-component-fuse"
	nativeTangentEllipseCavity = "trimmed-closed-ellipse-cavity-cut"
)

type tangentEllipseBoundaryOperation func(context.Context, Solid, Solid, SplineShellRepeatedEllipseOptions) (Solid, SplineShellCombineHistory, SplineShellRepeatedEllipse, error)

type tangentEllipseFuseRecipe struct {
	Version    int               `json:"version"`
	Kind       string            `json:"kind"`
	Sources    []json.RawMessage `json:"sources"`
	Faces      []int             `json:"faces"`
	Projection []int             `json:"projection"`
	AxisHint   []string          `json:"axisHint,omitempty"`
	ConicHint  []string          `json:"conicHint,omitempty"`
}

// WriteTangentEllipseComponentFuseJSON re-proves the operands and writes
// their original boundaries and selected chart data. The fused boundary and
// its material classification are reconstructed by the reader.
func WriteTangentEllipseComponentFuseJSON(ctx context.Context, w io.Writer, a, b Solid, o SplineShellRepeatedEllipseOptions) error {
	return writeTangentEllipseBoundaryJSON(ctx, w, a, b, o, nativeTangentEllipseFuse, FuseExternallyTangentSplineShellEllipseComponents)
}

// WriteTangentEllipseCavityCutJSON re-proves the nested original shells and
// writes the source recipe for the outward stock and inward cavity boundary.
func WriteTangentEllipseCavityCutJSON(ctx context.Context, w io.Writer, stock, cutter Solid, o SplineShellRepeatedEllipseOptions) error {
	return writeTangentEllipseBoundaryJSON(ctx, w, stock, cutter, o, nativeTangentEllipseCavity, CutInternallyTangentSplineShellEllipseCavity)
}

func writeTangentEllipseBoundaryJSON(ctx context.Context, w io.Writer, a, b Solid, o SplineShellRepeatedEllipseOptions,
	kind string, operation tangentEllipseBoundaryOperation) error {
	if w == nil {
		return geom.ErrInvalid
	}
	_, _, proof, e := operation(ctx, a, b, o)
	if e != nil {
		return e
	}
	n := tangentEllipseFuseRecipe{Version: 1, Kind: kind, Faces: proof.Faces[:], Projection: proof.Projection[:]}
	for _, s := range [2]Solid{a, b} {
		if e := ctx.Err(); e != nil {
			return e
		}
		var source bytes.Buffer
		if e := WriteJSON(&source, s); e != nil {
			return e
		}
		n.Sources = append(n.Sources, source.Bytes())
	}
	if o.Hint != (geom.GraphEllipseHint{}) {
		for _, x := range [4]*big.Rat{o.Hint.Center[0], o.Hint.Center[1], o.Hint.RadiusSquared[0], o.Hint.RadiusSquared[1]} {
			n.AxisHint = append(n.AxisHint, x.RatString())
		}
	}
	if o.QuadraticHint != (geom.GraphQuadraticEllipseHint{}) {
		for _, x := range o.QuadraticHint.Coefficients {
			n.ConicHint = append(n.ConicHint, x.RatString())
		}
	}
	for _, hint := range [][]string{n.AxisHint, n.ConicHint} {
		for _, x := range hint {
			if _, e := parseAlgebraicSeamRational(x); e != nil {
				return e
			}
		}
	}
	raw, e := json.Marshal(n)
	if e != nil {
		return e
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	raw = append(raw, '\n')
	count, e := w.Write(raw)
	if e == nil && count != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}

// ReadTangentEllipseComponentFuseJSON independently recovers the two native
// shells and proves complete exterior contact and distinct material interiors.
// MaxBytes bounds the complete recipe. The remaining JSON limits apply to
// each original boundary.
func ReadTangentEllipseComponentFuseJSON(ctx context.Context, r io.Reader, limits JSONLimits, shells SplineShellBooleanOptions,
	contact geom.GraphPointContactOptions) (Solid, SplineShellCombineHistory, SplineShellRepeatedEllipse, error) {
	return readTangentEllipseBoundaryJSON(ctx, r, limits, shells, contact, nativeTangentEllipseFuse, FuseExternallyTangentSplineShellEllipseComponents)
}

// ReadTangentEllipseCavityCutJSON independently proves containment and exact
// closed contact before constructing the outward stock and inward shell.
func ReadTangentEllipseCavityCutJSON(ctx context.Context, r io.Reader, limits JSONLimits, shells SplineShellBooleanOptions,
	contact geom.GraphPointContactOptions) (Solid, SplineShellCombineHistory, SplineShellRepeatedEllipse, error) {
	return readTangentEllipseBoundaryJSON(ctx, r, limits, shells, contact, nativeTangentEllipseCavity, CutInternallyTangentSplineShellEllipseCavity)
}

func readTangentEllipseBoundaryJSON(ctx context.Context, r io.Reader, limits JSONLimits, shells SplineShellBooleanOptions,
	contact geom.GraphPointContactOptions, kind string, operation tangentEllipseBoundaryOperation) (Solid, SplineShellCombineHistory, SplineShellRepeatedEllipse, error) {
	var solid Solid
	var history SplineShellCombineHistory
	var proof SplineShellRepeatedEllipse
	if ctx == nil || r == nil || limits.MaxBytes < 0 || limits.MaxBytes > 1<<30 || limits.MaxControlPoints < 0 || limits.MaxEntities < 0 {
		return solid, history, proof, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return solid, history, proof, e
	}
	if limits.MaxBytes == 0 {
		limits.MaxBytes = 32 << 20
	}
	raw, e := io.ReadAll(io.LimitReader(r, limits.MaxBytes+1))
	if e != nil {
		return solid, history, proof, e
	}
	if int64(len(raw)) > limits.MaxBytes {
		return solid, history, proof, geom.ErrConvergence
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var n tangentEllipseFuseRecipe
	if e := d.Decode(&n); e != nil {
		return solid, history, proof, e
	}
	if e := d.Decode(new(any)); e != io.EOF || n.Version != 1 || n.Kind != kind || len(n.Sources) != 2 ||
		len(n.Faces) != 2 || len(n.Projection) != 2 || len(n.AxisHint) != 0 && len(n.AxisHint) != 4 ||
		len(n.ConicHint) != 0 && len(n.ConicHint) != 6 || len(n.AxisHint) != 0 && len(n.ConicHint) != 0 {
		return solid, history, proof, geom.ErrInvalid
	}
	o := SplineShellRepeatedEllipseOptions{Faces: [2]int(n.Faces), Projection: [2]int(n.Projection), Shells: shells, Contact: contact}
	for i, raw := range n.AxisHint {
		x, e := parseAlgebraicSeamRational(raw)
		if e != nil {
			return solid, history, proof, e
		}
		if i < 2 {
			o.Hint.Center[i] = x
		} else {
			o.Hint.RadiusSquared[i-2] = x
		}
	}
	for i, raw := range n.ConicHint {
		x, e := parseAlgebraicSeamRational(raw)
		if e != nil {
			return solid, history, proof, e
		}
		o.QuadraticHint.Coefficients[i] = x
	}
	var originals [2]Solid
	for i, raw := range n.Sources {
		s, e := ReadJSONContext(ctx, bytes.NewReader(raw), limits)
		if e != nil {
			return solid, history, proof, e
		}
		originals[i] = s
	}
	return operation(ctx, originals[0], originals[1], o)
}
