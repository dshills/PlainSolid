//go:build research

package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestBuildProjectiveSplineRoofDifferenceCancelsBaseAndPreservesBothOriginalRoofs(t *testing.T) {
	ctx := context.Background()
	as, bs := projectiveImprintShellFixture(t, true, false), projectiveImprintShellFixture(t, false, true)
	a, b := as.faces[5], bs.faces[5]
	for _, axes := range [][2]int{{0, 1}, {1, 0}} {
		for i, faces := range [][2]Face{{a, b}, {b, a}} {
			q, err := BuildProjectiveSplineRoofDifference(ctx, faces[0], faces[1], axes, 0, ProjectiveRoofSolidOptions{})
			if err != nil || !q.Complete {
				t.Fatal("both original Cut orders must close without a collapsed wall edge", axes, i, err)
			}
			s := q.Solid
			if len(s.faces) != 5 || len(s.edges) != 9 || len(s.vertices) != 6 ||
				len(q.Walls) != 3 || len(q.FloorSources) != 3 || q.BaseFace != -1 || q.Roofs != [2]int{0, 1} {
				t.Fatal("bounded Cut topology changed or the common base leaked", axes, i, q)
			}
			if s.faces[0].Surface != faces[0].Surface || s.faces[1].Surface != faces[1].Surface ||
				s.faces[0].Reversed != faces[0].Reversed || s.faces[1].Reversed == faces[1].Reversed {
				t.Fatal("Cut must retain both original lifts and reverse only the floor", axes, i)
			}
			triangles := 0
			for k, wall := range q.Walls {
				if wall.Source.OriginalEdge < 0 || q.FloorSources[k].OriginalEdge < 0 {
					t.Fatal("original wall and floor owner ancestry was lost", wall, q.FloorSources[k])
				}
				if len(s.faces[wall.Face].Outer) == 3 {
					triangles++
				}
			}
			if triangles != 2 {
				t.Fatal("both shared physical seam terminals need walls without collapsed edges", triangles)
			}
			m, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			wantX := 5.0 / 6
			if i == 1 {
				wantX = 1.0 / 6
			}
			if err != nil || math.Abs(m.Volume-1.0/8) > 1e-10 || math.Abs(m.Centroid.X-wantX) > 1e-10 ||
				math.Abs(m.Centroid.Y-.5) > 1e-10 {
				t.Fatal("retained Cut material disagrees with independent integrals", axes, i, m, err)
			}
			var native, replay bytes.Buffer
			if err := WriteJSON(&native, s); err != nil {
				t.Fatal("write original Cut shell", axes, i, err)
			}
			reopened, err := ReadJSONContext(ctx, bytes.NewReader(native.Bytes()), JSONLimits{})
			if err != nil || len(reopened.faces) != 5 {
				t.Fatal("reopen original Cut shell", axes, i, err)
			}
			if err := WriteJSON(&replay, reopened); err != nil || !bytes.Equal(native.Bytes(), replay.Bytes()) {
				t.Fatal("native Cut shell and original exact trims must replay identically", axes, i, err)
			}
		}
	}
	if q, err := BuildProjectiveSplineRoofDifference(ctx, a, b, [2]int{0, 1}, .5, ProjectiveRoofSolidOptions{}); !errors.Is(err, geom.ErrConvergence) || q.Complete || len(q.Solid.faces) != 0 {
		t.Fatal("unproved positive original stock cannot publish a Cut", q, err)
	}
}

func TestBuildProjectiveSplineRoofDifferenceCertifiesClosedDisplay(t *testing.T) {
	ctx := context.Background()
	a, b := projectiveImprintShellFixture(t, false, true), projectiveImprintShellFixture(t, true, false)
	q, err := BuildProjectiveSplineRoofDifference(ctx, a.faces[5], b.faces[5], [2]int{0, 1}, 0, ProjectiveRoofSolidOptions{})
	if err != nil || !q.Complete {
		t.Fatal("closed Cut", err)
	}
	o := SolidExactQuadraticMeshOptions{Shell: ExactQuadraticShellMeshOptions{Join: ExactQuadraticFaceJoinOptions{
		Face: ExactQuadraticRegionMeshOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .04}}},
	}}}
	certified, err := q.Solid.TessellateExactQuadraticShellCertified(ctx, o)
	if err != nil || len(certified.Mesh.Triangles) == 0 || certified.ModelEdgeUsesCertified != 18 ||
		len(certified.OriginalEdges) != 9 || len(certified.OriginalVertices) != 6 {
		t.Fatal("closed display must retain and prove every original wall and roof coedge", err)
	}
}
