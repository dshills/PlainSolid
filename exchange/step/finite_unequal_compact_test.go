package step

import (
	"bytes"
	"context"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"strings"
	"testing"
)

func TestFiniteUnequalCompactSplineAgreement(t *testing.T) {
	for _, r := range [][3]float64{{3, 2, 0}, {4, 2, .5}} {
		maxModel, maxOwner, maxResidual := 0., 0., 0.
		frame, _ := geom.FrameFromNormal(geom.Vec3{X: 20.125, Y: -40.25, Z: 100.5}, geom.Vec3{Z: -1})
		result, err := trimmed.BoxFiniteSkewCrossedBores(context.Background(), frame,
			geom.Vec3{X: 20, Y: 24, Z: 18}, geom.Vec3{X: 8.25, Y: 12.5, Z: 9}, r[0], r[1], r[2], 15, 15, false, true, 1e-9)
		if err != nil {
			t.Fatal(err)
		}
		faces, edges, vertices := result.Solid.Faces(), result.Solid.Edges(), result.Solid.Vertices()
		owners := make([][]longEdgeOwner, len(edges))
		supports := make([]longEdgeSurface, len(faces))
		for faceID, face := range faces {
			supports[faceID], err = longEdgeSurfaceOf(face.Surface)
			if err != nil {
				t.Fatal(err)
			}
			if supports[faceID].kind != "CYLINDRICAL_SURFACE" {
				continue
			}
			for _, wire := range append([][]trimmed.Use{face.Outer}, face.Holes...) {
				for _, use := range wire {
					if use.Curve != nil {
						owners[use.Edge] = append(owners[use.Edge], longEdgeOwner{faceID, use})
					}
				}
			}
		}

		check := func(poles []geom.Vec3, step int, t float64) geom.Vec3 {
			spans := (len(poles) - 2) / 2
			start, end := poles[0], poles[len(poles)-1]
			if step > 0 {
				start = poles[2*step].Scale(.5).Add(poles[2*step+1].Scale(.5))
			}
			if step < spans-1 {
				end = poles[2*step+2].Scale(.5).Add(poles[2*step+3].Scale(.5))
			}
			u := 1 - t
			return start.Scale(u * u * u).Add(poles[2*step+1].Scale(3 * u * u * t)).Add(poles[2*step+2].Scale(3 * u * t * t)).Add(end.Scale(t * t * t))
		}

		count := 0
		for edgeID, adjacent := range owners {
			if len(adjacent) != 2 {
				continue
			}
			count++
			edge := edges[edgeID]
			splines, err := approximateUnequalBoreEdgeOptions(vertices[edge.Start], vertices[edge.End], adjacent, supports, UnequalBoreSTEPAccuracyMillimeters, true)
			if err != nil {
				t.Fatal(err)
			}
			if !splines.smooth {
				t.Fatal("finite Hermite joins not compacted")
			}
			spans := (len(splines.model) - 2) / 2
			for index, owner := range adjacent {
				surface := faces[owner.face].Surface
				for i := range spans {
					for _, f := range []float64{.125, .25, .5, .75, .875} {
						point := check(splines.model, i, f)
						approx := check(splines.owners[index], i, f)
						projected, err := surface.Eval(approx.X, approx.Y)
						if err != nil {
							t.Fatal(err)
						}
						tEdge := (float64(i) + f) / float64(spans)
						if owner.use.Reversed {
							tEdge = 1 - tEdge
						}
						uv, err := owner.use.Curve.EvalUV(tEdge)
						if err != nil {
							t.Fatal(err)
						}
						exact, err := surface.Eval(uv.X, uv.Y)
						maxModel = max(maxModel, point.Distance(exact))
						maxOwner = max(maxOwner, projected.Distance(exact))
						maxResidual = max(maxResidual, projected.Distance(point))
						if err != nil || point.Distance(exact) > UnequalBoreSTEPAccuracyMillimeters/4 || projected.Distance(exact) > UnequalBoreSTEPAccuracyMillimeters/4 ||
							projected.Distance(point) > UnequalBoreSTEPAccuracyMillimeters/2 {
							t.Fatalf("edge %d owner %d spline misses native intersection at %g", edgeID, index, tEdge)
						}
					}
				}
			}
		}
		if count != 8 {
			t.Fatalf("tested %d shared spline edges", count)
		}
		t.Logf("radii/offset%v model=%g owner=%g residual=%g", r, maxModel, maxOwner, maxResidual)
	}
}

func TestFiniteUnequalCompactReadsLegacy(t *testing.T) {
	slowtest.Skip(t)
	frame, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, geom.Vec3{Z: 1})
	for _, offset := range []float64{0, .5} {
		m := UnequalBoreModel{Body: UnequalBoreBody{Name: "legacy finite", Frame: frame, Dimensions: geom.Vec3{X: 20, Y: 24, Z: 18}, Crossing: geom.Vec3{X: 10, Y: 12, Z: 9}, XRadius: 4, ZRadius: 2, Offset: offset, XBlindDepth: 15.25, ZBlindDepth: 14.4, ZOppositeCap: true}, LengthUnit: .001}
		var legacy, compact bytes.Buffer
		if e := writeUnequalBoreOptions(&legacy, m, false); e != nil {
			t.Fatal(e)
		}
		old, e := ReadUnequalBore(bytes.NewReader(legacy.Bytes()), Limits{})
		if e != nil {
			t.Fatal("old finite file rejected", e)
		}
		if e := WriteUnequalBore(&compact, old); e != nil {
			t.Fatal(e)
		}
		current, e := ReadUnequalBore(bytes.NewReader(compact.Bytes()), Limits{})
		if e != nil || !reflect.DeepEqual(old, current) {
			t.Fatal("changed retained recipe", e)
		}
		if compact.Len() >= legacy.Len() {
			t.Fatal("no compaction", legacy.Len(), compact.Len())
		}
		bad := strings.Replace(legacy.String(), "CYLINDRICAL_SURFACE(", "CONICAL_SURFACE(", 1)
		if _, e := ReadUnequalBore(strings.NewReader(bad), Limits{}); e == nil {
			t.Fatal("legacy fallback admitted corrupt support")
		}
		t.Logf("offset%g old=%d compact=%d", offset, legacy.Len(), compact.Len())
	}
}
