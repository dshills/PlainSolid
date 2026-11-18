//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestTangentEllipseComponentFuseRecipe(t *testing.T) {
	ctx := context.Background()
	stock, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	tool := quarticRoofSolid(t, 1.0/16, 1.0/16)
	options := SplineShellRepeatedEllipseOptions{Faces: [2]int{4, 1}, Hint: geom.GraphEllipseHint{
		Center:        [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)},
		RadiusSquared: [2]*big.Rat{big.NewRat(1, 16), big.NewRat(1, 16)},
	}}
	var data bytes.Buffer
	if e := WriteTangentEllipseComponentFuseJSON(ctx, &data, stock, tool, options); e != nil {
		t.Fatal(e)
	}
	read := func(raw []byte, limits JSONLimits) (Solid, SplineShellCombineHistory, SplineShellRepeatedEllipse, error) {
		return ReadTangentEllipseComponentFuseJSON(ctx, bytes.NewReader(raw), limits, SplineShellBooleanOptions{}, geom.GraphPointContactOptions{})
	}
	q, h, p, e := read(data.Bytes(), JSONLimits{})
	if e != nil || !p.Contact.Proven || p.SeparatedPairs != 35 || len(q.Topology().Shells) != 2 || h.FacesB[0][0] != 6 {
		t.Fatal("source-bound component reload", e)
	}
	verifiesNoResult := func(raw []byte, limits JSONLimits) {
		t.Helper()
		q, h, p, e := read(raw, limits)
		if e == nil || len(q.faces) != 0 || h.Method != "" || p.Contact.Proven || p.QuadraticContact.Proven {
			t.Fatal("invalid recipe produced a result", e)
		}
	}
	verifiesNoResult(data.Bytes(), JSONLimits{MaxBytes: int64(data.Len() - 1)})
	verifiesNoResult(append(append([]byte(nil), data.Bytes()...), []byte("{}")...), JSONLimits{})
	var recipe tangentEllipseFuseRecipe
	if e := json.Unmarshal(data.Bytes(), &recipe); e != nil {
		t.Fatal(e)
	}
	malformed := recipe
	malformed.AxisHint = append([]string(nil), recipe.AxisHint...)
	malformed.AxisHint[2] = "1/15"
	raw, e := json.Marshal(malformed)
	if e != nil {
		t.Fatal(e)
	}
	verifiesNoResult(raw, JSONLimits{})
	// The independently valid modified stock has a strict gap. Its new
	// geometry cannot inherit the original material-contact certificate.
	separated, _ := makeOpenSplineBox(t, [3]float64{0, 0, .125}, [3]float64{1, 1, 1.125}, 0)
	var source bytes.Buffer
	if e := WriteJSON(&source, separated); e != nil {
		t.Fatal(e)
	}
	recipe.Sources[0] = source.Bytes()
	raw, e = json.Marshal(recipe)
	if e != nil {
		t.Fatal(e)
	}
	verifiesNoResult(raw, JSONLimits{})
}
