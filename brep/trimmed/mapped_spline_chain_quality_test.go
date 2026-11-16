package trimmed

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/geom"
)

func TestMappedRoofQualityKeepsOriginalKnotAndTrimEdges(t *testing.T) {
	ctx := context.Background()
	makeQuad := func(points [4][2]float64, swap bool) CertifiedCurvedQuadraticTrimMesh {
		var face CertifiedCurvedQuadraticTrimMesh
		face.Mesh.Triangles = [][3]int{{0, 1, 2}, {0, 2, 3}}
		for _, p := range points {
			x, y := ratInset(p[0]), ratInset(p[1])
			face.ExactUV = append(face.ExactUV, [2]*big.Rat{x, y})
			if swap {
				x, y = y, x
			}
			face.ExactPoints = append(face.ExactPoints, [3]*big.Rat{x, y, new(big.Rat)})
		}
		return face
	}
	hasInterior := func(face CertifiedCurvedQuadraticTrimMesh, want mappedRoofEdge) bool {
		counts := make(map[mappedRoofEdge]int)
		for _, tri := range face.Mesh.Triangles {
			if exactUVSignedArea(face.ExactUV[tri[0]], face.ExactUV[tri[1]], face.ExactUV[tri[2]]).Sign() <= 0 {
				t.Fatal("flip inverted the source chart")
			}
			for i, a := range tri {
				counts[roofEdge(a, tri[(i+1)%3])]++
			}
		}
		return counts[want] == 2 && counts[roofEdge(0, 1)] == 1 && counts[roofEdge(1, 2)] == 1 &&
			counts[roofEdge(2, 3)] == 1 && counts[roofEdge(3, 0)] == 1
	}
	points := [4][2]float64{{0, 0}, {1, 0}, {1, 1}, {0, .5}}
	for _, swap := range []bool{false, true} {
		face := makeQuad(points, swap)
		orientation := 1
		if swap {
			orientation = -1
		}
		q := &mappedRoofQuality{orientation: orientation, remaining: 1}
		if err := q.improve(ctx, &face); err != nil || q.remaining != 0 || !hasInterior(face, roofEdge(1, 3)) {
			t.Fatal("strictly illegal projected diagonal should be flipped in either chart orientation", swap, err)
		}
	}
	face := makeQuad(points, false)
	before := append([][3]int(nil), face.Mesh.Triangles...)
	q := &mappedRoofQuality{orientation: 1}
	if err := q.improve(ctx, &face); !errors.Is(err, geom.ErrConvergence) || !reflect.DeepEqual(face.Mesh.Triangles, before) {
		t.Fatal("zero remaining flip work must reject before changing the diagonal", err)
	}

	// The illegal diagonal now coincides with the exact original support knot.
	face = makeQuad([4][2]float64{{0, -1}, {.25, 0}, {0, 1}, {-1, 0}}, false)
	q = &mappedRoofQuality{orientation: 1, remaining: 1}
	unlocked := makeQuad([4][2]float64{{0, -1}, {.25, 0}, {0, 1}, {-1, 0}}, false)
	if err := q.improve(ctx, &unlocked); err != nil || q.remaining != 0 || !hasInterior(unlocked, roofEdge(1, 3)) {
		t.Fatal("test knot diagonal must be strictly illegal without the constraint", err)
	}
	q = &mappedRoofQuality{orientation: 1, remaining: 1, knots: [2][]*big.Rat{{new(big.Rat)}, nil}}
	if err := q.improve(ctx, &face); err != nil || q.remaining != 1 || !hasInterior(face, roofEdge(0, 2)) {
		t.Fatal("original support knot must survive a quality-improving alternative", err)
	}
}
