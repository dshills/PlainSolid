package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func acnodalCubicStockFixture(t *testing.T, separateArc bool) [3]*geom.BSplineSurface {
	t.Helper()
	// A−B=(3/16)*((v-1/2)^2-(u-13/16)^2*(u-13/16-gap)).
	// These are the exact Bernstein controls of the free-coordinate term.
	if separateArc {
		return nodalCubicStockFixture(t, [4]float64{-7605.0 / 65536, 1339.0 / 65536, -213.0 / 65536, 27.0 / 65536}, 3.0/64)
	}
	return nodalCubicStockFixture(t, [4]float64{-8619.0 / 65536, 1157.0 / 65536, -75.0 / 65536, -27.0 / 65536}, 3.0/64)
}

func TestNativeAcnodalCubicEnvelope(t *testing.T) {
	ctx := context.Background()
	for _, branch := range []bool{false, true} {
		src := acnodalCubicStockFixture(t, branch)
		// Integrate 3×2 times the original signed height polynomial.
		// The separate arc has opposing material 4703/1614807040:
		// integrate (4/3)*y(t)^3 with y(t)=(1/8+t²)*t,
		// du=2t dt and 0<=t<=1/4, and multiply by 6*(3/16).
		// The bilinear roof above the original base integrates to
		// 6*(2+3/32+3/128-1/16)=789/64 before the signed correction.
		volumes := [2]float64{789.0 / 64, 412713.0 / 32768}
		if branch {
			volumes = [2]float64{19907538337.0 / 1614807040, 20297401823.0 / 1614807040}
		}
		for order := range 2 {
			for k, build := range []func(context.Context, *geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface, int, float64, AcnodalCubicEnvelopeOptions) (*AcnodalCubicEnvelope, error){
				CommonAcnodalCubicSplineGraphs, FuseAcnodalCubicSplineGraphs,
			} {
				t.Run(fmt.Sprintf("arc_%v/order_%d/%s", branch, order, [2]string{"common", "fuse"}[k]), func(t *testing.T) {
					owners := [2]*geom.BSplineSurface{src[1+order], src[2-order]}
					out, err := build(ctx, src[0], owners[0], owners[1], order, 1e-9, AcnodalCubicEnvelopeOptions{})
					if err != nil {
						t.Fatal(err)
					}
					s, h := out.Boundary(), out.History()
					wantOwner := 1 ^ order ^ k
					if h.IsolatedFace != h.RoofFaces[0] || h.RoofOwners[0] != wantOwner ||
						s.faces[h.IsolatedFace].Surface != owners[wantOwner] || s.faces[h.Base].Surface != src[0] {
						t.Fatal("the isolated point must belong to one unsplit original roof", h)
					}
					p := out.IsolatedContact()
					q, err := geom.RepresentedExactPoint(p)
					if err != nil || q != (geom.Vec3{X: 39.0 / 16, Y: 1, Z: 2243.0 / 1024}) || slices.Contains(s.vertices, q) {
						t.Fatal("isolated proof changed or turned into a seam vertex", q, err)
					}
					p[0].SetInt64(0)
					if out.IsolatedContact()[0].Sign() == 0 {
						t.Fatal("caller can mutate retained original contact")
					}
					topo := s.Topology()
					faces := 6
					if branch {
						faces = 9
					}
					if len(topo.Shells) != 1 || topo.EulerCharacteristic != 2 || topo.CollapsedEdges != 0 || len(s.faces) != faces {
						t.Fatal("wrong closed material topology", topo)
					}
					if branch {
						if s.faces[h.RoofFaces[1]].Surface != owners[wantOwner^1] || s.vertices[h.ArcVertices[0]].X != 45.0/16 {
							t.Fatal("wrong regular cap owner or turning point", h)
						}
						for i, id := range h.ArcEdges {
							e := s.edges[id]
							if !slices.Contains([]int{e.Start, e.End}, h.ArcVertices[0]) ||
								!slices.Contains([]int{e.Start, e.End}, h.ArcVertices[i+1]) ||
								!slices.Equal(nodalCubicIncidentFaces(s, id), h.RoofFaces[:]) {
								t.Fatal("a regular restriction lost its original occurrence", id, e)
							}
							at := e.Range.Min
							if e.End == h.ArcVertices[0] {
								at = e.Range.Max
							}
							v, err := e.Curve.Derivative(at)
							if err != nil || v.X != 0 || math.Abs(v.Y) != .0625 {
								t.Fatal("the regular turn was treated as a singular cusp", v, err)
							}
						}
					} else if h.RoofFaces[1] >= 0 || h.ArcVertices != ([3]int{-1, -1, -1}) || h.ArcEdges != ([2]int{-1, -1}) {
						t.Fatal("nonexistent arc created a face or seam", h)
					}
					m, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
					if err != nil || math.Abs(m.Volume-volumes[k]) > 1e-9 {
						t.Fatal("different material from independently integrated original roofs", m, volumes[k], err)
					}
					t.Logf("volume %.16g area %.16g", m.Volume, m.Area)
					var native, replay bytes.Buffer
					if err := out.WriteJSON(ctx, &native); err != nil {
						t.Fatal(err)
					}
					loaded, err := ReadAcnodalCubicEnvelopeJSON(ctx, bytes.NewReader(native.Bytes()), JSONLimits{}, AcnodalCubicEnvelopeOptions{})
					if err != nil || loaded.History() != out.History() || loaded.upper != (k == 1) {
						t.Fatal("incorrect native operation or contact incidence on replay", err)
					}
					if err := loaded.WriteJSON(ctx, &replay); err != nil || !bytes.Equal(native.Bytes(), replay.Bytes()) {
						t.Fatal("replayed isolated contact or branch receipt changed", err)
					}
				})
			}
		}
	}
}

func TestNativeAcnodalCubicEnvelopeRejectsChangedLedger(t *testing.T) {
	ctx := context.Background()
	src := acnodalCubicStockFixture(t, true)
	o := AcnodalCubicEnvelopeOptions{}
	result, err := CommonAcnodalCubicSplineGraphs(ctx, src[0], src[1], src[2], 0, 1e-9, o)
	if err != nil {
		t.Fatal(err)
	}
	var raw bytes.Buffer
	if err := result.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func(*acnodalCubicEnvelopeRecipe)
	}{
		{"moved isolated contact", func(r *acnodalCubicEnvelopeRecipe) { r.Ledger.IsolatedPoint[0] = "5/2" }},
		{"changed source tangent", func(r *acnodalCubicEnvelopeRecipe) { r.Ledger.IsolatedTangents[0][2] = "1" }},
		{"forged roof ownership", func(r *acnodalCubicEnvelopeRecipe) { r.Ledger.RoofOwners[0] ^= 1 }},
		{"omitted separate branch", func(r *acnodalCubicEnvelopeRecipe) { r.Ledger.ArcEdges = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var recipe acnodalCubicEnvelopeRecipe
			if err := json.Unmarshal(raw.Bytes(), &recipe); err != nil {
				t.Fatal(err)
			}
			tc.edit(&recipe)
			changed, err := json.Marshal(recipe)
			if err != nil {
				t.Fatal(err)
			}
			if out, err := ReadAcnodalCubicEnvelopeJSON(ctx, bytes.NewReader(changed), JSONLimits{}, o); out != nil || !errors.Is(err, geom.ErrConvergence) {
				t.Fatal("forged receipt published material", err)
			}
		})
	}
	if out, err := ReadAcnodalCubicEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{MaxEntities: 1}, o); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("truncated topology budget published material", err)
	}
}

func TestNativeAcnodalCubicEnvelopeOtherStockSides(t *testing.T) {
	ctx := context.Background()
	stock := acnodalCubicStockFixture(t, true)
	for name, place := range map[string]func(geom.Vec3) geom.Vec3{
		"left":  func(p geom.Vec3) geom.Vec3 { return geom.Vec3{X: 3 - p.X, Y: p.Y, Z: p.Z} },
		"upper": func(p geom.Vec3) geom.Vec3 { return geom.Vec3{X: p.Y, Y: p.X, Z: p.Z} },
		"lower": func(p geom.Vec3) geom.Vec3 { return geom.Vec3{X: p.Y, Y: 3 - p.X, Z: p.Z} },
	} {
		t.Run(name, func(t *testing.T) {
			var source [3]*geom.BSplineSurface
			for i, patch := range stock {
				d := patch.Data()
				for j, p := range d.Poles {
					d.Poles[j] = place(p)
				}
				var err error
				source[i], err = geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
				if err != nil {
					t.Fatal(err)
				}
			}
			out, err := CommonAcnodalCubicSplineGraphs(ctx, source[0], source[1], source[2], 0, 1e-9, AcnodalCubicEnvelopeOptions{})
			if err != nil {
				t.Fatal(err)
			}
			s, h := out.Boundary(), out.History()
			for i, vertex := range [3]geom.Vec3{
				{X: 45.0 / 16, Y: 1, Z: 2273.0 / 1024},
				{X: 3, Y: 29.0 / 32, Z: 4567.0 / 2048},
				{X: 3, Y: 35.0 / 32, Z: 4585.0 / 2048},
			} {
				if s.vertices[h.ArcVertices[i]] != place(vertex) {
					t.Fatal("placement changed original branch occurrence", i, s.vertices[h.ArcVertices[i]], place(vertex))
				}
			}
			m, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
			if err != nil || math.Abs(m.Volume-19907538337.0/1614807040) > 1e-9 {
				t.Fatal("axis permutation or reflection changed signed material", m, err)
			}
		})
	}
}
