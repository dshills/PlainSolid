package step

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"plainsolid/brep/parametric"
	"reflect"
)

type ellipseExtrusionRecipe struct {
	Format      string          `json:"format"`
	Version     int             `json:"version"`
	Source      string          `json:"source_step"`
	MaxDistance float64         `json:"max_distance_model_units"`
	Surface     json.RawMessage `json:"reconstructed_surface"`
}

// WriteRepairedEllipseExtrusionFaceJSON persists an explicit repair recipe and
// its reconstructed native support. It re-proves Source and rejects stale
// geometry, tolerance, or ancestry. This is not repaired STEP serialization.
func WriteRepairedEllipseExtrusionFaceJSON(w io.Writer, f RepairedEllipseExtrusionFace) error {
	if w == nil || !ellipseOwnedSourceBound(f.Source, 89) || f.Bounds.Tolerance == nil {
		return fmt.Errorf("step: invalid ellipse extrusion recipe")
	}
	raw, e := mappedPlanarSourceBytes(f.Source)
	if e != nil {
		return e
	}
	tol, exact := f.Bounds.Tolerance.Float64()
	if !exact {
		return fmt.Errorf("step: unrepresentable tolerance")
	}
	rebuilt, e := RepairRoundedEllipseExtrusionFace(bytes.NewReader(raw), Limits{}, EllipticalWireRepairOptions{MaxDistance: tol})
	if e != nil {
		return e
	}
	surface, e := parametric.EncodeSurface(f.Face.Surface)
	if e != nil {
		return e
	}
	expected, e := parametric.EncodeSurface(rebuilt.Face.Surface)
	if e != nil {
		return e
	}
	// Surface implementations are private immutable values; their codec is the
	// supported identity bridge. Everything else is copied, typed source binding.
	f.Face.Surface = nil
	rebuilt.Face.Surface = nil
	f.Source = nil
	rebuilt.Source = nil
	if !bytes.Equal(surface, expected) || !reflect.DeepEqual(f, rebuilt) {
		return fmt.Errorf("step: stale ellipse extrusion reconstruction")
	}
	b, e := json.Marshal(ellipseExtrusionRecipe{Format: "plainsolid-repaired-ellipse-extrusion-face", Version: 1, Source: string(raw), MaxDistance: tol, Surface: surface})
	if e != nil {
		return e
	}
	if len(b) > 256<<10 {
		return fmt.Errorf("step: ellipse extrusion recipe limit")
	}
	b = append(b, '\n')
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		e = io.ErrShortWrite
	}
	return e
}

// ReadRepairedEllipseExtrusionFaceJSON reconstructs atomically from the original
// complete source proof and enforced tolerance. Forged native support is refused.
func ReadRepairedEllipseExtrusionFaceJSON(r io.Reader, l Limits) (RepairedEllipseExtrusionFace, error) {
	fail := func() (RepairedEllipseExtrusionFace, error) {
		return RepairedEllipseExtrusionFace{}, fmt.Errorf("step: invalid ellipse extrusion repair recipe")
	}
	if r == nil {
		return fail()
	}
	raw, e := io.ReadAll(io.LimitReader(r, (256<<10)+1))
	if e != nil {
		return RepairedEllipseExtrusionFace{}, e
	}
	if len(raw) > 256<<10 {
		return fail()
	}
	var recipe ellipseExtrusionRecipe
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&recipe) != nil {
		return fail()
	}
	var trailing any
	if d.Decode(&trailing) != io.EOF {
		return fail()
	}
	if recipe.Format != "plainsolid-repaired-ellipse-extrusion-face" || recipe.Version != 1 {
		return fail()
	}
	f, e := RepairRoundedEllipseExtrusionFace(bytes.NewBufferString(recipe.Source), l, EllipticalWireRepairOptions{MaxDistance: recipe.MaxDistance})
	if e != nil {
		return RepairedEllipseExtrusionFace{}, e
	}
	expected, e := parametric.EncodeSurface(f.Face.Surface)
	if e != nil {
		return fail()
	}
	var a, b any
	if json.Unmarshal(recipe.Surface, &a) != nil || json.Unmarshal(expected, &b) != nil || !reflect.DeepEqual(a, b) {
		return fail()
	}
	return f, nil
}
