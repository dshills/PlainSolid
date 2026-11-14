package geom

import (
	"bytes"
	"context"
	"math/big"
	"strings"
	"testing"
)

func TestObliqueCylinderTorusCavityRecipe(t *testing.T) {
	r := big.NewRat
	ctx := context.Background()
	pose := [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {r(-4, 5), r(0, 1), r(3, 5), r(0, 1)}}
	source, e := NewObliqueCylinderTorusCavityMaterial(ctx, 3, 1, 8, -6, 6, pose, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e = WriteObliqueCylinderTorusCavityRecipe(&raw, source); e != nil {
		t.Fatal(e)
	}
	loaded, e := ReadObliqueCylinderTorusCavityRecipe(ctx, bytes.NewReader(raw.Bytes()), SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var again bytes.Buffer
	WriteObliqueCylinderTorusCavityRecipe(&again, loaded)
	if !bytes.Equal(raw.Bytes(), again.Bytes()) {
		t.Fatal("canonical")
	}
	frame, _ := loaded.TorusFrame()
	if frame[0][0].Cmp(r(3, 5)) != 0 {
		t.Fatal("rational frame")
	}
	frame[0][0].SetInt64(99)
	p := [3]*big.Rat{r(17, 5), r(0, 1), r(-16, 5)}
	l, m, e := loaded.ClassifyRational(ctx, p, SurfacePatchOptions{})
	if e != nil || l != BoreBoundary || m != TorusCylinderTorus {
		t.Fatal(l, m, e)
	}
	line, e := loaded.ClipRationalLine(ctx, [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}, [3]*big.Rat{r(3, 5), r(0, 1), r(-4, 5)}, r(-9, 1), r(9, 1), ToroidLineOptions{ParameterTolerance: 1e-9})
	if e != nil || len(line.Intervals) != 3 {
		t.Fatal(e)
	}
	tol := r(1, 1000000)
	mom, e := loaded.BoundMoments(ctx, TorusCylinderMomentOptions{VolumeTolerance: tol, SecondMomentTolerance: tol, CentroidTolerance: tol, InertiaTolerance: tol})
	if e != nil || mom.SecondCoefficients[0][2][1].Cmp(r(333, 25)) != 0 {
		t.Fatal("mass binding", e)
	}
	identity := [3][4]*big.Rat{{r(1, 1), r(0, 1), r(0, 1), r(0, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {r(0, 1), r(0, 1), r(1, 1), r(0, 1)}}
	bounds, e := loaded.BoundAffineRange(ctx, identity, 64, SurfacePatchOptions{})
	if e != nil || bounds.Maximum[2][1].Cmp(r(6, 1)) != 0 {
		t.Fatal("range binding", e)
	}
	for _, bad := range []string{strings.Replace(raw.String(), `"3/5"`, `"0.6"`, 1), strings.Replace(raw.String(), `"3/5"`, `"2/5"`, 1), strings.Replace(raw.String(), `["3","1","8","-6","6"]`, `["3","1","4","-6","6"]`, 1), strings.Replace(raw.String(), `"version":1`, `"version":2`, 1), strings.Replace(raw.String(), `"version":1`, `"version":1,"proof":true`, 1), raw.String() + `{}`, strings.Repeat(" ", obliqueCavityRecipeBytes+1)} {
		if _, e = ReadObliqueCylinderTorusCavityRecipe(ctx, strings.NewReader(bad), SurfacePatchOptions{}); e == nil {
			t.Fatal("accepted tamper", bad)
		}
	}
	if _, e = ReadObliqueCylinderTorusCavityRecipe(ctx, bytes.NewReader(raw.Bytes()), SurfacePatchOptions{MaxOperations: 1}); e == nil {
		t.Fatal("budget")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadObliqueCylinderTorusCavityRecipe(cancelled, bytes.NewReader(raw.Bytes()), SurfacePatchOptions{}); e == nil {
		t.Fatal("cancel")
	}
}
