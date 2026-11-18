package trimmed

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"reflect"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

func TestCertifiedParabolaCircleCrossingsContactAssembly(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	pi := 15.0 * math.Pi / 4194304
	expected := [2][2]float64{
		{237703427.0/56371445760 - pi, 4135679.0/28185722880 + pi},
		{1263364931.0/56371445760 + pi, 4135679.0/28185722880 - pi},
	}
	for axis := range 2 {
		stock := parabolaCircleCrossingsStock(t, axis)
		for order, want := range expected {
			t.Run(fmt.Sprintf("%d/%d", axis, order), func(t *testing.T) {
				cut, err := CutParabolaCircleCrossingsSplineGraphs(ctx, stock[0], stock[1+order], stock[2-order], axis, 1e-9, ParabolaCircleGraphOptions{})
				if err != nil {
					t.Fatal(err)
				}
				if axis == 1 && order == 1 {
					var sourceOnly bytes.Buffer
					if err := cut.WriteJSON(ctx, &sourceOnly); err != nil {
						t.Fatal(err)
					}
					cut, err = ReadParabolaCircleCrossingsCutJSON(ctx, &sourceOnly, JSONLimits{}, ParabolaCircleGraphOptions{})
					if err != nil {
						t.Fatal(err)
					}
				}
				options := ParabolaTwoCirclesMeshOptions{NormalAngle: .2, NormalTurn: .35}
				if order == 1 {
					options.NormalAngle = .4
					if axis == 0 {
						options.Collision.MaxPairs = 1250000
					}
				}
				display, err := cut.TessellateCertified(ctx, options)
				if err != nil {
					t.Fatal(err)
				}
				var meshes [2]mesh.Mesh
				volume := new(big.Rat)
				for i, m := range display.Components {
					meshes[i] = m.Mesh
					s := cut.components[i]
					if m.Region != [2]int{order, 3 - order}[i] || m.Region != cut.history[i].Region ||
						len(m.Mesh.Vertices)-len(m.Mesh.Triangles)/2 != 2 || m.Mesh.ValidateSolid() != nil ||
						!m.Collision.Complete || len(m.Collision.Contacts) != 0 {
						t.Fatal("invalid original positive shell display", i, m.Region)
					}
					lo, _ := m.SourceVolume[0].Float64()
					hi, _ := m.SourceVolume[1].Float64()
					actual, _ := m.SignedDisplayVolume.Float64()
					distance, _ := m.DistanceUpper.Float64()
					if math.Abs(lo-want[i]) > want[i]*1e-14 || math.Abs(hi-want[i]) > want[i]*1e-14 ||
						math.Abs(actual-want[i]) > want[i]*.02 || distance <= 0 || distance > .05 {
						t.Fatal("wrong exact source integral or display bound", i, lo, hi, actual, want[i], distance)
					}
					volume.Add(volume, m.SignedDisplayVolume)
					uv := func(p geom.Vec3) geom.Vec2 { return geom.Vec2{X: p.X, Y: p.Y} }
					verifyParabolaTwoCirclesSourceTurnInChart(t, s, m, options.NormalTurn, uv)
					checkParabolaCircleFacetDisplay(t, s, m, options.NormalAngle, uv)
					t.Logf("region=%d volume=%.15g exact source=%.15g distance=%.12g triangles=%d self pairs=%d",
						m.Region, actual, want[i], distance, len(m.Mesh.Triangles), m.Collision.BroadPhasePairs)
				}
				if len(display.Assembly.Contacts) != 2 || display.Assembly.ContactPairs < 2 ||
					display.Assembly.SignedVolume.Cmp(volume) != 0 {
					t.Fatal("failed exact signed contact assembly", display.Assembly)
				}
				for i, contact := range display.Contacts {
					if !reflect.DeepEqual(contact.Source, cut.contacts[i]) ||
						!slices.Contains(display.Assembly.Contacts, contact.Display) ||
						contact.Display.A.Solid != 0 || contact.Display.B.Solid != 1 {
						t.Fatal("changed the source node pair", i, contact)
					}
					for j, v := range [2]collision.AssemblyVertex{contact.Display.A, contact.Display.B} {
						component := display.Components[j]
						if v.Vertex != component.OriginalVertices[cut.contacts[i].Vertices[j]] ||
							!exactPointCoordinatesEqual(exactBoundPoint(component.Mesh.Vertices[v.Vertex]), cut.contacts[i].Point) {
							t.Fatal("lost original shell-local node", i, j, v)
						}
					}
				}
				t.Logf("two declared node pairs, %d proved incident triangle contacts, %d cross candidates",
					display.Assembly.ContactPairs, display.Assembly.BroadPairs)
				if axis == 0 && order == 0 {
					checkParabolaCircleCrossingDisplayRefusals(t, ctx, cut, meshes, display.Assembly.Contacts[:1])
				}
			})
		}
	}
}

func checkParabolaCircleCrossingDisplayRefusals(t *testing.T, ctx context.Context, cut *ParabolaCircleCrossingsCut,
	meshes [2]mesh.Mesh, incomplete []collision.AssemblyVertexContact) {
	t.Helper()
	_, err := collision.ValidateMeshVertexContactAssembly(ctx, meshes[:], incomplete, collision.VertexContactAssemblyOptions{
		CrossBVH:   true,
		Validation: collision.LocationOptions{MaxTriangles: collision.MaxTriangles, MaxCollisionPairs: collision.DefaultMaxPairs},
	})
	if !errors.Is(err, collision.ErrInvalid) {
		t.Fatal("assembly accepted a missing original contact", err)
	}
	short, err := cut.TessellateCertified(ctx, ParabolaTwoCirclesMeshOptions{MaxTriangles: 4})
	if !errors.Is(err, parametric.ErrBudget) || short.Assembly.SignedVolume != nil ||
		len(short.Components[0].Mesh.Triangles) != 0 || len(short.Components[1].Mesh.Triangles) != 0 {
		t.Fatal("insufficient triangles published partial display", err)
	}
}
