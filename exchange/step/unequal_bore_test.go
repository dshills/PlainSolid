package step

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"plainsolid/internal/slowtest"
	"regexp"
	"strings"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestUnequalBoreSTEP(t *testing.T) {
	slowtest.Skip(t)
	for i, normal := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		for j, radii := range [][2]float64{{4, 2}, {2.25, 3.75}} {
			t.Run(fmt.Sprintf("%d/%d", i, j), func(t *testing.T) {
				frame, err := geom.FrameFromNormal(geom.Vec3{X: 16, Y: -32, Z: 48}, normal)
				if err != nil {
					t.Fatal(err)
				}
				body := UnequalBoreBody{Name: "unequal bores", Frame: frame, Dimensions: geom.Vec3{X: 20, Y: 24, Z: 18}, Crossing: geom.Vec3{X: 8.25, Y: 12.5, Z: 9}, XRadius: radii[0], ZRadius: radii[1]}
				var out bytes.Buffer
				if err := WriteUnequalBore(&out, UnequalBoreModel{Body: body, LengthUnit: .001}); err != nil {
					t.Fatal(err)
				}
				if strings.Count(out.String(), "B_SPLINE_CURVE_WITH_KNOTS") != 24 || !strings.Contains(out.String(), "GLOBAL_UNCERTAINTY_ASSIGNED_CONTEXT") {
					t.Fatal("missing paired spline curves or distance uncertainty")
				}
				m, err := ReadUnequalBore(bytes.NewReader(out.Bytes()), Limits{})
				if err != nil {
					t.Fatal(err)
				}
				var duplicate bytes.Buffer
				if err := WriteUnequalBore(&duplicate, m); err != nil || !bytes.Equal(out.Bytes(), duplicate.Bytes()) {
					t.Fatal("noncanonical reload", err)
				}

				if os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
					return
				}
				path, report := filepath.Join(t.TempDir(), "unequal.step"), filepath.Join(t.TempDir(), "mass.json")
				if err := os.WriteFile(path, out.Bytes(), 0600); err != nil {
					t.Fatal(err)
				}
				stepOracle(t, report, "step-mass-precise", path)
				raw, err := os.ReadFile(report)
				if err != nil {
					t.Fatal(err)
				}
				var oracle struct {
					Volume, Area float64
					Centroid     [3]float64
				}
				if err := json.Unmarshal(raw, &oracle); err != nil {
					t.Fatal(err)
				}
				native, err := trimmed.BoxUnequalCrossedBores(context.Background(), body.Frame, body.Dimensions, body.Crossing, body.XRadius, body.ZRadius, 1e-9)
				if err != nil {
					t.Fatal(err)
				}
				mass, err := native.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-10, MaxSubdivisions: 128})
				if err != nil {
					t.Fatal(err)
				}
				center := geom.Vec3{X: oracle.Centroid[0], Y: oracle.Centroid[1], Z: oracle.Centroid[2]}
				if math.Abs(oracle.Volume-mass.Volume) > 2e-6 || math.Abs(oracle.Area-mass.Area) > 2e-6 || center.Distance(mass.Centroid) > 1e-7 {
					t.Fatalf("OCCT readback %+v differs from native %+v", oracle, mass)
				}
			})
		}
	}
}

func TestUnequalBoreSTEPLimitsAndMalformed(t *testing.T) {
	t.Parallel()
	frame, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	body := UnequalBoreBody{Name: "unequal bores", Frame: frame, Dimensions: geom.Vec3{X: 20, Y: 24, Z: 18}, Crossing: geom.Vec3{X: 8, Y: 12, Z: 9}, XRadius: 4, ZRadius: 2}
	var out bytes.Buffer
	if err := WriteUnequalBore(&out, UnequalBoreModel{Body: body, LengthUnit: .001}); err != nil {
		t.Fatal(err)
	}
	source := out.String()
	corruptions := []string{
		strings.Replace(source, "B_SPLINE_CURVE_WITH_KNOTS('',3", "B_SPLINE_CURVE_WITH_KNOTS('',2", 1),
		strings.Replace(source, "distance_accuracy_value", "wrong uncertainty", 1),
	}
	re := regexp.MustCompile(`(?m)^#(\d+)=B_SPLINE_CURVE_WITH_KNOTS\('',3,\(#(\d+),#(\d+),`)
	poles := re.FindStringSubmatch(source)
	if len(poles) != 4 {
		t.Fatal("missing spline pole")
	}
	control := regexp.MustCompile(`(?m)^#` + poles[3] + `=CARTESIAN_POINT\('',\([^\n]+$`)
	badPole := control.ReplaceAllString(source, "#"+poles[3]+"=CARTESIAN_POINT('',(999.,0.,0.));")
	if badPole == source {
		t.Fatal("could not change a spline control")
	}
	for i, bad := range append(corruptions, badPole) {
		if _, err := ReadUnequalBore(strings.NewReader(bad), Limits{}); err == nil {
			t.Fatalf("accepted corruption %d", i)
		}
	}
	if _, err := ReadUnequalBore(bytes.NewReader(out.Bytes()), Limits{MaxFaces: 15}); err == nil {
		t.Fatal("accepted model outside caller face budget")
	}
	body.ZRadius = 3.99
	var rejected bytes.Buffer
	if err := WriteUnequalBore(&rejected, UnequalBoreModel{Body: body, LengthUnit: .001}); err == nil || rejected.Len() != 0 {
		t.Fatal("export must reject a curve beyond approximation budget before publishing", err)
	}
}

func TestUnequalBoreSplineAgreement(t *testing.T) {
	t.Parallel()
	frame, _ := geom.FrameFromNormal(geom.Vec3{X: 20.125, Y: -40.25, Z: 100.5}, geom.Vec3{Z: -1})
	result, err := trimmed.BoxUnequalCrossedBores(context.Background(), frame,
		geom.Vec3{X: 20, Y: 24, Z: 18}, geom.Vec3{X: 8.25, Y: 12.5, Z: 9}, 4.125, 2.25, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	faces, edges, vertices := result.Solid.Faces(), result.Solid.Edges(), result.Solid.Vertices()
	owners := make([][]longEdgeOwner, len(edges))
	supports := make([]longEdgeSurface, len(faces))
	for faceID, face := range faces {
		supports[faceID], err = longEdgeSurfaceOf(face.Surface)
		if err != nil {
			t.Fatal(err)
		}
		if supports[faceID].kind != "CYLINDRICAL_SURFACE" {
			continue
		}
		for _, wire := range append([][]trimmed.Use{face.Outer}, face.Holes...) {
			for _, use := range wire {
				if use.Curve != nil {
					owners[use.Edge] = append(owners[use.Edge], longEdgeOwner{faceID, use})
				}
			}
		}
	}
	check := func(poles []geom.Vec3, step int, t float64) geom.Vec3 {
		k := 3 * step
		u := 1 - t
		return poles[k].Scale(u * u * u).Add(poles[k+1].Scale(3 * u * u * t)).
			Add(poles[k+2].Scale(3 * u * t * t)).Add(poles[k+3].Scale(t * t * t))
	}
	count := 0
	for edgeID, adjacent := range owners {
		if len(adjacent) != 2 {
			continue
		}
		count++
		edge := edges[edgeID]
		splines, err := approximateUnequalBoreEdge(vertices[edge.Start], vertices[edge.End], adjacent, supports, UnequalBoreSTEPAccuracyMillimeters)
		if err != nil {
			t.Fatal(err)
		}
		spans := (len(splines.model) - 1) / 3
		for index, owner := range adjacent {
			surface := faces[owner.face].Surface
			for i := range spans {
				for _, f := range []float64{.125, .25, .5, .75, .875} {
					point := check(splines.model, i, f)
					approx := check(splines.owners[index], i, f)
					projected, err := surface.Eval(approx.X, approx.Y)
					if err != nil {
						t.Fatal(err)
					}
					tEdge := (float64(i) + f) / float64(spans)
					if owner.use.Reversed {
						tEdge = 1 - tEdge
					}
					uv, err := owner.use.Curve.EvalUV(tEdge)
					if err != nil {
						t.Fatal(err)
					}
					exact, err := surface.Eval(uv.X, uv.Y)
					if err != nil || point.Distance(exact) > 1e-8 || projected.Distance(exact) > 1e-8 ||
						projected.Distance(point) > 2e-8 {
						t.Fatalf("edge %d owner %d spline misses native intersection at %g", edgeID, index, tEdge)
					}
				}
			}
		}
	}
	if count != 8 {
		t.Fatalf("tested %d shared spline edges", count)
	}
}

func TestSkewBoreSTEP(t *testing.T) {
	slowtest.Skip(t)
	for i, placement := range []struct {
		normal geom.Vec3
		x, z   float64
		offset float64
	}{
		{geom.Vec3{Z: 1}, 5, 2, 1},
		{geom.Vec3{Y: -1}, 2, 5, -.625},
		{geom.Vec3{X: -1}, 5, 2, .3},
		{geom.Vec3{Z: -1}, 2, 5, .3},
	} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			frame, err := geom.FrameFromNormal(geom.Vec3{X: 16.125, Y: -32.75, Z: 48.5}, placement.normal)
			if err != nil {
				t.Fatal(err)
			}
			body := UnequalBoreBody{Name: "skew bores", Frame: frame, Dimensions: geom.Vec3{X: 20, Y: 24, Z: 18},
				Crossing: geom.Vec3{X: 8.25, Y: 12.5, Z: 9}, XRadius: placement.x, ZRadius: placement.z, Offset: placement.offset}
			var out bytes.Buffer
			if err := WriteUnequalBore(&out, UnequalBoreModel{Body: body, LengthUnit: .001}); err != nil {
				t.Fatal(err)
			}
			m, err := ReadUnequalBore(bytes.NewReader(out.Bytes()), Limits{})
			if err != nil || m.Body.Offset == 0 || m.Body.XRadius <= m.Body.ZRadius {
				t.Fatal(m, err)
			}
			var duplicate bytes.Buffer
			if err := WriteUnequalBore(&duplicate, m); err != nil || !bytes.Equal(out.Bytes(), duplicate.Bytes()) {
				t.Fatal("noncanonical skew bore reload", err)
			}
			if os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
				return
			}
			path, report := filepath.Join(t.TempDir(), "skew.step"), filepath.Join(t.TempDir(), "mass.json")
			if err := os.WriteFile(path, out.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			stepOracle(t, report, "step-mass-precise", path)
			raw, err := os.ReadFile(report)
			if err != nil {
				t.Fatal(err)
			}
			var oracle struct {
				Volume, Area float64
				Centroid     [3]float64
			}
			if err := json.Unmarshal(raw, &oracle); err != nil {
				t.Fatal(err)
			}
			native, err := trimmed.BoxSkewCrossedBores(context.Background(), body.Frame, body.Dimensions, body.Crossing, body.XRadius, body.ZRadius, body.Offset, 1e-9)
			if err != nil {
				t.Fatal(err)
			}
			mass, err := native.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-10, MaxSubdivisions: 128})
			if err != nil {
				t.Fatal(err)
			}
			center := geom.Vec3{X: oracle.Centroid[0], Y: oracle.Centroid[1], Z: oracle.Centroid[2]}
			if math.Abs(oracle.Volume-mass.Volume) > 2e-6 || math.Abs(oracle.Area-mass.Area) > 2e-6 || center.Distance(mass.Centroid) > 1e-7 {
				t.Fatalf("OCCT readback %+v differs from native %+v", oracle, mass)
			}
		})
	}
}
