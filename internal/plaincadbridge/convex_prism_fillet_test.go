package plaincadbridge

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"testing"
)

func TestConvexPrismFilletDispatchesOriginalAxialEdges(t *testing.T) {
	for _, pose := range []struct {
		name string
		mapv func(vector) vector
	}{
		{"xy", func(p vector) vector { return p }},
		{"yz", func(p vector) vector { return vector{p[2] + 2, p[0] - 3, p[1] + 4} }},
	} {
		t.Run(pose.name, func(t *testing.T) {
			b := New()
			zero := pose.mapv(vector{})
			axisEnd := pose.mapv(vector{0, 0, 1})
			axis := vector{axisEnd[0] - zero[0], axisEnd[1] - zero[1], axisEnd[2] - zero[2]}
			stock := shape(t, b, map[string]any{"op": "prism", "outer": []vector{
				zero, pose.mapv(vector{4, 0, 0}), pose.mapv(vector{0, 3, 0})},
				"offset": vector{5 * axis[0], 5 * axis[1], 5 * axis[2]}})
			a, c := pose.mapv(vector{4, 0, 0}), pose.mapv(vector{4, 0, 5})
			acute := edgeRef(a, c)
			out := shape(t, b, map[string]any{"op": "fillet", "handle": stock.Handle,
				"edges": []any{acute, edgeRef(c, a)}, "normal": axis, "direction": axis, "radius": 5.0 / 16})
			const radius = 5.0 / 16
			w := math.Atan2(3, -4)
			area := 6 - radius*radius*(3-w/2)
			perimeter := 12 - 6*radius + radius*w
			near(t, out.GeometryAssertions.Volume, 5*area)
			near(t, out.GeometryAssertions.SurfaceArea, 2*area+5*perimeter)
			if out.Bounds.Min != pose.mapv(vector{}) || out.Bounds.Max != pose.mapv(vector{54.0 / 16, 3, 5}) ||
				!out.GeometryAssertions.Valid || out.GeometryAssertions.SolidCount != 1 {
				t.Fatal("actual native cylindrical bounds and solid topology", out)
			}
			invoke(t, b, map[string]any{"op": "mesh", "handle": out.Handle, "linearDeflection": .025, "angularDeflection": .2})
			invoke(t, b, map[string]any{"op": "stl", "handle": out.Handle, "linearDeflection": .025, "angularDeflection": .2})
			first, err := b.exportSTEP([]string{out.Handle})
			if err != nil {
				t.Fatal(err)
			}
			data, err := base64.StdEncoding.DecodeString(first.Base64)
			if err != nil {
				t.Fatal(err)
			}
			loaded, err := b.importSTEP(string(data))
			if err != nil || loaded.Bounds != out.Bounds || loaded.GeometryAssertions != out.GeometryAssertions {
				t.Fatal("STEP changed the fillet", loaded, out, err)
			}
			second, err := b.exportSTEP([]string{loaded.Handle})
			if err != nil || second.Base64 != first.Base64 {
				t.Fatal("STEP re-export changed the canonical file", err)
			}
			var available struct{ Available bool }
			for _, a := range []struct {
				handle string
				want   bool
			}{{stock.Handle, true}, {out.Handle, false}} {
				if e := json.Unmarshal(invoke(t, b, map[string]any{"op": "edgeAvailable", "handle": a.handle,
					"edgeStart": pose.mapv(vector{4, 0, 0}), "edgeEnd": pose.mapv(vector{4, 0, 5})}), &available); e != nil || available.Available != a.want {
					t.Fatal("only the new solid consumes the original edge", a, available, e)
				}
			}
			rejectGroup(t, b, map[string]any{"op": "fillet", "handle": stock.Handle,
				"edges": []any{acute, edgeRef(zero, a)}, "normal": axis, "direction": axis, "radius": 5.0 / 16})
		})
	}
}
