package trimmed

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestCertifiedParabolaCircleCrossingsEnvelopeFacetAngle(t *testing.T) {
	ctx := context.Background()
	for axis := range 2 {
		stock := parabolaCircleCrossingsStock(t, axis)
		for order := range 2 {
			for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
				t.Run(fmt.Sprintf("%d/%d/%d", axis, order, op), func(t *testing.T) {
					r, err := CombineParabolaCircleCrossingsSplineGraphs(ctx, stock[0], stock[1+order], stock[2-order],
						axis, op, 1e-9, ParabolaCircleGraphOptions{})
					if err != nil {
						t.Fatal(err)
					}
					o := ParabolaTwoCirclesMeshOptions{NormalAngle: .1, NormalTurn: .35, MaxSubdivisions: 32}
					out, err := r.TessellateCertified(ctx, o)
					if err != nil {
						t.Fatal(err)
					}
					if out.SignedDisplayVolume.Sign() <= 0 || out.DistanceUpper.Cmp(ratInset(.05)) > 0 ||
						len(out.Mesh.Vertices)-len(out.Mesh.Triangles)/2 != 2 || out.Mesh.ValidateSolid() != nil {
						t.Fatal("facet certification changed the signed closed envelope")
					}
					for i, node := range r.history.Junctions {
						if out.JunctionVertices[i] != out.OriginalVertices[node.Vertex] ||
							out.Mesh.Vertices[out.JunctionVertices[i]] != r.solid.vertices[node.Vertex] {
							t.Fatal("changed the exact original crossing node", i)
						}
					}
					chart := func(p geom.Vec3) geom.Vec2 { return geom.Vec2{X: p.X, Y: p.Y} }
					checkParabolaCircleFacetDisplay(t, r.solid, out.CertifiedParabolaTwoCirclesComponentMesh, o.NormalAngle, chart)
					verifyParabolaTwoCirclesSourceTurnInChart(t, r.solid, out.CertifiedParabolaTwoCirclesComponentMesh, o.NormalTurn, chart)
					if axis == 0 && order == 0 && op == parametric.Intersection {
						checkCrossingsEnvelopeFacetRefusal(t, ctx, r, o)
					}
				})
			}
		}
	}
}

func checkCrossingsEnvelopeFacetRefusal(t *testing.T, ctx context.Context, r *ParabolaCircleCrossingsEnvelope, o ParabolaTwoCirclesMeshOptions) {
	t.Helper()
	o.NormalAngle, o.MaxSubdivisions = .2, 16
	coarse, err := r.TessellateCertified(ctx, o)
	if err != nil || coarse.SquaredFacetNormalTangent == nil {
		t.Fatal("16 segments failed to prove a .2-radian cone", err)
	}
	// The original-angle request remains unproved on this coarser grid.
	o.NormalAngle = .115
	out, err := r.TessellateCertified(ctx, o)
	if !errors.Is(err, parametric.ErrBudget) || !reflect.DeepEqual(out, CertifiedParabolaCircleCrossingsEnvelopeMesh{}) {
		t.Fatal("unproved facet deviation published a partial display", err)
	}
}
