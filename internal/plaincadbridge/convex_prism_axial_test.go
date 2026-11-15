package plaincadbridge

import (
	"encoding/json"
	"math"
	"testing"
)

func TestConvexPrismChamferDispatchesOriginalAxialEdges(t *testing.T) {
	for _, pose := range []struct {
		name  string
		world func(vector) vector
		axis  vector
	}{
		{"xy", func(p vector) vector { return p }, vector{0, 0, 1}},
		{"yz", func(p vector) vector { return vector{p[2] + 2, p[0] - 3, p[1] + 4} }, vector{1, 0, 0}},
	} {
		t.Run(pose.name, func(t *testing.T) {
			b := New()
			origin, acute, untouched := pose.world(vector{}), pose.world(vector{4, 0, 0}), pose.world(vector{0, 3, 0})
			stock := shape(t, b, map[string]any{"op": "prism", "outer": []vector{origin, acute, untouched},
				"offset": vector{5 * pose.axis[0], 5 * pose.axis[1], 5 * pose.axis[2]}})
			acuteEnd := pose.world(vector{4, 0, 5})
			right := edgeRef(origin, pose.world(vector{0, 0, 5}))
			out := shape(t, b, map[string]any{"op": "chamfer", "handle": stock.Handle,
				"edges":  []any{edgeRef(acute, acuteEnd), right, edgeRef(acuteEnd, acute)},
				"normal": pose.axis, "direction": pose.axis, "distance": 5.0 / 16})

			// The two corner cuts remove triangles of area 15/512 and 25/512.
			area := 6 - 40.0/512
			perimeter := 12 - 20.0/16 + (math.Sqrt(10)+5*math.Sqrt(2))/16
			near(t, out.GeometryAssertions.Volume, 5*area)
			near(t, out.GeometryAssertions.SurfaceArea, 2*area+5*perimeter)
			if !out.GeometryAssertions.Valid || out.GeometryAssertions.SolidCount != 1 || out.Bounds.Max != pose.world(vector{15.0 / 4, 3, 5}) {
				t.Fatalf("unexpected bevel bounds or topology: %+v", out)
			}
			invoke(t, b, map[string]any{"op": "mesh", "handle": out.Handle})
			invoke(t, b, map[string]any{"op": "stl", "handle": out.Handle})
			for _, check := range []struct {
				handle string
				p, q   vector
				want   bool
			}{
				{stock.Handle, acute, acuteEnd, true},
				{out.Handle, acute, acuteEnd, false},
				{out.Handle, untouched, pose.world(vector{0, 3, 5}), true},
			} {
				var reply struct{ Available bool }
				if err := json.Unmarshal(invoke(t, b, map[string]any{"op": "edgeAvailable", "handle": check.handle,
					"edgeStart": check.p, "edgeEnd": check.q}), &reply); err != nil || reply.Available != check.want {
					t.Fatalf("original edge availability: %+v, %+v, %v", check, reply, err)
				}
			}
			rejectGroup(t, b, map[string]any{"op": "chamfer", "handle": stock.Handle,
				"edges": []any{edgeRef(acute, acuteEnd), right}, "normal": pose.axis, "direction": pose.axis, "distance": 5})
			rejectGroup(t, b, map[string]any{"op": "chamfer", "handle": stock.Handle,
				"edges": []any{edgeRef(acute, acuteEnd), edgeRef(origin, acute)}, "normal": pose.axis, "direction": pose.axis, "distance": 5.0 / 16})
		})
	}
}
