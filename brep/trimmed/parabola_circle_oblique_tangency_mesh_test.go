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
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

func TestCertifiedParabolaCircleObliqueTangencyCut(t *testing.T) {
	ctx := context.Background()
	graphOptions := ParabolaCircleGraphOptions{}
	volume := [2][2]float64{
		{20734973.0 / 1006632960, 36148125.0 * math.Pi / 35184372088832},
		{156503579.0 / 7046430720, 35851875.0 * math.Pi / 35184372088832},
	}
	for axis := range 2 {
		for side, want := range volume {
			t.Run(fmt.Sprintf("%d/%d", axis, side), func(t *testing.T) {
				src := parabolaCircleObliqueStock(t, axis, originalObliqueCircleFixture(side), false)
				original, err := CutParabolaCircleObliqueTangencySplineGraphs(ctx, src[0], src[2-side], src[1+side],
					axis, 1e-9, graphOptions)
				if err != nil {
					t.Fatal(err)
				}
				var raw, same bytes.Buffer
				if err := original.WriteJSON(ctx, &raw); err != nil {
					t.Fatal(err)
				}
				r, err := ReadParabolaCircleObliqueTangencyCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, graphOptions)
				if err != nil {
					t.Fatal(err)
				}
				if err := r.WriteJSON(ctx, &same); err != nil || !bytes.Equal(raw.Bytes(), same.Bytes()) {
					t.Fatal("source-only reopen changed the native receipt", err)
				}
				o := ParabolaTwoCirclesMeshOptions{NormalTurn: .35, NormalAngle: .2}
				out, err := r.TessellateCertified(ctx, o)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(out.Contact.Source, original.Contact()) || len(out.Assembly.Contacts) != 1 ||
					out.Assembly.Contacts[0] != out.Contact.Display || out.Assembly.ContactPairs == 0 {
					t.Fatal("lost the single original oblique contact", out.Contact, out.Assembly)
				}
				checkObliqueDisplayJets(t, out.Contact, axis, side)
				var meshes [2]mesh.Mesh
				total := new(big.Rat)
				contactPoint := geom.Vec3{X: .5, Y: .5, Z: 2}
				pair := [2]collision.AssemblyVertex{out.Contact.Display.A, out.Contact.Display.B}
				for i, m := range out.Components {
					s := r.cut.components[i]
					vertex := pair[i].Vertex
					if m.Region != [2]int{1 - side, 2}[i] || m.Mesh.ValidateSolid() != nil || !m.Collision.Complete ||
						len(m.Collision.Contacts) != 0 || len(m.Mesh.Vertices)-len(m.Mesh.Triangles)/2 != 2 ||
						pair[i].Solid != i || m.OriginalVertices[out.Contact.Source.Vertices[i]] != vertex || m.Mesh.Vertices[vertex] != contactPoint {
						t.Fatal("invalid independently indexed shell or tangent vertex", i, m.Region)
					}
					for direction, ray := range out.Contact.Source.Rays[i] {
						e := m.OriginalEdges[ray]
						point := e.Vertices[1]
						if e.Vertices[len(e.Vertices)-1] == vertex {
							point = e.Vertices[len(e.Vertices)-2]
						} else if e.Vertices[0] != vertex {
							t.Fatal("lost an exact original projected ray", i, ray)
						}
						if station := exactBoundPoint(m.Mesh.Vertices[point])[axis]; station.Cmp(big.NewRat(1, 2)) != 2*direction-1 {
							t.Fatal("first displayed segment lost its raw source direction", i, direction)
						}
					}
					lo, _ := m.SourceVolume[0].Float64()
					hi, _ := m.SourceVolume[1].Float64()
					signed, _ := m.SignedDisplayVolume.Float64()
					distance, _ := m.DistanceUpper.Float64()
					if math.Abs(lo-want[i]) > want[i]*1e-14 || math.Abs(hi-want[i]) > want[i]*1e-14 ||
						math.Abs(signed-want[i]) > want[i]*.02 || distance <= 0 || distance > .05 {
						t.Fatal("wrong exact source volume or physical certificate", i, lo, hi, signed, want[i], distance)
					}
					uv := func(p geom.Vec3) geom.Vec2 { return geom.Vec2{X: p.X, Y: p.Y} }
					verifyParabolaTwoCirclesSourceTurnInChart(t, s, m, o.NormalTurn, uv)
					checkParabolaCircleFacetDisplay(t, s, m, o.NormalAngle, uv)
					t.Logf("region %d, %d triangles, distance %.12g, signed volume %.17g, source %.17g",
						m.Region, len(m.Mesh.Triangles), distance, signed, want[i])
					total.Add(total, m.SignedDisplayVolume)
					meshes[i] = m.Mesh
				}
				if out.Assembly.SignedVolume.Cmp(total) != 0 {
					t.Fatal("signed assembly volume changed")
				}
				t.Logf("one native tangent vertex pair, %d accepted triangle contacts, %d cross candidates",
					out.Assembly.ContactPairs, out.Assembly.BroadPairs)
				if axis == 0 && side == 1 {
					checkObliqueDisplayRefusals(t, ctx, r, meshes)
				}
			})
		}
	}
}

func checkObliqueDisplayJets(t *testing.T, contact ParabolaCircleObliqueMeshContact, axis, side int) {
	t.Helper()
	if contact.Slope.Cmp(big.NewRat(3, 4)) != 0 {
		t.Fatal("changed the exact original nonzero slope", contact.Slope)
	}
	wantSecond := [2]*big.Rat{big.NewRat(1, 4), big.NewRat(int64(50*side-25), 1)}
	for branch, model := range contact.ModelSecond {
		if contact.DependentSecond[branch].Cmp(wantSecond[branch]) != 0 {
			t.Fatal("wrong original free-coordinate second derivative", branch)
		}
		for owner, lift := range model {
			source := contact.Owners[owner]
			if source.Parameters[0].Cmp(big.NewRat(1, 2)) != 0 || source.Parameters[1].Cmp(big.NewRat(1, 2)) != 0 ||
				source.Tangent[axis].Cmp(big.NewRat(1, 1)) != 0 || source.Tangent[1-axis].Cmp(contact.Slope) != 0 ||
				source.Tangent[2].Sign() != 0 || lift[axis].Sign() != 0 || lift[1-axis].Cmp(wantSecond[branch]) != 0 ||
				lift[2].Sign() != 0 {
				t.Fatal("wrong independently lifted oblique source jets", branch, owner, source, lift)
			}
		}
	}
}

func checkObliqueDisplayRefusals(t *testing.T, ctx context.Context, r *ParabolaCircleObliqueTangencyCut, meshes [2]mesh.Mesh) {
	t.Helper()
	if _, err := collision.ValidateMeshVertexContactAssembly(ctx, meshes[:], nil,
		collision.VertexContactAssemblyOptions{CrossBVH: true,
			Validation: collision.LocationOptions{MaxTriangles: collision.MaxTriangles, MaxCollisionPairs: collision.DefaultMaxPairs}}); !errors.Is(err, collision.ErrInvalid) {
		t.Fatal("physical contact passed without its declaration", err)
	}
	copyOfCut := *r.cut
	rays := &copyOfCut.contact.Rays[0]
	rays[0], rays[1] = rays[1], rays[0]
	forged := ParabolaCircleObliqueTangencyCut{cut: &copyOfCut}
	out, err := forged.TessellateCertified(ctx, ParabolaTwoCirclesMeshOptions{})
	if !errors.Is(err, geom.ErrConvergence) || !reflect.DeepEqual(out, CertifiedParabolaCircleObliqueTangencyMesh{}) {
		t.Fatal("a changed raw ray ledger published a partial assembly", err)
	}
	out, err = r.TessellateCertified(ctx, ParabolaTwoCirclesMeshOptions{MaxTriangles: 4})
	if !errors.Is(err, parametric.ErrBudget) || !reflect.DeepEqual(out, CertifiedParabolaCircleObliqueTangencyMesh{}) {
		t.Fatal("a short triangle budget published a partial assembly", err)
	}
}
