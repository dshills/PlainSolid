package step

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"strconv"
	"strings"
	"testing"
)

func TestObliqueCavitySecondSource(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	frame := [3][4]*big.Rat{{r(9, 25), r(-4, 5), r(12, 25), r(-1, 2)}, {r(12, 25), r(3, 5), r(16, 25), r(3, 4)}, {r(-4, 5), r(0, 1), r(3, 5), r(1, 4)}}
	body, q, e := trimmed.ObliqueTorusCavity(ctx, 2, .5, 6, -4, 5, frame, 1e-9, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = trimmed.WriteJSON(&native, body); e != nil {
		t.Fatal(e)
	}
	restored, e := trimmed.ReadJSON(bytes.NewReader(native.Bytes()), trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := trimmed.PrepareObliqueTorusCavity(ctx, restored, 2, .5, 6, -4, 5, frame, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var again bytes.Buffer
	if e = trimmed.WriteJSON(&again, restored); e != nil || !bytes.Equal(native.Bytes(), again.Bytes()) {
		t.Fatal("native deterministic", e)
	}
	tol := r(1, 1000000)
	mass, e := q.BoundMoments(ctx, geom.TorusCylinderMomentOptions{VolumeTolerance: tol, SecondMomentTolerance: tol, CentroidTolerance: tol, InertiaTolerance: tol})
	if e != nil {
		t.Fatal(e)
	}
	if mass.VolumeCoefficients[0].Cmp(r(324, 1)) != 0 || mass.VolumeCoefficients[1].Cmp(r(-1, 1)) != 0 {
		t.Fatal("independent exact volume")
	}
	area, e := trimmed.BoundObliqueTorusCavityArea(ctx, restored, 2, .5, 6, -4, 5, frame, geom.TorusCylinderAreaOptions{AreaTolerance: tol})
	if e != nil {
		t.Fatal(e)
	}
	alo, _ := area.Lower.Float64()
	ahi, _ := area.Upper.Float64()
	truth := 180*math.Pi + 4*math.Pi*math.Pi
	if truth < alo-1e-10 || truth > ahi+1e-10 {
		t.Fatal("independent area")
	}
	center := [3]*big.Rat{r(-1, 2), r(3, 4), r(1, 4)}
	direction := [3]*big.Rat{r(9, 25), r(12, 25), r(-4, 5)}
	var foot [3]*big.Rat
	for i := 0; i < 3; i++ {
		foot[i] = new(big.Rat).Add(center[i], new(big.Rat).Mul(r(5, 2), direction[i]))
	}
	normal, e := p.BoundaryUnitNormalsRational(ctx, foot, geom.SurfacePatchOptions{})
	if e != nil || len(normal.OriginalFaces[0]) != 4 {
		t.Fatal("native seam normal", e)
	}
	for i := 0; i < 3; i++ {
		if normal.Units[0][i].Cmp(new(big.Rat).Neg(direction[i])) != 0 {
			t.Fatal("independent inward normal")
		}
	}
	section, e := p.PartitionRationalLine(ctx, center, direction, r(-10, 1), r(10, 1), trimmed.NativeObliqueCavityLineOptions{Line: geom.ToroidLineOptions{ParameterTolerance: 1e-9}})
	if e != nil {
		t.Fatal(e)
	}
	expected := []*big.Rat{r(-95, 16), r(-5, 2), r(-3, 2), r(3, 2), r(5, 2), r(85, 16)}
	if len(section.Boundary.Hits) != 6 {
		t.Fatal("hit count")
	}
	for i, h := range section.Boundary.Hits {
		sign, e := h.Parameter.SignPolynomial(ctx, []*big.Rat{new(big.Rat).Neg(expected[i]), r(1, 1)}, geom.ExactCurvePlaneEventOptions{})
		if e != nil || sign.Sign != 0 {
			t.Fatal("independent station", i, e)
		}
	}
	source := ObliqueCylinderTorusCavityBody{Name: "second independent source", MajorRadius: 2, MinorRadius: .5, CylinderRadius: 6, Lower: -4, Upper: 5, Frame: frame}
	var raw bytes.Buffer
	if e = WriteObliqueCylinderTorusCavity(&raw, source, .001); e != nil {
		t.Fatal(e)
	}
	loaded, e := ReadObliqueCylinderTorusCavity(bytes.NewReader(raw.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	recovered := loaded.Source()
	if recovered.MajorRadius != 2 || recovered.MinorRadius != .5 || recovered.CylinderRadius != 6 || recovered.Lower != -4 || recovered.Upper != 5 {
		t.Fatal("STEP dimensions")
	}
	for i := range frame {
		for j := range frame[i] {
			if recovered.Frame[i][j].Cmp(frame[i][j]) != 0 {
				t.Fatal("exact recovered frame")
			}
		}
	}
	var round bytes.Buffer
	if e = loaded.WriteSTEP(&round); e != nil || !bytes.Equal(raw.Bytes(), round.Bytes()) {
		t.Fatal("STEP deterministic", e)
	}
	loc, mask, e := loaded.Query().ClassifyRational(ctx, foot, geom.SurfacePatchOptions{})
	if e != nil || loc != geom.BoreBoundary || mask != geom.TorusCylinderTorus {
		t.Fatal("STEP source foot", e)
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_OBLIQUE_CAVITY_SECOND"); oracle != "" {
		path := filepath.Join(t.TempDir(), "second.step")
		if e = os.WriteFile(path, raw.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		data, e := exec.Command(oracle, path).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(data))
		}
		text := string(data)
		at := strings.LastIndex(text, "RESULT ")
		if at < 0 {
			t.Fatal(text)
		}
		fields := strings.Fields(text[at+7:])
		if len(fields) != 11 {
			t.Fatal(fields)
		}
		values := make([]float64, len(fields))
		for i, f := range fields {
			values[i], e = strconv.ParseFloat(f, 64)
			if e != nil {
				t.Fatal(e)
			}
		}
		compare := func(value float64, b [2]*big.Rat) {
			t.Helper()
			lo, _ := b[0].Float64()
			hi, _ := b[1].Float64()
			if value < lo-1e-6 || value > hi+1e-6 {
				t.Fatal("OCCT measure", value, lo, hi)
			}
		}
		compare(values[0], mass.Volume)
		compare(values[1], [2]*big.Rat{area.Lower, area.Upper})
		for i := 0; i < 3; i++ {
			compare(values[2+i], mass.Centroid[i])
		}
		for i, x := range expected {
			f, _ := x.Float64()
			if math.Abs(f-values[5+i]) > 1e-7 {
				t.Fatal("OCCT station", i)
			}
		}
	}
}
