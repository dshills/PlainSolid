package trimmed

import (
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"plainsolid/geom"
	"testing"
	"time"
)

func affineCapAbuttingFixture(t *testing.T, shift float64, sameSide bool) (Solid, Solid) {
	return affineCapAbuttingFixtureShape(t, shift, sameSide, false)
}
func affineCapAbuttingFixtureShape(t *testing.T, shift float64, sameSide, diamond bool) (Solid, Solid) {
	build := func(second bool) Solid {
		ss := []*geom.BSplineSurface{}
		rev := []bool{}
		domains := []geom.ParameterRectangle{}
		for axis := 0; axis < 3; axis++ {
			u, v := (axis+1)%3, (axis+2)%3
			for side := 0; side < 2; side++ {
				poles := []geom.Vec3{}
				for i := 0; i < 2; i++ {
					for j := 0; j < 2; j++ {
						q := [3]float64{}
						q[axis] = float64(side)
						if second {
							q[u] = float64(j)
							q[v] = float64(i)
						} else {
							q[u] = float64(i)
							q[v] = float64(j)
						}
						x, y, w := q[0], q[1], q[2]-1
						d := geom.Vec3{X: .5, Z: 2}
						if second {
							x += shift
							y += .25
							if diamond {
								x = shift + .5*q[0] - .5*q[1]
								y = -.25 + .5*q[0] + .5*q[1]
							}
							w = q[2]
							d = geom.Vec3{X: -.25, Y: .5, Z: 1.5}
							if sameSide {
								w -= 1
							}
						}
						poles = append(poles, geom.Vec3{X: 2*x + d.X*w, Y: 2*y + d.Y*w, Z: x + y + d.Z*w})
					}
				}
				ku, kv := []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}
				domain := geom.ParameterRectangle{UMax: 1, VMax: 1}
				if second {
					ku = []float64{2, 2, 4, 4}
					kv = []float64{-1, -1, 3, 3}
					domain = geom.ParameterRectangle{UMin: 2, UMax: 4, VMin: -1, VMax: 3}
				}
				p, e := geom.NewBSplineSurface(1, 1, 2, 2, poles, ku, kv, nil)
				if e != nil {
					t.Fatal(e)
				}
				ss = append(ss, p)
				rev = append(rev, (side == 0) != second)
				domains = append(domains, domain)
			}
		}
		s, _, e := SewSplineRectangles(context.Background(), ss, domains, rev, SewOptions{Tolerance: 1e-10})
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	return build(false), build(true)
}
func TestAffineShellInclinedCapContact(t *testing.T) {
	a, b := affineCapAbuttingFixture(t, .5, false)
	r, e := CertifyAffineShellCapContact(context.Background(), a, b, [2]int{5, 4}, AffineShellCapContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	area, sq, _, e := r.Contact().CoplanarArea()
	if e != nil || area.Cmp(big.NewRat(3, 8)) != 0 || sq.Cmp(big.NewRat(27, 8)) != 0 || r.ExactCommonVolume().Sign() != 0 {
		t.Fatal(area, sq, e)
	}
}
func TestAffineShellInclinedCapFuse(t *testing.T) {
	ctx := context.Background()
	a, b := affineCapAbuttingFixture(t, .5, false)
	for order, pair := range [][2]Solid{{a, b}, {b, a}} {
		caps := [2]int{5, 4}
		if order == 1 {
			caps = [2]int{4, 5}
		}
		r, e := CertifyAffineShellCapContact(ctx, pair[0], pair[1], caps, AffineShellCapContactOptions{})
		if e != nil {
			t.Fatal(e)
		}
		f, e := r.Fuse(ctx, AffineShellCapFuseOptions{})
		if e != nil {
			t.Fatal(e)
		}
		solid := f.Solid()
		if f.ExactVolume().Cmp(big.NewRat(25, 2)) != 0 || len(solid.faces) != 32 || len(solid.vertices)-len(solid.edges)+len(solid.faces) != 2 {
			t.Fatal("native mass/topology")
		}
		if e = affineMaterialVertexLinks(ctx, solid); e != nil {
			t.Fatal(e)
		}
		removed := f.RemovedCapUVArea()
		want := [2]*big.Rat{big.NewRat(3, 8), big.NewRat(3, 1)}
		if order == 1 {
			want[0], want[1] = want[1], want[0]
		}
		for k := range removed {
			if removed[k].Cmp(want[k]) != 0 {
				t.Fatal("wrong removed cap region")
			}
		}
		interfaces := f.InterfaceEdges()
		capUses := [2]int{}
		for fi, h := range f.FaceSources() {
			original := pair[h.Owner].faces[h.Face]
			if solid.faces[fi].Surface != original.Surface || solid.faces[fi].Reversed != original.Reversed {
				t.Fatal("replaced original support")
			}
			proof := f.SourceFaceFragments(h.Owner, h.Face)
			if proof == nil {
				t.Fatal("lost fragment proof")
			}
			for _, u := range proof.Cycles()[h.Fragment].Uses {
				if u.SectionEdge >= 0 {
					if u.SectionEdge >= len(interfaces) || interfaces[u.SectionEdge].Owner != h.Owner {
						t.Fatal("lost original cap edge")
					}
				}
			}
			if h.Face == caps[h.Owner] {
				capUses[h.Owner]++
			}
		}
		if capUses[0] == 0 || capUses[1] == 0 {
			t.Fatal("removed whole partial cap")
		}
		n, d, sides := r.SupportingPlane()
		if sides[0] != -sides[1] || sides[0] == 0 {
			t.Fatal("occupancy")
		}
		for owner, s := range pair {
			for _, p := range s.vertices {
				q := new(big.Rat).Neg(d)
				xyz := affineMaterialXYZ(p)
				for k := 0; k < 3; k++ {
					q.Add(q, new(big.Rat).Mul(n[k], xyz[k]))
				}
				if q.Sign()*sides[owner] < 0 {
					t.Fatal("source beyond supporting plane")
				}
			}
		}
		if _, e = r.Fuse(ctx, AffineShellCapFuseOptions{MaxOperations: f.ExactOperations()}); e != nil {
			t.Fatal("exact quota", e)
		}
		if q, e := r.Fuse(ctx, AffineShellCapFuseOptions{MaxOperations: f.ExactOperations() - 1}); q != nil || e == nil {
			t.Fatal("short quota")
		}
		removed[0].SetInt64(99)
		n[0].SetInt64(99)
		if f.RemovedCapUVArea()[0].Cmp(want[0]) != 0 {
			t.Fatal("mutable removed region")
		}
	}
}
func TestAffineShellInclinedCapRejects(t *testing.T) {
	ctx := context.Background()
	a, b := affineCapAbuttingFixture(t, .5, false)
	r, e := CertifyAffineShellCapContact(ctx, a, b, [2]int{5, 4}, AffineShellCapContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, shift := range []float64{1, 2} {
		x, y := affineCapAbuttingFixture(t, shift, false)
		if q, e := CertifyAffineShellCapContact(ctx, x, y, [2]int{5, 4}, AffineShellCapContactOptions{}); q != nil || e == nil {
			t.Fatal("zero-area contact")
		}
	}
	x, y := affineCapAbuttingFixture(t, .5, true)
	if q, e := CertifyAffineShellCapContact(ctx, x, y, [2]int{5, 5}, AffineShellCapContactOptions{}); q != nil || e == nil {
		t.Fatal("same side occupancy")
	}
	if q, e := CertifyAffineShellCapContact(ctx, a, b, [2]int{5, 4}, AffineShellCapContactOptions{MaxOperations: r.ExactOperations() - 1}); q != nil || e == nil {
		t.Fatal("contact quota")
	}
	for _, o := range []AffineShellCapFuseOptions{{MaxOperations: 1}, {MaxFaces: 1}, {Evaluation: geom.SurfacePatchEvaluationOptions{MaxOperations: 1}}} {
		if q, e := r.Fuse(ctx, o); q != nil || e == nil {
			t.Fatal("partial native publication")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if q, e := r.Fuse(canceled, AffineShellCapFuseOptions{}); q != nil || e == nil {
		t.Fatal("cancellation")
	}
}
func TestOCCTAffinePartialCap(t *testing.T) {
	executable := os.Getenv("PLAINSOLID_OCCT_AFFINE_CAP")
	if executable == "" {
		t.Skip("set PLAINSOLID_OCCT_AFFINE_CAP")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, diamond := range []bool{false, true} {
		args := []string{}
		if diamond {
			args = append(args, "diamond")
		}
		data, e := exec.CommandContext(ctx, executable, args...).Output()
		if e != nil {
			t.Fatal(e)
		}
		var q struct {
			Valid                                bool
			Solids                               int
			Volume, CommonVolume, CapAreaSquared float64
		}
		if e = json.Unmarshal(data, &q); e != nil {
			t.Fatal(e)
		}
		a, b := affineCapAbuttingFixtureShape(t, .5, false, diamond)
		r, e := CertifyAffineShellCapContact(ctx, a, b, [2]int{5, 4}, AffineShellCapContactOptions{})
		if e != nil {
			t.Fatal(e)
		}
		f, e := r.Fuse(ctx, AffineShellCapFuseOptions{})
		if e != nil {
			t.Fatal(e)
		}
		v, _ := f.ExactVolume().Float64()
		_, sq, _, e := r.Contact().CoplanarArea()
		if e != nil {
			t.Fatal(e)
		}
		area, _ := sq.Float64()
		if !q.Valid || q.Solids != 1 || math.Abs(q.Volume-v) > 1e-10 || math.Abs(q.CommonVolume) > 1e-12 || math.Abs(q.CapAreaSquared-area) > 1e-10 {
			t.Fatalf("OCCT diamond=%v %+v", diamond, q)
		}
	}
}

func TestAffineShellInclinedDiamondCapFuse(t *testing.T) {
	ctx := context.Background()
	a, b := affineCapAbuttingFixtureShape(t, .5, false, true)
	for order, pair := range [][2]Solid{{a, b}, {b, a}} {
		caps := [2]int{5, 4}
		if order == 1 {
			caps = [2]int{4, 5}
		}
		r, e := CertifyAffineShellCapContact(ctx, pair[0], pair[1], caps, AffineShellCapContactOptions{})
		if e != nil {
			t.Fatal(e)
		}
		_, sq, _, e := r.Contact().CoplanarArea()
		if e != nil || sq.Cmp(big.NewRat(147, 32)) != 0 {
			t.Fatal("diamond common", sq, e)
		}
		f, e := r.Fuse(ctx, AffineShellCapFuseOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if f.ExactVolume().Cmp(big.NewRat(39, 4)) != 0 {
			t.Fatal("diamond volume", f.ExactVolume())
		}
		removed := f.RemovedCapUVArea()
		want := [2]*big.Rat{big.NewRat(7, 16), big.NewRat(7, 1)}
		if order == 1 {
			want[0], want[1] = want[1], want[0]
		}
		for k := range removed {
			if removed[k].Cmp(want[k]) != 0 {
				t.Fatal("diamond removed area")
			}
		}
		solid := f.Solid()
		if len(solid.vertices)-len(solid.edges)+len(solid.faces) != 2 {
			t.Fatal("diamond topology")
		}
		t.Logf("order=%d diamond original triangles=%d", order, len(solid.faces))
	}
}
