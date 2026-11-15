package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestOCCTExternallyTangentSplineShellEllipse(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_REPEATED_ELLIPSE")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_REPEATED_ELLIPSE to the independent author")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "ellipse-contact.json")
	if logs, e := exec.CommandContext(ctx, oracle, path, "repeated-ellipse-tangent-shells").CombinedOutput(); e != nil {
		t.Fatalf("independent OCCT double ellipse: %v %s", e, logs)
	}
	data, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var p struct {
		Name         string
		Area, Volume float64
		Centroid     [3]float64
	}
	if e := json.Unmarshal(data, &p); e != nil {
		t.Fatal(e)
	}
	if p.Name != "repeated-ellipse-stock-cut" || math.Abs(p.Volume-1) > 1e-10 || math.Abs(p.Area-6) > 1e-10 {
		t.Fatal("independent regularized material result", p)
	}
	for _, x := range p.Centroid {
		if math.Abs(x-.5) > 1e-10 {
			t.Fatal("independent retained material centroid", p)
		}
	}
}

func TestExternallyTangentSplineShellEllipse(t *testing.T) {
	ctx := context.Background()
	stock, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	tool := quarticRoofSolid(t, 1.0/16, 1.0/16)
	for _, order := range [][2]int{{0, 1}, {1, 0}} {
		shells := [2]Solid{stock, tool}
		faces := [2]int{4, 1}
		operands := [2]Solid{shells[order[0]], shells[order[1]]}
		o := SplineShellRepeatedEllipseOptions{Faces: [2]int{faces[order[0]], faces[order[1]]},
			Hint: geom.GraphEllipseHint{Center: [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, RadiusSquared: [2]*big.Rat{big.NewRat(1, 16), big.NewRat(1, 16)}}}
		if order[0] == 1 {
			o.Hint = geom.GraphEllipseHint{}
		}
		for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Difference} {
			x, h, proof, e := CombineExternallyTangentSplineShellEllipse(ctx, operands[0], operands[1], op, o)
			wantSide := 1 - 2*order[0]
			if e != nil || h.Method != "repeated-ellipse-tangency" || proof.Faces != o.Faces || proof.SeparatedPairs != 35 ||
				!proof.Contact.Proven || proof.Contact.InsideSide != wantSide || proof.Contact.OutsideSide != wantSide {
				t.Fatal("proved closed curve and every original face pair", order, op, proof, e)
			}
			if op == parametric.Intersection {
				if len(x.faces) != 0 {
					t.Fatal("boundary-only common has no volume")
				}
			} else {
				var original, cut bytes.Buffer
				if e := WriteJSON(&original, operands[0]); e != nil {
					t.Fatal(e)
				}
				if e := WriteJSON(&cut, x); e != nil || !bytes.Equal(original.Bytes(), cut.Bytes()) || h.EdgesA[0][0] != 0 || len(h.EdgesB[0]) != 0 {
					t.Fatal("Cut lost original material and ancestry", e)
				}
			}
			// (3/4,1/2,0) lies on the independent exact material rim.
			p, e := exactFaceSupportPatch(operands[1].faces[o.Faces[1]], geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			uv := [2]*big.Rat{big.NewRat(3, 4), big.NewRat(1, 2)}
			var mapped [2]*big.Rat
			for i, row := range proof.MapAtoB {
				mapped[i] = new(big.Rat).Set(row[2])
				for j, v := range uv {
					mapped[i].Add(mapped[i], new(big.Rat).Mul(v, row[j]))
				}
			}
			q, e := p.EvaluateRational(ctx, mapped[0], mapped[1], geom.SurfacePatchEvaluationOptions{})
			if e != nil || q.Point[2].Sign() != 0 || q.Point[0].Cmp(uv[0]) != 0 || q.Point[1].Cmp(uv[1]) != 0 {
				t.Fatal("contact is on both original support charts", q, e)
			}
		}
		if x, _, p, e := CombineExternallyTangentSplineShellEllipse(ctx, operands[0], operands[1], parametric.Union, o); !errors.Is(e, geom.ErrConvergence) || len(x.faces) != 0 || p.Contact.Proven {
			t.Fatal("singular contacting union must not publish a native shell", e)
		}
	}
	wrong := SplineShellRepeatedEllipseOptions{Faces: [2]int{4, 1}, Hint: geom.GraphEllipseHint{
		Center: [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, RadiusSquared: [2]*big.Rat{big.NewRat(1, 16), big.NewRat(1, 25)}}}
	if x, _, p, e := CombineExternallyTangentSplineShellEllipse(ctx, stock, tool, parametric.Intersection, wrong); e == nil || len(x.faces) != 0 || p.Contact.Proven {
		t.Fatal("incorrect ellipse must not yield material proof", e)
	}
}
