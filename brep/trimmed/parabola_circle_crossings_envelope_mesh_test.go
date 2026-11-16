package trimmed

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestCertifiedParabolaCircleCrossingsEnvelope(t *testing.T) {
	ctx := context.Background()
	for axis := range 2 {
		stock := parabolaCircleCrossingsStock(t, axis)
		for order := range 2 {
			for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
				t.Run(fmt.Sprintf("%d/%d/%d", axis, order, op), func(t *testing.T) {
					result, err := CombineParabolaCircleCrossingsSplineGraphs(ctx, stock[0], stock[1+order], stock[2-order], axis, op, 1e-9, ParabolaCircleGraphOptions{})
					if err != nil {
						t.Fatal(err)
					}
					o := ParabolaTwoCirclesMeshOptions{Deflection: .05, NormalTurn: .35}
					out, err := result.TessellateCertified(ctx, o)
					if err != nil {
						t.Fatal(err)
					}
					want := big.NewRat(1071399207, 536870912)
					if op == parametric.Union {
						want = big.NewRat(5429263229, 2684354560)
					}
					intervalWidth := new(big.Rat).Sub(out.SourceVolume[1], out.SourceVolume[0])
					if out.SourceVolume[0].Cmp(want) > 0 || out.SourceVolume[1].Cmp(want) < 0 || intervalWidth.Cmp(big.NewRat(1, 1_000_000_000_000_000_000)) > 0 {
						t.Fatal("changed exact signed integral", out.SourceVolume, want)
					}
					actual, _ := out.SignedDisplayVolume.Float64()
					source, _ := want.Float64()
					distance, _ := out.DistanceUpper.Float64()
					turn, _ := out.SquaredSourceNormalTurn.Float64()
					if !out.Collision.Complete || len(out.Collision.Contacts) != 0 || out.Mesh.ValidateSolid() != nil ||
						len(out.Mesh.Vertices)-len(out.Mesh.Triangles)/2 != 2 || math.Abs(actual-source) > source*.02 ||
						distance <= 0 || distance > o.Deflection || turn <= 0 || math.Sqrt(turn) > o.NormalTurn {
						t.Fatal("invalid signed display or physical certificate", actual, source, distance, turn)
					}
					for _, edge := range out.OriginalEdges {
						source := result.solid.edges[edge.Edge].Curve
						for k, id := range edge.Vertices {
							station, _ := edge.Parameters[k].Float64()
							point, err := source.Eval(station)
							if err != nil || point.Sub(out.Mesh.Vertices[id]).Length() > 2e-14 {
								t.Fatal("display station differs from original model coedge", edge.Edge, station, err)
							}
						}
					}
					for i, node := range result.history.Junctions {
						wantNode := geom.Vec3{X: float64(2*i+1) / 4, Y: 49. / 128, Z: 2}
						if axis == 1 {
							wantNode.X, wantNode.Y = wantNode.Y, wantNode.X
						}
						v := out.JunctionVertices[i]
						if out.OriginalVertices[node.Vertex] != v || out.Mesh.Vertices[v] != wantNode {
							t.Fatal("changed original crossing node", i, wantNode, out.Mesh.Vertices[v])
						}
						for _, id := range node.Rays {
							e := out.OriginalEdges[id].Vertices
							if e[0] != v && e[len(e)-1] != v {
								t.Fatal("lost original ray", i, id)
							}
						}
					}
					verifyParabolaTwoCirclesSourceTurnInChart(t, result.solid, out.CertifiedParabolaTwoCirclesComponentMesh, o.NormalTurn,
						func(p geom.Vec3) geom.Vec2 { return geom.Vec2{X: p.X, Y: p.Y} })
					t.Logf("triangles=%d subdivisions=%d distance=%.12g source turn=%.12g signed volume=%.15g exact source=%.15g self pairs=%d",
						len(out.Mesh.Triangles), out.Subdivisions, distance, math.Sqrt(turn), actual, source, out.Collision.BroadPhasePairs)
					if axis == 1 && order == 1 && op == parametric.Union {
						basic, err := result.TessellateCertified(ctx, ParabolaTwoCirclesMeshOptions{})
						if err != nil || basic.SquaredSourceNormalTurn != nil || basic.SquaredFacetNormalTangent != nil ||
							basic.DistanceUpper.Cmp(ratInset(.05)) > 0 || !basic.Collision.Complete ||
							basic.SignedDisplayVolume.Sign() <= 0 || basic.JunctionVertices != out.JunctionVertices {
							t.Fatal("default request failed the distance-only display", err)
						}
					}
					if axis == 0 && order == 0 && op == parametric.Intersection {
						checkCrossingsEnvelopeDisplayReplay(t, ctx, result, o, out)
					}
				})
			}
		}
	}
}

func checkCrossingsEnvelopeDisplayReplay(t *testing.T, ctx context.Context, result *ParabolaCircleCrossingsEnvelope,
	o ParabolaTwoCirclesMeshOptions, want CertifiedParabolaCircleCrossingsEnvelopeMesh) {
	t.Helper()
	var source bytes.Buffer
	if err := result.WriteJSON(ctx, &source); err != nil {
		t.Fatal(err)
	}
	reopened, err := ReadParabolaCircleCrossingsEnvelopeJSON(ctx, &source, JSONLimits{}, ParabolaCircleGraphOptions{})
	if err != nil {
		t.Fatal(err)
	}
	out, err := reopened.TessellateCertified(ctx, o)
	if err != nil || !reflect.DeepEqual(out, want) {
		t.Fatal("original-source-only reopen changed the certified display", err)
	}
	o.MaxTriangles = 4
	short, err := result.TessellateCertified(ctx, o)
	if !errors.Is(err, parametric.ErrBudget) || !reflect.DeepEqual(short, CertifiedParabolaCircleCrossingsEnvelopeMesh{}) {
		t.Fatal("insufficient triangles published a partial display", err)
	}
	unsupported := o
	unsupported.MaxTriangles, unsupported.NormalAngle = 0, .35
	short, err = result.TessellateCertified(ctx, unsupported)
	if !errors.Is(err, geom.ErrInvalid) || !reflect.DeepEqual(short, CertifiedParabolaCircleCrossingsEnvelopeMesh{}) {
		t.Fatal("unproved facet angle published a partial display", err)
	}
	o.MaxTriangles, o.MaxSubdivisions, o.NormalTurn = 0, 1, .001
	short, err = result.TessellateCertified(ctx, o)
	if !errors.Is(err, parametric.ErrBudget) || !reflect.DeepEqual(short, CertifiedParabolaCircleCrossingsEnvelopeMesh{}) {
		t.Fatal("unproved source turn published a partial display", err)
	}
}
