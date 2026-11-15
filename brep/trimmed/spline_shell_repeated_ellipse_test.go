package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
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

func TestOCCTInternallyTangentSplineShellEllipse(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_CONTAINED_ELLIPSE")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_CONTAINED_ELLIPSE to the independent author")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "contained-ellipse-contact.json")
	if logs, e := exec.CommandContext(ctx, oracle, path, "contained-ellipse-tangent-shells").CombinedOutput(); e != nil {
		t.Fatalf("independent OCCT contained ellipse: %v %s", e, logs)
	}
	r, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	decoder := json.NewDecoder(r)
	for _, want := range []struct {
		name   string
		volume float64
	}{{"contained-tangent-common", 20227.0 / 20480}, {"contained-tangent-fuse", 25.0 / 8}} {
		var q struct {
			Name   string
			Volume float64
		}
		if e := decoder.Decode(&q); e != nil || q.Name != want.name || math.Abs(q.Volume-want.volume) > 1e-9 {
			t.Fatal("independent analytic volume and OCCT result", q, want, e)
		}
	}
	if e := decoder.Decode(new(any)); e != io.EOF {
		t.Fatal("unexpected reference records", e)
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

func TestInternallyTangentSplineShellEllipse(t *testing.T) {
	ctx := context.Background()
	outer, _ := makeOpenSplineBox(t, [3]float64{-.125, -.125, -2}, [3]float64{1.125, 1.125, 0}, 0)
	inner := quarticRoofSolid(t, 1.0/16, 1.0/16)
	for outerIndex, sources := range [2][2]Solid{{outer, inner}, {inner, outer}} {
		ids := [2]int{5, 1}
		if outerIndex == 1 {
			ids = [2]int{1, 5}
		}
		o := SplineShellRepeatedEllipseOptions{Faces: ids}
		for _, operation := range []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference} {
			result, h, p, e := CombineInternallyTangentSplineShellEllipse(ctx, sources[0], sources[1], operation, o)
			if operation == parametric.Difference && outerIndex == 0 {
				if !errors.Is(e, geom.ErrConvergence) || len(result.faces) != 0 || p.Contact.Proven {
					t.Fatal("internally touching cavity does not have a single manifold boundary", e)
				}
				continue
			}
			if e != nil || p.Enclosing != outerIndex || !p.Contact.Proven || p.SeparatedPairs != 35 || h.Method != "contained-repeated-ellipse-tangency" {
				t.Fatal("complete original pair proof and nesting", outerIndex, operation, p.Enclosing, p.SeparatedPairs, e)
			}
			if operation == parametric.Difference {
				if len(result.faces) != 0 {
					t.Fatal("enclosing cutter removes all inner material")
				}
				continue
			}
			owner := outerIndex
			if operation == parametric.Intersection {
				owner = 1 - outerIndex
			}
			faceSources := [2][][]int{h.FacesA, h.FacesB}
			if len(result.faces) != len(sources[owner].faces) || result.faces[0].Surface != sources[owner].faces[0].Surface || len(faceSources[owner][0]) != 1 || len(faceSources[1-owner][0]) != 0 {
				t.Fatal("regularized result must retain the identified original shell", outerIndex, operation, h.FacesA, h.FacesB)
			}
		}
	}
}
