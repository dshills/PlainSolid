package plaincadbridge

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/exchange"
	"plainsolid/geom"
)

func TestObliqueThroughBoreScopeAndSTL(t *testing.T) {
	const radius = 0.35
	for _, tc := range []struct {
		name           string
		origin, axis   vector
		height, length float64
		entry, exit    vector
		exitNormal     vector
	}{
		{"opposite caps", vector{0, 2, -3}, vector{1, 0, 3}, 11, 2 * math.Sqrt(10), vector{1, 2, 0}, vector{3, 2, 6}, vector{0, 0, 1}},
		{"adjacent caps", vector{-1, 2, -1}, vector{2, 0, 1}, 10, 1.5 * math.Sqrt(5), vector{1, 2, 0}, vector{4, 2, 1.5}, vector{1, 0, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bridge := New()
			stock := box(t, bridge, vector{}, vector{4, 5, 6})
			tool := circle(t, bridge, tc.origin, tc.axis, radius, tc.height)
			n, err := tc.axis.vec().Unit()
			if err != nil {
				t.Fatal(err)
			}
			removed := math.Pi * radius * radius * tc.length
			entryOpen := math.Pi * radius * radius / n.Z
			exitOpen := math.Pi * radius * radius / n.Dot(tc.exitNormal.vec())
			wantVolume := 120 - removed
			wantArea := 148 + 2*math.Pi*radius*tc.length - entryOpen - exitOpen
			count, next := len(bridge.shapes), bridge.next
			near(t, overlapMeasure(t, bridge, stock.Handle, tool.Handle), removed)
			near(t, overlapMeasure(t, bridge, tool.Handle, stock.Handle), removed)
			if len(bridge.shapes) != count || bridge.next != next {
				t.Fatal("scope proof allocated a shape")
			}
			direct := shape(t, bridge, map[string]any{"op": "cut", "a": stock.Handle, "b": tool.Handle})
			cut, err := bridge.cutMany(stock.Handle, []string{tool.Handle})
			if err != nil || cut.GeometryAssertions != direct.GeometryAssertions || cut.Bounds != stock.Bounds {
				t.Fatalf("direct %v scoped %v: %v", direct, cut, err)
			}
			near(t, cut.GeometryAssertions.Volume, wantVolume)
			near(t, cut.GeometryAssertions.SurfaceArea, wantArea)
			body := bridge.shapes[cut.Handle].bore
			props, err := body.solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			near(t, props.Volume, wantVolume)
			near(t, props.Area, wantArea)

			for _, f := range []struct {
				point, normal vector
				opening       float64
			}{{tc.entry, vector{0, 0, -1}, entryOpen}, {tc.exit, tc.exitNormal, exitOpen}} {
				source, err := matchingPlanarFace(body.base, f.point.vec(), f.normal.vec())
				if err != nil {
					t.Fatal(err)
				}
				kept, err := body.planarFace(f.point.vec(), f.normal.vec())
				if err != nil || source.Face != kept.Face {
					t.Fatalf("opening changed original face %d to %d: %v", source.Face, kept.Face, err)
				}
				near(t, kept.Area, source.Area-f.opening)
				if len(body.solid.Faces()[kept.Face].Holes) != 1 {
					t.Fatal("missing retained elliptical rim")
				}
			}
			meshJSON := invoke(t, bridge, map[string]any{"op": "mesh", "handle": cut.Handle, "linearDeflection": 0.5, "angularDeflection": 0.2, "analyticNormals": true})
			var rendered Mesh
			if err := json.Unmarshal(meshJSON, &rendered); err != nil || len(rendered.Positions) == 0 || rendered.GeometryAssertions != cut.GeometryAssertions {
				t.Fatalf("oblique bore display: %v", err)
			}
			stepModel, err := bridge.exportSTEP([]string{cut.Handle})
			if err != nil || len(stepModel.After) != 1 {
				t.Fatalf("oblique bore STEP export: %v", err)
			}
			data, err := base64.StdEncoding.DecodeString(stepModel.Base64)
			if err != nil {
				t.Fatal(err)
			}
			imported, err := bridge.importSTEP(string(data))
			if err != nil || imported.Bounds != cut.Bounds {
				t.Fatalf("oblique bore STEP import: %v", err)
			}
			near(t, imported.GeometryAssertions.Volume, wantVolume)
			near(t, imported.GeometryAssertions.SurfaceArea, wantArea)
			again, err := bridge.exportSTEP([]string{imported.Handle})
			if err != nil || again.Base64 != stepModel.Base64 {
				t.Fatalf("oblique bore re-export changed the STEP graph: %v", err)
			}
			var parsed struct{ Base64 string }
			out := invoke(t, bridge, map[string]any{"op": "stl", "handle": cut.Handle, "linearDeflection": 0.02, "angularDeflection": 0.2})
			if err := json.Unmarshal(out, &parsed); err != nil {
				t.Fatal(err)
			}
			raw, err := base64.StdEncoding.DecodeString(parsed.Base64)
			if err != nil {
				t.Fatal(err)
			}
			m, err := exchange.ReadSTL(bytes.NewReader(raw), exchange.Limits{})
			if err != nil {
				t.Fatal(err)
			}
			if err := m.ValidateSolid(); err != nil {
				t.Fatal(err)
			}
			if math.Abs(m.SignedVolume()-wantVolume) > removed*0.02 {
				t.Fatalf("STL volume %.10g, want %.10g", m.SignedVolume(), wantVolume)
			}
		})
	}
}

func TestObliqueThroughBoreRejectsIncompleteOpeningsAtomically(t *testing.T) {
	bridge := New()
	stock := box(t, bridge, vector{}, vector{4, 5, 6})
	for _, tc := range []struct {
		name           string
		origin, axis   vector
		radius, height float64
		diagnostic     string
	}{
		{"entry generators uncovered", vector{1, 2, 0}, vector{1, 0, 3}, 0.35, 11, "does not cover both entire stock openings"},
		{"exit generators uncovered", vector{0, 2, -3}, vector{1, 0, 3}, 0.35, 3 * math.Sqrt(10), "does not cover both entire stock openings"},
		{"ellipse touches stock side", vector{0, 0.35, -3}, vector{1, 0, 3}, 0.35, 11, "requires strictly interior elliptical openings"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tool := circle(t, bridge, tc.origin, tc.axis, tc.radius, tc.height)
			before, next := len(bridge.shapes), bridge.next
			if _, err := bridge.overlapVolume(stock.Handle, tool.Handle); err == nil || !strings.Contains(err.Error(), tc.diagnostic) {
				t.Fatalf("invalid scope result: %v", err)
			}
			if _, err := bridge.cutMany(stock.Handle, []string{tool.Handle}); err == nil || !strings.Contains(err.Error(), tc.diagnostic) {
				t.Fatalf("invalid cutMany result: %v", err)
			}
			if _, err := bridge.curvedBoolean(bridge.shapes[stock.Handle], bridge.shapes[tool.Handle], "cut"); err == nil || !strings.Contains(err.Error(), tc.diagnostic) {
				t.Fatalf("invalid cut result: %v", err)
			}
			if len(bridge.shapes) != before || bridge.next != next || bridge.shapes[stock.Handle].result != stock {
				t.Fatal("rejection changed stock or leaked a handle")
			}
		})
	}
}

func TestObliqueFiniteCapContactProof(t *testing.T) {
	frame, err := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{X: 1, Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	tool, err := newCylinder(frame, 0.5, 4)
	if err != nil {
		t.Fatal(err)
	}
	r := tool.radius * math.Hypot(frame.XAxis().X, frame.YAxis().X)
	plane := func(x float64) geom.Plane {
		p, err := geom.NewPlane(geom.Vec3{X: x}, geom.Vec3{X: -1})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	if !toolCapClearsOpening(tool, plane(r), 0) ||
		toolCapClearsOpening(tool, plane(math.Nextafter(r, 0)), 0) {
		t.Fatal("failed exact represented disk support equality/neighbor check")
	}
}
