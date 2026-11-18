//go:build research

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
	"plainsolid/geom"
)

func TestCertifiedParabolaCircleObliqueTangencyEnvelope(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	pi := math.Pi / 35184372088832
	volume := [2][2]float64{
		{2034000893.0/1006632960 + 36148125*pi, 2000370691.0/1006632960 - 36148125*pi},
		{14175219227.0/7046430720 + 35851875*pi, 13936357861.0/7046430720 - 35851875*pi},
	}
	for axis := range 2 {
		for side, want := range volume {
			src := parabolaCircleObliqueStock(t, axis, originalObliqueCircleFixture(side), false)
			for order := range 2 {
				for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
					t.Run(fmt.Sprintf("%d/%d/%d/%d", axis, side, order, op), func(t *testing.T) {
						r, err := CombineParabolaCircleObliqueTangencySplineGraphs(ctx, src[0], src[1+order], src[2-order],
							axis, op, 1e-9, ParabolaCircleGraphOptions{})
						if err != nil {
							t.Fatal(err)
						}
						o := ParabolaTwoCirclesMeshOptions{NormalAngle: .2, NormalTurn: .35, RelativeVolumeTolerance: .001}
						m, err := r.TessellateCertified(ctx, o)
						if err != nil {
							t.Fatal(err)
						}
						checkObliqueEnvelopeMesh(t, r, m, want[op], o)
						checkObliqueEnvelopeBridge(t, r, m, axis, side)
						checkObliqueDisplayJets(t, ParabolaCircleObliqueMeshContact{
							Slope: m.Slope, Owners: m.Owners, DependentSecond: m.DependentSecond, ModelSecond: m.ModelSecond,
						}, axis, side)
						if axis == 0 && side == 0 && order == 0 && op == parametric.Intersection {
							checkObliqueEnvelopeReplayAndRefusal(t, ctx, r, o, m)
						}
					})
				}
			}
		}
	}
}

func checkObliqueEnvelopeMesh(t *testing.T, r *ParabolaCircleObliqueTangencyEnvelope,
	m CertifiedParabolaCircleObliqueEnvelopeMesh, want float64, o ParabolaTwoCirclesMeshOptions) {
	t.Helper()
	h := r.History()
	point := geom.Vec3{X: .5, Y: .5, Z: 2}
	if m.Region != -1 || m.Mesh.ValidateSolid() != nil || !m.Collision.Complete || len(m.Collision.Contacts) != 0 ||
		len(m.Mesh.Vertices)-len(m.Mesh.Triangles)/2 != 2 || m.ContactVertex != m.OriginalVertices[h.Vertex] ||
		m.Mesh.Vertices[m.ContactVertex] != point || m.Rays != h.Rays || m.Bridge != h.Bridge {
		t.Fatal("lost closed original oblique shell or native contact", m.ContactVertex, m.Rays, m.Bridge)
	}
	for _, rays := range m.Rays {
		for _, id := range rays {
			e := m.OriginalEdges[id].Vertices
			if e[0] != m.ContactVertex && e[len(e)-1] != m.ContactVertex {
				t.Fatal("lost an original oblique ray at its common vertex", id)
			}
		}
	}
	lo, _ := m.SourceVolume[0].Float64()
	hi, _ := m.SourceVolume[1].Float64()
	v, _ := m.SignedDisplayVolume.Float64()
	distance, _ := m.DistanceUpper.Float64()
	if math.Abs(lo-want) > 1e-14 || math.Abs(hi-want) > 1e-14 || math.Abs(v-want) > want*o.RelativeVolumeTolerance ||
		distance <= 0 || distance > .05 {
		t.Fatal("incorrect original material volume or physical distance", lo, hi, v, want, distance)
	}
	chart := func(p geom.Vec3) geom.Vec2 { return geom.Vec2{X: p.X, Y: p.Y} }
	s := r.Boundary()
	checkParabolaCircleFacetDisplay(t, s, m.CertifiedParabolaTwoCirclesComponentMesh, o.NormalAngle, chart)
	verifyParabolaTwoCirclesSourceTurnInChart(t, s, m.CertifiedParabolaTwoCirclesComponentMesh, o.NormalTurn, chart)
	t.Logf("%d triangles, %d shared stations, distance %.12g, signed volume %.15g, exact source %.15g",
		len(m.Mesh.Triangles), m.Subdivisions, distance, v, want)
}

func checkObliqueEnvelopeBridge(t *testing.T, r *ParabolaCircleObliqueTangencyEnvelope,
	m CertifiedParabolaCircleObliqueEnvelopeMesh, axis, side int) {
	t.Helper()
	far := geom.Vec2{X: 19. / 32, Y: 3. / 8}
	wall := geom.Vec2{X: 7. / 8, Y: 0}
	if side == 1 {
		far = geom.Vec2{X: 13. / 32, Y: 5. / 8}
		wall = geom.Vec2{X: 1. / 8, Y: 1}
	}
	if axis == 1 {
		far.X, far.Y = far.Y, far.X
		wall.X, wall.Y = wall.Y, wall.X
	}
	if m.BridgeUV != ([2]geom.Vec2{far, wall}) && m.BridgeUV != ([2]geom.Vec2{wall, far}) {
		t.Fatal("moved original far circle point or stock station", m.BridgeUV, far, wall)
	}
	e := m.OriginalEdges[m.Bridge]
	if len(e.Vertices) < 3 || e.Parameters[0].Sign() != 0 || e.Parameters[len(e.Parameters)-1].Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("lost the original sampled bridge", e)
	}
	model := r.envelope.solid.edges[m.Bridge].Curve
	for k, id := range e.Vertices {
		parameter, _ := e.Parameters[k].Float64()
		p, err := model.Eval(parameter)
		if err != nil || p.Sub(m.Mesh.Vertices[id]).Length() > 2e-14 {
			t.Fatal("bridge sample escaped the original owner curve", k, parameter, err)
		}
		uv := exactBoundPoint(m.Mesh.Vertices[id])
		du, dv := new(big.Rat).Sub(uv[axis], big.NewRat(1, 2)), new(big.Rat).Sub(uv[1-axis], big.NewRat(1, 2))
		if du.Add(du, dv.Mul(dv, m.Slope)).Sign() != 0 {
			t.Fatal("a bridge station ceased to be normal to the original tangent", k)
		}
	}
}

func checkObliqueEnvelopeReplayAndRefusal(t *testing.T, ctx context.Context, r *ParabolaCircleObliqueTangencyEnvelope,
	o ParabolaTwoCirclesMeshOptions, want CertifiedParabolaCircleObliqueEnvelopeMesh) {
	t.Helper()
	var raw, same bytes.Buffer
	if err := r.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	reopened, err := ReadParabolaCircleObliqueTangencyEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, ParabolaCircleGraphOptions{})
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
	copyOfEnvelope := *r.envelope
	copyOfEnvelope.solid.edges = slices.Clone(r.envelope.solid.edges)
	edge := &copyOfEnvelope.solid.edges[copyOfEnvelope.history.Bridge]
	model := edge.Curve.(surfaceTrimCurve)
	model.b.X = math.Nextafter(model.b.X, 1)
	edge.Curve = model
	forged := ParabolaCircleObliqueTangencyEnvelope{envelope: &copyOfEnvelope}
	bad, err := forged.TessellateCertified(ctx, o)
	if !errors.Is(err, geom.ErrConvergence) || !reflect.DeepEqual(bad, CertifiedParabolaCircleObliqueEnvelopeMesh{}) {
		t.Fatal("a moved raw normal-bridge station published a display", err)
	}
	short, err := r.TessellateCertified(ctx, ParabolaTwoCirclesMeshOptions{MaxTriangles: 4})
	if !errors.Is(err, parametric.ErrBudget) || !reflect.DeepEqual(short, CertifiedParabolaCircleObliqueEnvelopeMesh{}) {
		t.Fatal("insufficient triangle budget published a partial envelope", err)
	}
}
