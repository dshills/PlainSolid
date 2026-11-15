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
	compareOCCTExteriorEllipse(t, "PLAINSOLID_OCCT_REPEATED_ELLIPSE", "repeated-ellipse-tangent-shells")
}

func compareOCCTExteriorEllipse(t *testing.T, env, mode string) {
	t.Helper()
	oracle := os.Getenv(env)
	if oracle == "" {
		t.Skip("set " + env + " to the independent author")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "ellipse-contact.json")
	if logs, e := exec.CommandContext(ctx, oracle, path, mode).CombinedOutput(); e != nil {
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
	compareOCCTContainedEllipse(t, "PLAINSOLID_OCCT_CONTAINED_ELLIPSE", "contained-ellipse-tangent-shells", 20227.0/20480)
}

func TestOCCTRotatedTangentSplineShellEllipse(t *testing.T) {
	compareOCCTExteriorEllipse(t, "PLAINSOLID_OCCT_ROTATED_TANGENCY", "rotated-exterior-tangent-shells")
	compareOCCTContainedEllipse(t, "PLAINSOLID_OCCT_ROTATED_TANGENCY", "rotated-contained-tangent-shells", 20207.0/20480)
}

func compareOCCTContainedEllipse(t *testing.T, env, mode string, volume float64) {
	t.Helper()
	oracle := os.Getenv(env)
	if oracle == "" {
		t.Skip("set " + env + " to the independent author")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "contained-ellipse-contact.json")
	if logs, e := exec.CommandContext(ctx, oracle, path, mode).CombinedOutput(); e != nil {
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
	}{{"contained-tangent-common", volume}, {"contained-tangent-fuse", 25.0 / 8}} {
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

func TestRotatedTangentSplineShellEllipse(t *testing.T) {
	ctx := context.Background()
	// q = (u-.5)² + (v-.5)² + (u-.5)(v-.5)/2 - 1/16.
	// The support is -9q²/16 with independently built adjacent walls.
	hint := geom.GraphQuadraticEllipseHint{Coefficients: [6]*big.Rat{
		big.NewRat(9, 16), big.NewRat(-5, 4), big.NewRat(1, 1),
		big.NewRat(-5, 4), big.NewRat(1, 1), big.NewRat(1, 2),
	}}
	powers := [6][2]int64{{0, 0}, {1, 0}, {2, 0}, {0, 1}, {0, 2}, {1, 1}}
	inner := quarticHeightRoofSolid(t, func(i, j int) float64 {
		height := new(big.Rat)
		for a, uv := range powers {
			for b, xy := range powers {
				u, v := uv[0]+xy[0], uv[1]+xy[1]
				if u > int64(i) || v > int64(j) {
					continue
				}
				weight := new(big.Rat).SetFrac(
					new(big.Int).Mul(new(big.Int).Binomial(int64(i), u), new(big.Int).Binomial(int64(j), v)),
					new(big.Int).Mul(new(big.Int).Binomial(4, u), new(big.Int).Binomial(4, v)))
				height.Add(height, weight.Mul(weight, new(big.Rat).Mul(hint.Coefficients[a], hint.Coefficients[b])))
			}
		}
		value, exact := height.Mul(height, big.NewRat(9, 16)).Float64()
		if !exact {
			t.Fatal("independent source fixture must have exact controls")
		}
		return value
	})
	exterior, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	enclosure, _ := makeOpenSplineBox(t, [3]float64{-.125, -.125, -2}, [3]float64{1.125, 1.125, 0}, 0)
	for _, test := range []struct {
		source          [2]Solid
		faces           [2]int
		internal, infer bool
		outer           int
	}{
		{[2]Solid{exterior, inner}, [2]int{4, 1}, false, true, -1},
		{[2]Solid{inner, exterior}, [2]int{1, 4}, false, false, -1},
		{[2]Solid{enclosure, inner}, [2]int{5, 1}, true, false, 0},
		{[2]Solid{inner, enclosure}, [2]int{1, 5}, true, true, 1},
	} {
		o := SplineShellRepeatedEllipseOptions{Faces: test.faces, QuadraticHint: hint}
		if test.infer {
			o.QuadraticHint = geom.GraphQuadraticEllipseHint{}
		}
		combine := CombineExternallyTangentSplineShellEllipse
		if test.internal {
			combine = CombineInternallyTangentSplineShellEllipse
		}
		result, _, p, e := combine(ctx, test.source[0], test.source[1], parametric.Intersection, o)
		c := p.QuadraticContact
		if e != nil || p.Contact.Proven || !c.Proven || c.Ellipse.Coefficients[5].Cmp(big.NewRat(1, 2)) != 0 ||
			c.Center[0].Cmp(big.NewRat(1, 2)) != 0 || c.Center[1].Cmp(big.NewRat(1, 2)) != 0 ||
			c.ProjectionRadiusSquared[0].Cmp(big.NewRat(1, 15)) != 0 || c.ProjectionRadiusSquared[1].Cmp(big.NewRat(1, 15)) != 0 ||
			p.SeparatedPairs != 35 || p.Enclosing != test.outer {
			t.Fatal("complete mixed-term original chart proof", p, e)
		}
		if test.internal {
			if len(result.faces) != 6 || result.faces[1].Surface != inner.faces[1].Surface {
				t.Fatal("Common must retain the complete original inner material")
			}
		} else if len(result.faces) != 0 {
			t.Fatal("closed exterior contact has no common volume")
		}
	}
	wrong := hint
	wrong.Coefficients[5] = big.NewRat(0, 1)
	o := SplineShellRepeatedEllipseOptions{Faces: [2]int{4, 1}, QuadraticHint: wrong}
	if r, _, p, e := CombineExternallyTangentSplineShellEllipse(ctx, exterior, inner, parametric.Intersection, o); e == nil || p.QuadraticContact.Proven || len(r.faces) != 0 {
		t.Fatal("an axis-aligned substitute must not qualify an original rotated conic", e)
	}
	o.Hint.Center[0] = big.NewRat(1, 2)
	if _, _, _, e := CombineExternallyTangentSplineShellEllipse(ctx, exterior, inner, parametric.Intersection, o); !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("ambiguous explicit hints must be rejected", e)
	}
}
