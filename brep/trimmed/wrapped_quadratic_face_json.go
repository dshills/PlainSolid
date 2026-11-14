package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"plainsolid/geom"
	"strings"
)

// WrappedQuadraticFaceJSONOptions bounds standalone open-face reconstruction.
// MaxBytes defaults2MiB, max16MiB. MaxControlPoints defaults/max4096; surface
// degrees are capped16 and coefficient bits65536. Face supplies proof limits,
// never serialized budgets.
type WrappedQuadraticFaceJSONOptions struct {
	MaxBytes         int64
	MaxControlPoints int
	Face             WrappedQuadraticFaceOptions
}
type wrappedQuadraticFaceRecipe struct {
	Version         int                     `json:"version"`
	Kind            string                  `json:"kind"`
	Surface         geom.BSplineSurfaceData `json:"surface"`
	WrapU, WrapV    bool
	ContinuityOrder int
	ProjectionAxes  []int
	Reversed        bool
	Loops           [][][][]string `json:"loops"`
}

func wrappedQuadraticJSONOptions(o WrappedQuadraticFaceJSONOptions) (WrappedQuadraticFaceJSONOptions, error) {
	if o.MaxBytes == 0 {
		o.MaxBytes = 2 << 20
	}
	if o.MaxControlPoints == 0 {
		o.MaxControlPoints = 4096
	}
	if o.Face.MaxEdges == 0 {
		o.Face.MaxEdges = 32
	}
	if o.Face.MaxPieces == 0 {
		o.Face.MaxPieces = 4096
	}
	if o.Face.MaxOperations == 0 {
		o.Face.MaxOperations = 8000000
	}
	if o.Face.MaxCoefficientBits == 0 {
		o.Face.MaxCoefficientBits = 16384
	}
	if o.MaxBytes < 1 || o.MaxBytes > 16<<20 || o.MaxControlPoints < 4 || o.MaxControlPoints > 4096 || o.Face.MaxEdges < 3 || o.Face.MaxEdges > 32 || o.Face.MaxPieces < 1 || o.Face.MaxPieces > 65536 || o.Face.MaxOperations < 1 || o.Face.MaxCoefficientBits < 64 || o.Face.MaxCoefficientBits > 65536 {
		return o, geom.ErrInvalid
	}
	return o, nil
}

// WriteWrappedQuadraticFaceJSON emits only source support and exact trim data.
// It reconstructs/reproves the serialized recipe under the caller's limits
// before writing any bytes. I/O failures can still leave a partial stream.
func WriteWrappedQuadraticFaceJSON(ctx context.Context, dst io.Writer, face *WrappedQuadraticFace, o WrappedQuadraticFaceJSONOptions) error {
	if ctx == nil || dst == nil || face == nil || len(face.loops) == 0 {
		return geom.ErrInvalid
	}
	var e error
	o, e = wrappedQuadraticJSONOptions(o)
	if e != nil {
		return e
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	w, ok := face.native.Face.Surface.(*geom.WrappedSurfacePatch)
	if !ok {
		return geom.ErrInvalid
	}
	if w.FundamentalPatch() == nil {
		return geom.ErrInvalid
	}
	cu, cv := w.FundamentalPatch().ControlShape()
	if cu < 1 || cv < 1 || cu > o.MaxControlPoints || cv > o.MaxControlPoints || cu > o.MaxControlPoints/cv {
		return geom.ErrConvergence
	}
	source, e := w.ExportRecipe()
	if e != nil {
		return e
	}
	recipe := wrappedQuadraticFaceRecipe{Version: 1, Kind: "wrapped-quadratic-open-face", Surface: source.Surface.Data(), WrapU: source.Options.WrapU, WrapV: source.Options.WrapV, ContinuityOrder: source.Options.ContinuityOrder, ProjectionAxes: []int{face.projectionAxes[0], face.projectionAxes[1]}, Reversed: face.native.Face.Reversed, Loops: make([][][][]string, len(face.loops))}
	for li, l := range face.loops {
		for _, q := range l {
			h := make([][]string, 3)
			for i, p := range q.control {
				h[i] = make([]string, 3)
				for j, value := range p {
					if value == nil {
						return geom.ErrInvalid
					}
					h[i][j] = value.RatString()
				}
			}
			recipe.Loops[li] = append(recipe.Loops[li], h)
		}
	}
	raw, e := json.Marshal(recipe)
	if e != nil {
		return e
	}
	if int64(len(raw)) > o.MaxBytes {
		return geom.ErrConvergence
	}
	if _, e = decodeWrappedQuadraticFaceRecipe(ctx, recipe, o); e != nil {
		return e
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	n, e := dst.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}

// ReadWrappedQuadraticFaceJSON parses a bounded strict source-only recipe and
// independently revalidates normalized support data, exact seam order, loop
// topology, global lifted embedding and all continuous model edge proofs.
// Duplicate/unknown fields, trailing JSON and asserted proof fields reject.
func ReadWrappedQuadraticFaceJSON(ctx context.Context, src io.Reader, o WrappedQuadraticFaceJSONOptions) (*WrappedQuadraticFace, error) {
	if ctx == nil || src == nil {
		return nil, geom.ErrInvalid
	}
	var e error
	o, e = wrappedQuadraticJSONOptions(o)
	if e != nil {
		return nil, e
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	raw, e := io.ReadAll(io.LimitReader(src, o.MaxBytes+1))
	if e != nil {
		return nil, e
	}
	if int64(len(raw)) > o.MaxBytes {
		return nil, geom.ErrConvergence
	}
	if e := checkWrappedRecipeJSON(ctx, raw); e != nil {
		return nil, e
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var recipe wrappedQuadraticFaceRecipe
	if e = decoder.Decode(&recipe); e != nil {
		return nil, e
	}
	var trailing any
	if e = decoder.Decode(&trailing); e != io.EOF {
		return nil, geom.ErrInvalid
	}
	return decodeWrappedQuadraticFaceRecipe(ctx, recipe, o)
}

// Token validation rejects ambiguous duplicate fields before typed decoding.
func checkWrappedRecipeJSON(ctx context.Context, raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		if depth > 32 {
			return geom.ErrConvergence
		}
		t, e := d.Token()
		if e != nil {
			return e
		}
		delimiter, ok := t.(json.Delim)
		if !ok {
			return nil
		}
		switch delimiter {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				t, e := d.Token()
				if e != nil {
					return e
				}
				key, ok := t.(string)
				for _, ch := range key {
					if ch > 127 {
						return geom.ErrInvalid
					}
				}
				key = strings.ToLower(key)
				if !ok || seen[key] {
					return fmt.Errorf("trimmed: duplicate or invalid recipe field")
				}
				seen[key] = true
				if e := value(depth + 1); e != nil {
					return e
				}
			}
			t, e = d.Token()
			if e != nil || t != json.Delim('}') {
				return geom.ErrInvalid
			}
		case '[':
			for d.More() {
				if e := value(depth + 1); e != nil {
					return e
				}
			}
			t, e = d.Token()
			if e != nil || t != json.Delim(']') {
				return geom.ErrInvalid
			}
		default:
			return geom.ErrInvalid
		}
		return nil
	}
	if e := value(0); e != nil {
		return e
	}
	if _, e := d.Token(); e != io.EOF {
		return geom.ErrInvalid
	}
	return nil
}

func decodeWrappedQuadraticFaceRecipe(ctx context.Context, n wrappedQuadraticFaceRecipe, o WrappedQuadraticFaceJSONOptions) (*WrappedQuadraticFace, error) {
	return decodeWrappedQuadraticFaceRecipeMode(ctx, n, o, false)
}

func decodeWrappedQuadraticFaceRecipeMode(ctx context.Context, n wrappedQuadraticFaceRecipe, o WrappedQuadraticFaceJSONOptions, algebraic bool) (*WrappedQuadraticFace, error) {
	kind := "wrapped-quadratic-open-face"
	if algebraic {
		kind = "wrapped-algebraic-quadratic-open-face"
		if o.Face.MaxPieces > 8192 {
			return nil, geom.ErrInvalid
		}
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if n.Version != 1 || n.Kind != kind || len(n.Loops) < 1 || len(n.Loops) > 10 || len(n.ProjectionAxes) != 2 || n.ContinuityOrder < 0 || n.ContinuityOrder > 1 || !n.WrapU && !n.WrapV {
		return nil, geom.ErrInvalid
	}
	d := n.Surface
	if d.DegreeU < 1 || d.DegreeV < 1 || d.DegreeU > 16 || d.DegreeV > 16 || d.CountU < d.DegreeU+1 || d.CountV < d.DegreeV+1 || d.CountU > o.MaxControlPoints || d.CountV > o.MaxControlPoints || d.CountU > o.MaxControlPoints/d.CountV || len(d.Poles) != d.CountU*d.CountV || len(d.KnotsU) != d.CountU+d.DegreeU+1 || len(d.KnotsV) != d.CountV+d.DegreeV+1 || len(d.Weights) != 0 && len(d.Weights) != len(d.Poles) {
		return nil, geom.ErrInvalid
	}
	maxWeight := 0.0
	for _, weight := range d.Weights {
		if weight > maxWeight {
			maxWeight = weight
		}
	}
	if len(d.Weights) > 0 && maxWeight != 1 {
		return nil, geom.ErrUnrepresentable
	}
	total, used := 0, 0
	loops := make([][]ExactQuadraticUV, len(n.Loops))
	for li, loop := range n.Loops {
		if len(loop) < 3 || len(loop) > o.Face.MaxEdges-total {
			return nil, geom.ErrInvalid
		}
		total += len(loop)
		for _, encoded := range loop {
			if len(encoded) != 3 {
				return nil, geom.ErrInvalid
			}
			var h [3][3]*big.Rat
			for i, row := range encoded {
				if len(row) != 3 {
					return nil, geom.ErrInvalid
				}
				for j, text := range row {
					used++
					if used > o.Face.MaxOperations || len(text) > 2*o.Face.MaxCoefficientBits+8 {
						return nil, geom.ErrConvergence
					}
					// Reject exponent/base syntax before big.Rat parsing; a short exponent
					// must not request an unbounded integer allocation.
					slash := false
					for k, ch := range text {
						if ch >= '0' && ch <= '9' {
							continue
						}
						if ch == '-' && k == 0 {
							continue
						}
						if ch == '/' && !slash {
							slash = true
							continue
						}
						return nil, geom.ErrInvalid
					}
					q, ok := new(big.Rat).SetString(text)
					if !ok || q.RatString() != text {
						return nil, geom.ErrInvalid
					}
					if q.Num().BitLen() > o.Face.MaxCoefficientBits || q.Denom().BitLen() > o.Face.MaxCoefficientBits {
						return nil, geom.ErrConvergence
					}
					h[i][j] = q
				}
			}
			q, e := NewExactQuadraticUV(h)
			if e != nil {
				return nil, e
			}
			loops[li] = append(loops[li], q)
		}
	}
	if used >= o.Face.MaxOperations {
		return nil, geom.ErrConvergence
	}
	surface, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		return nil, e
	}
	patch, e := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{MaxPoles: o.MaxControlPoints, MaxOperations: o.Face.MaxOperations - used, MaxCoefficientBits: o.Face.MaxCoefficientBits})
	if e != nil {
		return nil, e
	}
	used += patch.ExactOperations()
	if used >= o.Face.MaxOperations {
		return nil, geom.ErrConvergence
	}
	wrapped, seam, e := geom.NewWrappedSurfacePatch(ctx, patch, geom.WrappedSurfaceOptions{WrapU: n.WrapU, WrapV: n.WrapV, ContinuityOrder: n.ContinuityOrder, MaxPoles: o.MaxControlPoints, MaxOperations: o.Face.MaxOperations - used, MaxCoefficientBits: o.Face.MaxCoefficientBits})
	if e != nil {
		return nil, e
	}
	used += seam.ExactOperations
	if used >= o.Face.MaxOperations {
		return nil, geom.ErrConvergence
	}
	options := o.Face
	options.MaxOperations -= used
	options.Reversed = n.Reversed
	if options.ProjectionAxes == ([2]int{}) {
		options.ProjectionAxes = [2]int{n.ProjectionAxes[0], n.ProjectionAxes[1]}
	}
	face, e := newWrappedQuadraticFace(ctx, wrapped, loops[0], loops[1:], options, algebraic)
	if e != nil {
		return nil, e
	}
	face.operations += used
	return face, nil
}
