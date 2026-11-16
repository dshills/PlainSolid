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

func TestCertifiedParabolaCircleApexTangencyCut(t *testing.T) {
	ctx := context.Background()
	options := ParabolaCircleGraphOptions{}
	volume := [2][2]float64{
		{57435031.0 / 1409286144, 1155.0 * math.Pi / 134217728},
		{64080467.0 / 7046430720, 1149.0 * math.Pi / 134217728},
	}
	for axis := range 2 {
		for side, want := range volume {
			t.Run(fmt.Sprintf("%d/%d", axis, side), func(t *testing.T) {
				src := parabolaCircleApexTangencyStock(t, axis, side)
				cut, err := CutParabolaCircleApexTangencySplineGraphs(ctx, src[0], src[2-side], src[1+side], axis, 1e-9, options)
				if err != nil {
					t.Fatal(err)
				}
				var raw, replay bytes.Buffer
				if err := cut.WriteJSON(ctx, &raw); err != nil {
					t.Fatal(err)
				}
				r, err := ReadParabolaCircleApexTangencyCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, options)
				if err != nil {
					t.Fatal(err)
				}
				if err := r.WriteJSON(ctx, &replay); err != nil || !bytes.Equal(raw.Bytes(), replay.Bytes()) {
					t.Fatal("source-only canonical reopen changed Cut receipt", err)
				}
				opts := ParabolaTwoCirclesMeshOptions{NormalTurn: .35, NormalAngle: .2}
				out, err := r.TessellateCertified(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(out.Contact.Source, cut.Contact()) || len(out.Assembly.Contacts) != 1 ||
					out.Assembly.Contacts[0] != out.Contact.Display || out.Assembly.ContactPairs < 1 {
					t.Fatal("changed original contact or admitted an additional contact", out.Contact, out.Assembly)
				}
				second := [2]*big.Rat{big.NewRat(1, 4), big.NewRat(int64((2*side-1)*8), 1)}
				checkApexDisplayJets(t, out.Contact, axis, second)
				var meshes [2]mesh.Mesh
				sum := new(big.Rat)
				vertices := [2]collision.AssemblyVertex{out.Contact.Display.A, out.Contact.Display.B}
				for i, m := range out.Components {
					meshes[i] = m.Mesh
					p := r.components[i]
					if m.Region != [2]int{1 - side, 2}[i] || len(m.Mesh.Vertices)-len(m.Mesh.Triangles)/2 != 2 ||
						m.Mesh.ValidateSolid() != nil || !m.Collision.Complete || len(m.Collision.Contacts) != 0 ||
						vertices[i].Solid != i || vertices[i].Vertex != m.OriginalVertices[r.contact.Vertices[i]] ||
						!exactPointCoordinatesEqual(exactBoundPoint(m.Mesh.Vertices[vertices[i].Vertex]), r.contact.Point) {
						t.Fatal("invalid original positive component or lost shell-local apex", i, m.Region)
					}
					lo, _ := m.SourceVolume[0].Float64()
					hi, _ := m.SourceVolume[1].Float64()
					actual, _ := m.SignedDisplayVolume.Float64()
					distance, _ := m.DistanceUpper.Float64()
					if math.Abs(lo-want[i]) > want[i]*1e-14 || math.Abs(hi-want[i]) > want[i]*1e-14 ||
						math.Abs(actual-want[i]) > want[i]*.02 || distance <= 0 || distance > .05 {
						t.Fatal("wrong original material integral or physical certificate", i, actual, want[i], lo, hi, distance)
					}
					sum.Add(sum, m.SignedDisplayVolume)
					uv := func(p geom.Vec3) geom.Vec2 { return geom.Vec2{X: p.X, Y: p.Y} }
					verifyParabolaTwoCirclesSourceTurnInChart(t, p, m, opts.NormalTurn, uv)
					checkParabolaCircleFacetDisplay(t, p, m, opts.NormalAngle, uv)
					t.Logf("region %d, signed volume %.16g, exact source %.16g, distance %.12g, %d triangles",
						m.Region, actual, want[i], distance, len(m.Mesh.Triangles))
				}
				if sum.Cmp(out.Assembly.SignedVolume) != 0 {
					t.Fatal("assembly signed volume changed")
				}
				t.Logf("one original apex and %d certified incident triangle contacts, %d cross candidates",
					out.Assembly.ContactPairs, out.Assembly.BroadPairs)
				if axis == 0 && side == 1 {
					plain, err := r.TessellateCertified(ctx, ParabolaTwoCirclesMeshOptions{})
					if err != nil || len(plain.Assembly.Contacts) != 1 || plain.Components[0].SquaredFacetNormalTangent != nil ||
						plain.Components[0].SquaredSourceNormalTurn != nil {
						t.Fatal("default distance-only display changed the contact contract", err)
					}
					checkApexCutDisplayRefusals(t, ctx, r, meshes)
				}
			})
		}
	}
}

func checkApexDisplayJets(t *testing.T, contact ParabolaCircleApexMeshContact, free int, second [2]*big.Rat) {
	t.Helper()
	for i, branch := range contact.ModelSecond {
		if contact.DependentSecond[i].Cmp(second[i]) != 0 {
			t.Fatal("changed original free-clock curvature", i, contact.DependentSecond[i])
		}
		for owner, model := range branch {
			first := contact.Owners[owner]
			if first.Parameters[free].Cmp(big.NewRat(1, 2)) != 0 || first.Parameters[1-free].Cmp(big.NewRat(3, 8)) != 0 ||
				first.FreeTangent[free].Cmp(big.NewRat(1, 1)) != 0 || first.FreeTangent[1-free].Sign() != 0 ||
				first.FreeTangent[2].Sign() != 0 || model[1-free].Cmp(second[i]) != 0 ||
				model[free].Sign() != 0 || model[2].Sign() != 0 {
				t.Fatal("changed original roof's first or second model jet", i, owner, first, model)
			}
		}
	}
}

func checkApexCutDisplayRefusals(t *testing.T, ctx context.Context, result *ParabolaCircleApexTangencyCut, meshes [2]mesh.Mesh) {
	t.Helper()
	if _, err := collision.ValidateMeshVertexContactAssembly(ctx, meshes[:], nil,
		collision.VertexContactAssemblyOptions{CrossBVH: true}); !errors.Is(err, collision.ErrInvalid) {
		t.Fatal("undeclared original apex was accepted", err)
	}
	forged := *result
	rays := &forged.contact.Rays[0]
	rays[0], rays[1] = rays[1], rays[0]
	bad, err := forged.TessellateCertified(ctx, ParabolaTwoCirclesMeshOptions{})
	if !errors.Is(err, geom.ErrConvergence) || !reflect.DeepEqual(bad, CertifiedParabolaCircleApexTangencyMesh{}) {
		t.Fatal("a reversed native apex ray ledger published a partial display", err)
	}
	short, err := result.TessellateCertified(ctx, ParabolaTwoCirclesMeshOptions{MaxTriangles: 4})
	if !errors.Is(err, parametric.ErrBudget) || !reflect.DeepEqual(short, CertifiedParabolaCircleApexTangencyMesh{}) {
		t.Fatal("a short triangle budget published a partial apex", err)
	}
}
