package plaincadbridge

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"math"
	"strconv"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange"
	"plainsolid/geom"
)

// generalBooleanOperandSolid is the trimmed.Solid the general Boolean adapter
// will read from a stored shape (specification L2): a planar solid lifts with
// trimmed.FromPlanar, a cylinder with trimmed.FromParametric, and a bored or
// analytic body holds one.
func generalBooleanOperandSolid(t *testing.T, s stored) trimmed.Solid {
	t.Helper()
	var solid trimmed.Solid
	var err error
	switch {
	case s.bore != nil:
		solid = s.bore.solid
	case s.analytic != nil:
		solid = s.analytic.solid
	case s.cylinder != nil:
		solid, err = trimmed.FromParametric(s.cylinder.solid)
	case s.planar():
		solid, err = trimmed.FromPlanar(s.solid, 1e-7)
	default:
		t.Fatal("stored shape holds no trimmed solid")
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(solid.Faces()) == 0 {
		t.Fatal("stored shape holds an empty trimmed solid")
	}
	return solid
}

// generalBooleanError is the error measure of specification A8.
func generalBooleanError(value, expected float64) float64 {
	return math.Abs(value-expected) / math.Max(1, math.Abs(expected))
}

// TestGeneralBooleanFoundation probes what the general Boolean trial needs
// from brep/trimmed on its operands (specification E1): Classify at the
// bridge tolerance answers inside, outside and boundary at analytic points,
// and MassProperties at integration tolerance 1e-9 reproduces the volume and
// area the bridge reports within 1e-7 × max(1, |value|). The achieved errors
// are logged.
func TestGeneralBooleanFoundation(t *testing.T) {
	const tolerance = 1e-7
	const in, out, on = parametric.Inside, parametric.Outside, parametric.Boundary
	type sample struct {
		point geom.Vec3
		want  parametric.Location
	}
	// around returns a boundary point with the points a step along its outward
	// normal on either side of it.
	around := func(p, normal geom.Vec3) []sample {
		const step = 1e-5
		return []sample{{p, on}, {p.Add(normal.Scale(step)), out}, {p.Add(normal.Scale(-step)), in}}
	}
	join := func(groups ...[]sample) []sample {
		var all []sample
		for _, g := range groups {
			all = append(all, g...)
		}
		return all
	}
	r2 := math.Sqrt(.5)
	// The oblique cylinder has axis (1,2,2)/3 through the origin.
	axis := geom.Vec3{X: 1. / 3, Y: 2. / 3, Z: 2. / 3}
	radial := geom.Vec3{X: 2 / math.Sqrt(5), Y: -1 / math.Sqrt(5)}
	roundedArea := 140.*108 - (4-math.Pi)*100
	holedArea := 196 - 9*math.Pi
	cases := []struct {
		name         string
		kind         string
		requests     []string
		volume, area float64
		samples      []sample
	}{
		{
			name: "planar box", kind: "planar",
			requests: []string{`{"op":"box","min":[0,0,0],"max":[10,8,6]}`},
			volume:   480, area: 376,
			samples: join(
				[]sample{{geom.Vec3{X: 5, Y: 4, Z: 3}, in}, {geom.Vec3{X: 11, Y: 4, Z: 3}, out}, {geom.Vec3{X: -1, Y: -1, Z: -1}, out}},
				around(geom.Vec3{X: 5, Y: 4, Z: 6}, geom.Vec3{Z: 1}),
				around(geom.Vec3{X: 10, Y: 4, Z: 6}, geom.Vec3{X: r2, Z: r2}),
				around(geom.Vec3{X: 10, Y: 8, Z: 6}, geom.Vec3{X: 1, Y: 1, Z: 1}.Scale(1/math.Sqrt(3))),
				[]sample{{geom.Vec3{X: 5, Y: 4, Z: 6 + 5e-8}, on}, {geom.Vec3{X: 5, Y: 4, Z: 6 - 5e-8}, on}},
			),
		},
		{
			name: "planar box with a void", kind: "planar",
			requests: []string{
				`{"op":"box","min":[0,0,0],"max":[10,10,10]}`,
				`{"op":"box","min":[2,2,2],"max":[8,8,8]}`,
				`{"op":"cut","a":"ps:1","b":"ps:2"}`,
			},
			volume: 784, area: 816,
			samples: join(
				[]sample{{geom.Vec3{X: 1, Y: 5, Z: 5}, in}, {geom.Vec3{X: 5, Y: 5, Z: 5}, out}, {geom.Vec3{X: 11, Y: 5, Z: 5}, out}},
				around(geom.Vec3{X: 2, Y: 5, Z: 5}, geom.Vec3{X: 1}),
				around(geom.Vec3{X: 10, Y: 5, Z: 5}, geom.Vec3{X: 1}),
				around(geom.Vec3{X: 2, Y: 2, Z: 5}, geom.Vec3{X: r2, Y: r2}),
			),
		},
		{
			name: "cylinder along z", kind: "cylinder",
			requests: []string{`{"op":"circle","origin":[1,2,3],"normal":[0,0,1],"radius":2,"height":5}`},
			volume:   20 * math.Pi, area: 28 * math.Pi,
			samples: join(
				[]sample{{geom.Vec3{X: 1, Y: 2, Z: 5.5}, in}, {geom.Vec3{X: 2, Y: 2.5, Z: 4}, in}, {geom.Vec3{X: 4, Y: 2, Z: 5}, out}, {geom.Vec3{X: 1, Y: 2, Z: 9}, out}},
				around(geom.Vec3{X: 3, Y: 2, Z: 5.5}, geom.Vec3{X: 1}),
				around(geom.Vec3{X: 1, Y: 4, Z: 5.5}, geom.Vec3{Y: 1}),
				around(geom.Vec3{X: 1 - 2*r2, Y: 2 - 2*r2, Z: 4}, geom.Vec3{X: -r2, Y: -r2}),
				around(geom.Vec3{X: 1, Y: 2, Z: 8}, geom.Vec3{Z: 1}),
				around(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{Z: -1}),
				around(geom.Vec3{X: 1.5, Y: 3, Z: 8}, geom.Vec3{Z: 1}),
				around(geom.Vec3{X: 1, Y: 0, Z: 8}, geom.Vec3{Y: -r2, Z: r2}),
			),
		},
		{
			name: "oblique cylinder", kind: "cylinder",
			requests: []string{`{"op":"circle","origin":[0,0,0],"normal":[1,2,2],"radius":1.5,"height":4}`},
			volume:   9 * math.Pi, area: 16.5 * math.Pi,
			samples: join(
				[]sample{{axis.Scale(2), in}, {axis.Scale(2).Add(radial.Scale(2)), out}, {axis.Scale(4.5), out}, {axis.Scale(-.5), out}},
				around(axis.Scale(2).Add(radial.Scale(1.5)), radial),
				around(axis.Scale(1).Add(radial.Scale(-1.5)), radial.Scale(-1)),
				around(axis.Scale(4), axis),
				around(axis.Scale(4).Add(radial.Scale(.75)), axis),
				around(geom.Vec3{}, axis.Scale(-1)),
			),
		},
		{
			name: "bored box", kind: "bore",
			requests: []string{
				`{"op":"box","min":[0,0,0],"max":[10,10,10]}`,
				`{"op":"circle","origin":[5,5,-1],"normal":[0,0,1],"radius":2,"height":12}`,
				`{"op":"cut","a":"ps:1","b":"ps:2"}`,
			},
			volume: 1000 - 40*math.Pi, area: 600 + 32*math.Pi,
			samples: join(
				[]sample{{geom.Vec3{X: 1, Y: 1, Z: 5}, in}, {geom.Vec3{X: 5, Y: 5, Z: 5}, out}, {geom.Vec3{X: 6, Y: 5.5, Z: 9}, out}, {geom.Vec3{X: 11, Y: 5, Z: 5}, out}, {geom.Vec3{X: 5, Y: 5, Z: 11}, out}},
				around(geom.Vec3{X: 7, Y: 5, Z: 5}, geom.Vec3{X: -1}),
				around(geom.Vec3{X: 5, Y: 3, Z: 4}, geom.Vec3{Y: 1}),
				around(geom.Vec3{X: 5 + 2*r2, Y: 5 + 2*r2, Z: 2}, geom.Vec3{X: -r2, Y: -r2}),
				around(geom.Vec3{X: 1, Y: 1, Z: 10}, geom.Vec3{Z: 1}),
				around(geom.Vec3{X: 5, Y: 7, Z: 10}, geom.Vec3{Y: -r2, Z: r2}),
			),
		},
		{
			name: "rounded rectangle extrusion", kind: "analytic",
			requests: []string{`{"op":"mixedExtrude","profile":{"height":10,"kind":"roundedRectangle","max":[70,54],"min":[-70,-54],"normal":[0,0,1],"origin":[0,0,0],"radii":[10,10,10,10],"xDirection":[1,0,0]}}`},
			volume:   roundedArea * 10, area: 2*roundedArea + (2*(140+108)-80+20*math.Pi)*10,
			samples: join(
				[]sample{{geom.Vec3{Z: 5}, in}, {geom.Vec3{X: 69, Y: 53, Z: 5}, out}, {geom.Vec3{X: 66, Y: 50, Z: 5}, in}, {geom.Vec3{X: 71, Z: 5}, out}},
				around(geom.Vec3{X: 60 + 10*r2, Y: 44 + 10*r2, Z: 5}, geom.Vec3{X: r2, Y: r2}),
				around(geom.Vec3{X: -60 - 10*r2, Y: 44 + 10*r2, Z: 2}, geom.Vec3{X: -r2, Y: r2}),
				around(geom.Vec3{X: 70, Z: 5}, geom.Vec3{X: 1}),
				around(geom.Vec3{X: 3, Y: 4, Z: 10}, geom.Vec3{Z: 1}),
				around(geom.Vec3{X: 60, Y: 54, Z: 5}, geom.Vec3{Y: 1}),
			),
		},
		{
			name: "plate with a round hole", kind: "analytic",
			requests: []string{`{"op":"mixedExtrude","profile":{"circles":[{"center":[0,0],"radius":3}],"height":3,"kind":"polygon","normal":[1,0,0],"origin":[3,5,7],"outer":[[-7,-7],[7,-7],[7,7],[-7,7]],"xDirection":[0,1,0]}}`},
			volume:   holedArea * 3, area: 2*holedArea + 56*3 + 18*math.Pi,
			samples: join(
				[]sample{{geom.Vec3{X: 4.5, Y: 10, Z: 7}, in}, {geom.Vec3{X: 4.5, Y: 5, Z: 7}, out}, {geom.Vec3{X: 4.5, Y: 13, Z: 7}, out}, {geom.Vec3{X: 7, Y: 10, Z: 7}, out}},
				around(geom.Vec3{X: 4.5, Y: 8, Z: 7}, geom.Vec3{Y: -1}),
				around(geom.Vec3{X: 4, Y: 5 - 3*r2, Z: 7 + 3*r2}, geom.Vec3{Y: r2, Z: -r2}),
				around(geom.Vec3{X: 3, Y: 10, Z: 7}, geom.Vec3{X: -1}),
				around(geom.Vec3{X: 6, Y: 10, Z: 9}, geom.Vec3{X: 1}),
				around(geom.Vec3{X: 6, Y: 5, Z: 10}, geom.Vec3{X: r2, Z: -r2}),
			),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := New()
			var reported Shape
			for _, request := range tc.requests {
				var reply generalBooleanReply
				if err := json.Unmarshal([]byte(b.Call(request)), &reply); err != nil || !reply.OK {
					t.Fatalf("%s: %+v %v", request, reply.Error, err)
				}
				if err := json.Unmarshal(reply.Result, &reported); err != nil {
					t.Fatal(err)
				}
			}
			s := b.shapes[reported.Handle]
			kind := "planar"
			switch {
			case s.bore != nil:
				kind = "bore"
			case s.analytic != nil:
				kind = "analytic"
			case s.cylinder != nil:
				kind = "cylinder"
			}
			if kind != tc.kind {
				t.Fatalf("stored as %s, want %s", kind, tc.kind)
			}
			solid := generalBooleanOperandSolid(t, s)
			for _, p := range tc.samples {
				got, err := solid.Classify(p.point, tolerance)
				if err != nil {
					t.Fatalf("Classify(%v): %v", p.point, err)
				}
				if got != p.want {
					t.Errorf("Classify(%v) = %d, want %d", p.point, got, p.want)
				}
			}
			mass, err := solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
			if err != nil {
				t.Fatal(err)
			}
			a := reported.GeometryAssertions
			volume, area := generalBooleanError(mass.Volume, a.Volume), generalBooleanError(mass.Area, a.SurfaceArea)
			t.Logf("%s (%s, %d faces, tolerance %.3g): MassProperties against the bridge: volume error %.3g, area error %.3g; against the closed form: volume error %.3g, area error %.3g; bridge against the closed form: volume %.3g, area %.3g",
				tc.name, kind, len(solid.Faces()), solid.Tolerance(), volume, area,
				generalBooleanError(mass.Volume, tc.volume), generalBooleanError(mass.Area, tc.area),
				generalBooleanError(a.Volume, tc.volume), generalBooleanError(a.SurfaceArea, tc.area))
			if volume > 1e-7 || area > 1e-7 {
				t.Errorf("MassProperties volume %.17g area %.17g, bridge reports %.17g and %.17g", mass.Volume, mass.Area, a.Volume, a.SurfaceArea)
			}
			if generalBooleanError(a.Volume, tc.volume) > 1e-7 || generalBooleanError(a.SurfaceArea, tc.area) > 1e-7 {
				t.Errorf("bridge reports volume %.17g area %.17g, closed form %.17g and %.17g", a.Volume, a.SurfaceArea, tc.volume, tc.area)
			}
		})
	}
}

// TestGeneralBooleanPublication probes owner decision D4 of the general
// Boolean plan: whether a result with a void, or with two separate solids,
// can be stored as an analytic body through publish, which putAnalyticBody
// refuses, and then inspected, meshed and exported like any other shape. The
// mesh checks are the ones the acceptance source tests apply: a non-empty
// mesh carrying the shape's assertions, and an STL that reads back as a
// closed manifold of the same volume.
func TestGeneralBooleanPublication(t *testing.T) {
	cases := []struct {
		name       string
		requests   []string
		solidCount int
		shells     int
	}{
		{
			name: "one solid with a void",
			requests: []string{
				`{"op":"box","min":[0,0,0],"max":[10,10,10]}`,
				`{"op":"box","min":[2,2,2],"max":[8,8,8]}`,
				`{"op":"cut","a":"ps:1","b":"ps:2"}`,
			},
			solidCount: 1, shells: 2,
		},
		{
			name: "two separate solids",
			requests: []string{
				`{"op":"box","min":[0,0,0],"max":[10,10,10]}`,
				`{"op":"box","min":[20,0,0],"max":[21,1,1]}`,
				`{"op":"fuse","a":"ps:1","b":"ps:2"}`,
			},
			solidCount: 2, shells: 2,
		},
	}
	call := func(t *testing.T, b *Bridge, request string, into any) {
		t.Helper()
		var reply generalBooleanReply
		if err := json.Unmarshal([]byte(b.Call(request)), &reply); err != nil || !reply.OK {
			t.Fatalf("%s: %+v %v", request, reply.Error, err)
		}
		if err := json.Unmarshal(reply.Result, into); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := New()
			var legacy Shape
			for _, request := range tc.requests {
				call(t, b, request, &legacy)
			}
			if legacy.GeometryAssertions.SolidCount != tc.solidCount {
				t.Fatalf("legacy solidCount %d, want %d", legacy.GeometryAssertions.SolidCount, tc.solidCount)
			}
			solid := generalBooleanOperandSolid(t, b.shapes[legacy.Handle])
			if n := len(solid.Topology().Shells); n != tc.shells {
				t.Fatalf("%d shells, want %d", n, tc.shells)
			}
			mass, err := solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
			if err != nil {
				t.Fatal(err)
			}
			box, err := solid.Bounds()
			if err != nil {
				t.Fatal(err)
			}
			body := &analyticBody{solid: solid, volume: mass.Volume, area: mass.Area}
			if _, err := b.putAnalyticBody(body); err == nil {
				t.Fatal("putAnalyticBody accepted more than one shell; D4 no longer applies")
			}
			published, err := b.publish(stored{analytic: body, result: Shape{
				GeometryAssertions: Assertions{true, mass.Volume, mass.Area, tc.solidCount},
				Bounds:             Bounds{coordinates(box.Min), coordinates(box.Max)},
			}})
			if err != nil {
				t.Fatal(err)
			}
			near7 := func(what string, got, want float64) {
				t.Helper()
				if generalBooleanError(got, want) > 1e-7 {
					t.Errorf("%s %.17g, legacy %.17g", what, got, want)
				}
			}
			var inspected Shape
			call(t, b, `{"op":"inspect","handle":"`+published.Handle+`"}`, &inspected)
			a := inspected.GeometryAssertions
			if !a.Valid || a.SolidCount != tc.solidCount {
				t.Errorf("inspect reports %+v", a)
			}
			near7("volume", a.Volume, legacy.GeometryAssertions.Volume)
			near7("surfaceArea", a.SurfaceArea, legacy.GeometryAssertions.SurfaceArea)
			// trimmed.Solid.Bounds pads its box by the solid's tolerance, so the
			// published bounds differ from the legacy ones by about that much.
			pad := 0.0
			for i := range 3 {
				pad = math.Max(pad, math.Abs(inspected.Bounds.Min[i]-legacy.Bounds.Min[i]))
				pad = math.Max(pad, math.Abs(inspected.Bounds.Max[i]-legacy.Bounds.Max[i]))
			}
			if pad > solid.Tolerance()+1e-9 {
				t.Errorf("bounds differ from the legacy bounds by %.3g, more than the tolerance %.3g", pad, solid.Tolerance())
			}
			var display Mesh
			call(t, b, `{"op":"mesh","handle":"`+published.Handle+`"}`, &display)
			if len(display.Positions) == 0 || len(display.Indices) == 0 || display.GeometryAssertions != a {
				t.Errorf("mesh has %d positions, %d indices, assertions %+v", len(display.Positions), len(display.Indices), display.GeometryAssertions)
			}
			var encoded struct {
				Base64 string `json:"base64"`
			}
			call(t, b, `{"op":"stl","handle":"`+published.Handle+`"}`, &encoded)
			data, err := base64.StdEncoding.DecodeString(encoded.Base64)
			if err != nil {
				t.Fatal(err)
			}
			m, err := exchange.ReadSTL(bytes.NewReader(data), exchange.Limits{})
			if err != nil {
				t.Fatal(err)
			}
			if err := m.ValidateSolid(); err != nil {
				t.Errorf("STL is not a closed manifold: %v", err)
			}
			p, err := m.MassProperties()
			if err != nil {
				t.Fatal(err)
			}
			near7("STL volume", p.Volume, legacy.GeometryAssertions.Volume)
			t.Logf("%s: published through publish with solidCount %d; inspect, mesh and stl succeeded; volume error %.3g, area error %.3g, STL volume error %.3g, %d triangles; bounds differ from the legacy bounds by %.3g (tolerance %.3g)",
				tc.name, tc.solidCount, generalBooleanError(a.Volume, legacy.GeometryAssertions.Volume),
				generalBooleanError(a.SurfaceArea, legacy.GeometryAssertions.SurfaceArea),
				generalBooleanError(p.Volume, legacy.GeometryAssertions.Volume), len(m.Triangles), pad, solid.Tolerance())
			// Observations beyond the three requests D4 names. They are logged
			// and decide nothing here.
			for _, request := range []string{
				`{"op":"clone","handle":"` + published.Handle + `"}`,
				`{"op":"transform","handle":"` + published.Handle + `","matrix":[[1,0,0,5],[0,1,0,0],[0,0,1,0]]}`,
			} {
				var reply generalBooleanReply
				if err := json.Unmarshal([]byte(b.Call(request)), &reply); err != nil {
					t.Fatal(err)
				}
				outcome := "succeeded"
				if !reply.OK {
					outcome = "failed: " + reply.Error.Code + ": " + reply.Error.Message
				} else {
					var moved Shape
					if err := json.Unmarshal(reply.Result, &moved); err != nil {
						t.Fatal(err)
					}
					outcome += " with solidCount " + strconv.Itoa(moved.GeometryAssertions.SolidCount)
				}
				t.Logf("%s: observation: %s %s", tc.name, request[:16], outcome)
			}
		})
	}
}
