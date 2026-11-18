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
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestCertifiedParabolaCircleApexTangencyEnvelope(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	pi := math.Pi / 134217728
	volume := [2][2]float64{
		{2876007319.0/1409286144 + 1155*pi, 14065067533.0/7046430720 - 1155*pi},
		{2844557431.0/1409286144 + 1149*pi, 14028780973.0/7046430720 - 1149*pi},
	}
	for axis := range 2 {
		for side, want := range volume {
			src := parabolaCircleApexTangencyStock(t, axis, side)
			for order := range 2 {
				for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
					t.Run(fmt.Sprintf("%d/%d/%d/%d", axis, side, order, op), func(t *testing.T) {
						r, err := CombineParabolaCircleApexTangencySplineGraphs(ctx, src[0], src[1+order], src[2-order],
							axis, op, 1e-9, ParabolaCircleGraphOptions{})
						if err != nil {
							t.Fatal(err)
						}
						o := ParabolaTwoCirclesMeshOptions{NormalAngle: .2, NormalTurn: .35, RelativeVolumeTolerance: .001}
						m, err := r.TessellateCertified(ctx, o)
						if err != nil {
							t.Fatal(err)
						}
						checkApexEnvelopeMesh(t, r, m, want[op], o)
						if axis == 0 && side == 0 && order == 0 && op == parametric.Intersection {
							checkApexEnvelopeDisplayReplay(t, ctx, r, o, m)
							checkApexEnvelopeSourceGate(t, ctx, r, side)
						}
					})
				}
			}
		}
	}
}

func checkApexEnvelopeMesh(t *testing.T, r *ParabolaCircleApexTangencyEnvelope, m CertifiedParabolaCircleApexEnvelopeMesh,
	want float64, o ParabolaTwoCirclesMeshOptions) {
	t.Helper()
	h := r.History()
	if m.Region != -1 || m.Mesh.ValidateSolid() != nil || !m.Collision.Complete || len(m.Collision.Contacts) != 0 ||
		len(m.Mesh.Vertices)-len(m.Mesh.Triangles)/2 != 2 || m.ApexVertex != m.OriginalVertices[h.Vertex] ||
		m.Mesh.Vertices[m.ApexVertex] != r.solid.vertices[h.Vertex] || m.Rays != h.Rays || m.Bridge != h.Bridge {
		t.Fatal("lost closed original apex assembly", m.ApexVertex, m.Rays, m.Bridge)
	}
	for _, rays := range m.Rays {
		for _, id := range rays {
			e := m.OriginalEdges[id].Vertices
			if e[0] != m.ApexVertex && e[len(e)-1] != m.ApexVertex {
				t.Fatal("lost a source ray at the common vertex", id)
			}
		}
	}
	bridge := m.OriginalEdges[m.Bridge]
	if len(bridge.Vertices) < 3 || bridge.Parameters[0].Sign() != 0 ||
		bridge.Parameters[len(bridge.Parameters)-1].Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("display lost the original sampled same-roof bridge", bridge)
	}
	lo, _ := m.SourceVolume[0].Float64()
	hi, _ := m.SourceVolume[1].Float64()
	v, _ := m.SignedDisplayVolume.Float64()
	distance, _ := m.DistanceUpper.Float64()
	if math.Abs(lo-want) > 1e-14 || math.Abs(hi-want) > 1e-14 || math.Abs(v-want) > want*o.RelativeVolumeTolerance ||
		distance <= 0 || distance > .05 {
		t.Fatal("incorrect physical volume or distance", lo, hi, v, want, distance)
	}
	chart := func(p geom.Vec3) geom.Vec2 { return geom.Vec2{X: p.X, Y: p.Y} }
	checkParabolaCircleFacetDisplay(t, r.solid, m.CertifiedParabolaTwoCirclesComponentMesh, o.NormalAngle, chart)
	verifyParabolaTwoCirclesSourceTurnInChart(t, r.solid, m.CertifiedParabolaTwoCirclesComponentMesh, o.NormalTurn, chart)
	t.Logf("%d triangles, %d shared stations, distance %.12g, signed volume %.15g, exact source %.15g",
		len(m.Mesh.Triangles), m.Subdivisions, distance, v, want)
}

func checkApexEnvelopeDisplayReplay(t *testing.T, ctx context.Context, r *ParabolaCircleApexTangencyEnvelope,
	o ParabolaTwoCirclesMeshOptions, want CertifiedParabolaCircleApexEnvelopeMesh) {
	t.Helper()
	var raw, same bytes.Buffer
	if err := r.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	reopened, err := ReadParabolaCircleApexTangencyEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, ParabolaCircleGraphOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := reopened.TessellateCertified(ctx, o)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("source-only reopen changed the certified envelope", err)
	}
	if err := reopened.WriteJSON(ctx, &same); err != nil || !bytes.Equal(raw.Bytes(), same.Bytes()) {
		t.Fatal("source-only reopen changed native bytes", err)
	}
	short, err := r.TessellateCertified(ctx, ParabolaTwoCirclesMeshOptions{MaxTriangles: 4})
	if !errors.Is(err, parametric.ErrBudget) || !reflect.DeepEqual(short, CertifiedParabolaCircleApexEnvelopeMesh{}) {
		t.Fatal("insufficient triangle budget published a partial envelope", err)
	}
}

func checkApexEnvelopeSourceGate(t *testing.T, ctx context.Context, r *ParabolaCircleApexTangencyEnvelope, side int) {
	t.Helper()
	p, err := proveParabolaCircleApexPartition(ctx, r.sources, r.axis, r.tolerance, ParabolaCircleGraphOptions{})
	if err != nil {
		t.Fatal(err)
	}
	regions, _, err := parabolaCircleApexEnvelopeRegions(p)
	if err != nil {
		t.Fatal(err)
	}
	loop := authenticatedParabolaCircleTangentLoop(regions[side].loop)
	face := r.solid.faces[r.history.RoofFaces[side]]
	curves := make([]PCurve, len(face.Outer))
	for i, use := range face.Outer {
		curves[i], err = mixedStoredUseCurveContext(ctx, use, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
	}
	plane, err := parabolaCircleMeshUVPlane(ctx)
	if err != nil {
		t.Fatal(err)
	}
	opts := MixedExactAffineMeshOptions{Topology: trimvalidate.Options{MaxWork: 200000}}
	if m, err := TessellateMixedExactAffineRegionCertified(ctx, plane, curves, nil, opts); err == nil || len(m.Mesh.Triangles) != 0 {
		t.Fatal("ordinary trim gate accepted the uncertified cusp", err)
	}
	for i, c := range curves {
		if face.Outer[i].Curve == nil {
			continue
		}
		controls := c.(ExactQuadraticUV).control
		controls[1][1-r.axis] = new(big.Rat).Add(controls[1][1-r.axis], big.NewRat(1, 1048576))
		curves[i], err = newExactQuadraticUV(controls, false)
		if err != nil {
			t.Fatal(err)
		}
		m, err := tessellateMixedExactAffineRegionWithTangentLoop(ctx, plane, curves, nil, nil, opts, loop)
		if !errors.Is(err, geom.ErrConvergence) || len(m.Mesh.Triangles) != 0 {
			t.Fatal("a changed original cusp coefficient passed authentication", err)
		}
		return
	}
	t.Fatal("fixture has no original curved cusp coedge")
}
