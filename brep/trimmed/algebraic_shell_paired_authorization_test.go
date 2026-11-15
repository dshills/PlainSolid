package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func pairedAffineShellFixture(t *testing.T) (Solid, Solid) {
	build := func(second bool) Solid {
		ss := []*geom.BSplineSurface{}
		rev := []bool{}
		for axis := 0; axis < 3; axis++ {
			u, v := (axis+1)%3, (axis+2)%3
			for side := 0; side < 2; side++ {
				poles := []geom.Vec3{}
				for i := 0; i < 2; i++ {
					for j := 0; j < 2; j++ {
						q := [3]float64{}
						q[axis] = float64(side)
						q[u] = float64(i)
						q[v] = float64(j)
						x, y, z := 2*q[0]-1, 2*q[1]-1, 2*q[2]-2
						if !second {
							x, z = 4*q[0]-2, 4*q[2]-4
						}
						if second {
							x, y, z = x+.25*z, y+.5*x-.125*z, z+x
						}
						poles = append(poles, geom.Vec3{X: x, Y: y, Z: z})
					}
				}
				p, e := geom.NewBSplineSurface(1, 1, 2, 2, poles, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
				if e != nil {
					t.Fatal(e)
				}
				ss = append(ss, p)
				rev = append(rev, side == 0)
			}
		}
		s, _, e := SewSplineRectangles(context.Background(), ss, make([]geom.ParameterRectangle, len(ss)), rev, SewOptions{Tolerance: 1e-10})
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	return build(false), build(true)
}
func TestAffineShellPairedImprintAuthorization(t *testing.T) {
	a, b := pairedAffineShellFixture(t)
	for _, swapped := range []bool{false, true} {
		x, y := a, b
		if swapped {
			x, y = b, a
		}
		r, e := AuthorizeAffineSplineShellPairedImprint(context.Background(), x, y, AffineShellPairedImprintOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if r.CompleteFacePairs() != 36 || len(r.Vertices()) != 9 || len(r.Edges()) != 10 || len(r.PairedNodes()) != 2 {
			t.Fatal("incomplete contact graph")
		}
		valences := map[int]int{}
		vs := r.Vertices()
		for _, node := range r.PairedNodes() {
			valences[len(node.IncidentEdges)]++
			if vs[node.Vertex][0].Sign() != 0 || vs[node.Vertex][2].Sign() != 0 || new(big.Rat).Abs(vs[node.Vertex][1]).Cmp(big.NewRat(1, 1)) != 0 {
				t.Fatal("wrong independent edge crossing")
			}
			expected := map[[2]int]bool{}
			for _, p := range node.IncidentPairs {
				expected[p] = true
			}
			if len(expected) != 4 {
				t.Fatal("four pair star")
			}
			for _, i := range node.IncidentEdges {
				edge := r.Edges()[i]
				if !expected[edge.Faces] || (edge.Vertices[0] != node.Vertex && edge.Vertices[1] != node.Vertex) {
					t.Fatal("extra germ")
				}
			}
			for owner, occ := range node.Owners {
				if occ.Operand != owner || occ.Incidents[0].Face == occ.Incidents[1].Face {
					t.Fatal("source incidence")
				}
				same, _, e := occ.ModelEvent.CompareModelPoint(context.Background(), node.Owners[1-owner].ModelEvent, geom.ExactCurvePlaneEventOptions{})
				if e != nil || !same {
					t.Fatal("original model identity", e)
				}
			}
		}
		if valences[2] != 1 || valences[4] != 1 {
			t.Fatal("invented pairing", valences)
		}
		rows := r.PairedNodes()
		rows[0].Owners[0].Edge = -1
		rows[0].IncidentEdges[0] = -1
		vs[0][0].SetInt64(999)
		if r.PairedNodes()[0].Owners[0].Edge < 0 || r.PairedNodes()[0].IncidentEdges[0] < 0 || r.Vertices()[0][0].Cmp(big.NewRat(999, 1)) == 0 {
			t.Fatal("mutable authorization")
		}
		if !swapped {
			for _, w := range []int{r.ExactOperations(), r.ExactOperations() - 1} {
				out, e := AuthorizeAffineSplineShellPairedImprint(context.Background(), a, b, AffineShellPairedImprintOptions{MaxWork: w})
				if w == r.ExactOperations() {
					if e != nil {
						t.Fatal("exact budget", e)
					}
				} else if e == nil || out != nil {
					t.Fatal("partial authorization")
				}
			}
		}
	}
}
func TestAffineShellPairedImprintReject(t *testing.T) {
	a, b := pairedAffineShellFixture(t)
	broken := a
	broken.faces = append([]Face(nil), a.faces...)
	broken.faces[5].Outer = append([]Use(nil), a.faces[5].Outer...)
	broken.faces[5].Outer[0].UVStart.X += .125
	duplicate := a
	duplicate.faces = append(append([]Face(nil), a.faces...), a.faces[5])
	missing := a
	missing.faces = append([]Face(nil), a.faces[:5]...)
	for _, bad := range []Solid{broken, duplicate, missing} {
		if out, e := AuthorizeAffineSplineShellPairedImprint(context.Background(), bad, b, AffineShellPairedImprintOptions{}); e == nil || out != nil {
			t.Fatal("inconsistent source ownership")
		}
	}
	if out, e := AuthorizeAffineSplineShellPairedImprint(context.Background(), a, a, AffineShellPairedImprintOptions{}); e == nil || out != nil {
		t.Fatal("coincident domains accepted")
	}
	for _, o := range []AffineShellPairedImprintOptions{{MaxFacePairs: 35}, {MaxVertices: 8}, {MaxWork: 1}} {
		if out, e := AuthorizeAffineSplineShellPairedImprint(context.Background(), a, b, o); e == nil || out != nil {
			t.Fatal("partial pair coverage")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out, e := AuthorizeAffineSplineShellPairedImprint(ctx, a, b, AffineShellPairedImprintOptions{}); e != context.Canceled || out != nil {
		t.Fatal(out, e)
	}
}
