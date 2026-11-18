package trimmed

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func openParabolicGraphFixture(t *testing.T, controls [3]float64, perturb bool, axes ...int) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	var out [3]*geom.BSplineSurface
	for owner := range out {
		var poles []geom.Vec3
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				u, v := float64(i)/2, float64(j)/2
				z := 0.0
				if owner > 0 {
					z = 3 + u*v/64
				}
				if owner == 1 {
					if len(axes) > 0 && axes[0] == 1 {
						z += u - controls[j]
					} else {
						z += v - controls[i]
					}
					if perturb {
						z += u * v / 32
					}
				}
				poles = append(poles, geom.Vec3{X: u, Y: v, Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		if owner > 0 {
			p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			knot := .3125
			if owner == 1 {
				knot = .375
			}
			if (owner == 1) != (len(axes) > 0 && axes[0] == 1) {
				p, e = p.InsertUKnot(knot, 1, geom.SurfacePatchOptions{})
			} else {
				p, e = p.InsertVKnot(knot, 1, geom.SurfacePatchOptions{})
			}
			if e != nil {
				t.Fatal(e)
			}
			s, e = p.ToBSplineSurface()
			if e != nil {
				t.Fatal(e)
			}
		}
		out[owner] = s
	}
	return out[0], out[1], out[2]
}
func parabolicGraphVolume(order int, op parametric.BooleanOp) float64 {
	if op == parametric.Union {
		return 3 + 1.0/256 + 107.0/480
	}
	if op == parametric.Difference {
		if order == 0 {
			return 107.0 / 480
		}
		return 9.0 / 160
	}
	return 3 + 1.0/256 - 9.0/160
}
func TestOpenParabolicGraphOCCT(t *testing.T)  { testParabolicOCCT(t, false) }
func TestOpenParabolicVGraphOCCT(t *testing.T) { testParabolicOCCT(t, true) }
func testParabolicOCCT(t *testing.T, vertical bool) {
	exe := os.Getenv("PLAINSOLID_OCCT_PARABOLIC_ROOFS")
	if exe == "" {
		t.Skip("optional independent original OCCT stocks")
	}
	args := []string{}
	if vertical {
		args = append(args, "vertical")
	}
	data, e := exec.Command(exe, args...).Output()
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		Order, Operation, Solids int
		Valid                    bool
		Vertical                 bool
		Volume                   float64
	}
	if e = json.Unmarshal(data, &rows); e != nil || len(rows) != 6 {
		t.Fatal(e)
	}
	seen := map[[2]int]bool{}
	for _, r := range rows {
		k := [2]int{r.Order, r.Operation}
		if seen[k] || r.Order < 0 || r.Order > 1 || r.Operation < 0 || r.Operation > 2 {
			t.Fatal("oracle identity")
		}
		seen[k] = true
		op := []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference}[r.Operation]
		if r.Vertical != vertical || !r.Valid || r.Solids != 1 || math.Abs(r.Volume-parabolicGraphVolume(r.Order, op)) > 2e-8 {
			t.Fatal(r)
		}
	}
}
