package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"strings"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
)

func planarSewToleranceFixture(t *testing.T, s Solid) ([]PlanarSewFace, []PlanarSewSeam) {
	t.Helper()
	faces := make([]PlanarSewFace, len(s.faces))
	seams := make([]PlanarSewSeam, len(s.edges))
	used := make([]bool, len(s.edges))
	for fi, f := range s.faces {
		surface, ok := f.Surface.(geom.PlaneSurface)
		if !ok {
			t.Fatal("planar example requires original plane charts")
		}
		faces[fi].Surface, faces[fi].Reversed = surface, f.Reversed
		for li, wire := range append([][]Use{f.Outer}, f.Holes...) {
			var uv []geom.Vec2
			for ui, u := range wire {
				if u.Curve != nil {
					t.Fatal("planar example requires straight trims")
				}
				uv = append(uv, u.UVStart)
				ref := PlanarSewBorder{Face: fi, Loop: li, Edge: ui}
				if used[u.Edge] {
					seams[u.Edge].Second = ref
					seams[u.Edge].Reversed = seams[u.Edge].Reversed != u.Reversed
				} else {
					used[u.Edge] = true
					seams[u.Edge] = PlanarSewSeam{First: ref, Reversed: u.Reversed}
				}
			}
			if li == 0 {
				faces[fi].Outer = uv
			} else {
				faces[fi].Holes = append(faces[fi].Holes, uv)
			}
		}
	}
	return faces, seams
}

func TestSewPlanarWithinTolerance(t *testing.T) {
	ctx := context.Background()
	body, err := brep.Box(geom.Vec3{}, geom.Vec3{X: 3, Y: 2, Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	trim, err := FromPlanar(body, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	faces, pairs := planarSewToleranceFixture(t, trim)
	const shift = 1.0 / 1024
	top := -1
	for id, face := range faces {
		if face.Surface.Frame().ZAxis().Z > 0 {
			top = id
			break
		}
	}
	if top <= 0 {
		t.Fatal("top repair face must not be the orientation seed", top)
	}
	f := &faces[top]
	f.Reversed = !f.Reversed
	for i := range f.Outer {
		f.Outer[i].X += shift
		f.Outer[i].Y -= shift
	}
	for _, o := range []PlanarToleranceSewOptions{
		{Tolerance: shift, RepairOrientation: true},
		{Tolerance: 2 * shift},
		{Tolerance: 2 * shift, RepairOrientation: true, Collision: collision.Options{MaxPairs: 1}},
	} {
		failed, report, err := SewPlanarWithinTolerance(ctx, faces, pairs, o)
		if err == nil || len(failed.faces)+len(report.Vertices) != 0 {
			t.Fatal("failed planar repair exposed a partial result", err, failed.Topology())
		}
	}
	out, history, err := SewPlanarWithinTolerance(ctx, faces, pairs, PlanarToleranceSewOptions{Tolerance: 2 * shift, RepairOrientation: true})
	if err != nil {
		t.Fatal(err)
	}
	want := big.NewRat(1, 524288)
	if len(history.FlippedFaces) != 1 || history.FlippedFaces[0] != top || history.MovedVertices != 4 || history.MaxSquaredDisplacement.Cmp(want) != 0 ||
		history.ExactVolume.Cmp(big.NewRat(6, 1)) != 0 || !history.Collision.Complete || len(history.Collision.Contacts) != 0 {
		t.Fatal("wrong planar repair witnesses", history)
	}
	for i, source := range faces {
		if out.faces[i].Surface != source.Surface {
			t.Fatal("the repair changed an original supporting plane", i)
		}
		for j, v := range history.Vertices[i][0] {
			if p := out.vertices[v.Vertex]; p.X != 0 && p.X != 3 || p.Y != 0 && p.Y != 2 || p.Z != 0 && p.Z != 1 {
				t.Fatal("repaired corner missed the original plane intersection", i, j, p)
			}
			if i == top && v.SquaredDisplacement.Cmp(want) != 0 || i != top && v.SquaredDisplacement.Sign() != 0 {
				t.Fatal("original vertex displacement report lost a witness", i, j, v)
			}
		}
	}
	var native bytes.Buffer
	if err := WriteJSON(&native, out); err != nil {
		t.Fatal(err)
	}
	got, err := ReadJSONContext(ctx, &native, JSONLimits{})
	if err != nil {
		t.Fatal(err)
	}
	proof, err := got.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !proof.Complete {
		t.Fatal("reloaded original charts lack continuous agreement", proof.Complete, err)
	}
	mass, err := got.MassProperties(ctx, parametric.IntegrationOptions{})
	if err != nil || math.Abs(mass.Volume-6) > 1e-10 {
		t.Fatal("reloaded repaired planar material has wrong volume", mass, err)
	}
}

func TestSewPlanarWithinToleranceProjectedLine(t *testing.T) {
	ctx := context.Background()
	input, _, small, _ := nonconformingStraightSewFixture(t)
	cut := input.vertices[input.edges[small].End]
	closed, _, err := SewExactStraightSubdivisions(ctx, input.vertices, input.edges, input.faces, ExactStraightSewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	faces, pairs := planarSewToleranceFixture(t, closed)
	const shift = 1.0 / 1024
	count := 0
	for i, f := range faces {
		for j, uv := range f.Outer {
			p, err := f.Surface.Eval(uv.X, uv.Y)
			if err != nil {
				t.Fatal(err)
			}
			if p != cut {
				continue
			}
			f := &faces[i]
			frame := f.Surface.Frame()
			delta := 2 * shift
			if count == 1 {
				delta = 0
			}
			f.Outer[j].X += delta * frame.XAxis().X
			f.Outer[j].Y += delta * frame.YAxis().X
			count++
		}
	}
	if count != 2 {
		t.Fatal("fixture midpoint must have exactly two original planes", count)
	}
	out, h, err := SewPlanarWithinTolerance(ctx, faces, pairs, PlanarToleranceSewOptions{Tolerance: shift})
	if err != nil {
		t.Fatal(err)
	}
	want := cut.Add(geom.Vec3{X: shift})
	found := false
	for _, p := range out.vertices {
		found = found || p == want
	}
	if !found || h.ExactVolume.Cmp(big.NewRat(6, 1)) != 0 || h.MovedVertices != 2 || h.MaxSquaredDisplacement.Cmp(big.NewRat(1, 1048576)) != 0 {
		t.Fatal("two-plane seam projection did not preserve its exact mean", want, out.vertices, h)
	}
	pairs[0].First = pairs[1].First
	if s, report, err := SewPlanarWithinTolerance(ctx, faces, pairs, PlanarToleranceSewOptions{Tolerance: shift}); !errors.Is(err, geom.ErrInvalid) || len(s.faces)+len(report.Vertices) != 0 {
		t.Fatal("duplicate seam endpoint ownership was accepted", err)
	}
}

func TestPlanarRepairRejectsInconsistentPlanes(t *testing.T) {
	frame := func(z float64) planarSewChart {
		t.Helper()
		f, err := geom.NewFrame(geom.Vec3{Z: z}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		if err != nil {
			t.Fatal(err)
		}
		surface, err := geom.NewPlaneSurface(f)
		if err != nil {
			t.Fatal(err)
		}
		c, err := newPlanarSewChart(surface)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	a, b := frame(0), frame(1.0/1024)
	if _, err := reconcilePlanarVertex([]planarSewChart{a, b}, []planarSewSample{{0, a.origin}, {1, b.origin}}, []int{0, 1}); !errors.Is(err, geom.ErrConvergence) || !strings.Contains(err.Error(), "do not meet") {
		t.Fatal("parallel offset original supports require a separate topology repair policy", err)
	}
}
