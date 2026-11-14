package trimmed

import (
	"context"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

// Exact determinant-one model warp z'=z+(x²+xy)/8. Unlike the earlier
// bilinear fixture, original and section edges varying x have curved images.
func makeNativeCurvedTubeBox(t *testing.T, lo, hi [3]float64) Solid {
	t.Helper()
	var surfaces []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	var reversed []bool
	for axis := 0; axis < 3; axis++ {
		u, v := (axis+1)%3, (axis+2)%3
		for side, fixed := range []float64{lo[axis], hi[axis]} {
			poles := make([]geom.Vec3, 0, 9)
			for i := 0; i < 3; i++ {
				for j := 0; j < 3; j++ {
					p := [3]float64{}
					p[axis] = fixed
					p[u] = lo[u] + (hi[u]-lo[u])*float64(i)/2
					p[v] = lo[v] + (hi[v]-lo[v])*float64(j)/2
					x2 := fixed * fixed
					if axis != 0 {
						index := i
						if v == 0 {
							index = j
						}
						x2 = []float64{lo[0] * lo[0], lo[0] * hi[0], hi[0] * hi[0]}[index]
					}
					p[2] += (x2 + p[0]*p[1]) / 8
					poles = append(poles, geom.Vec3{X: p[0], Y: p[1], Z: p[2]})
				}
			}
			ku := []float64{lo[u], lo[u], lo[u], hi[u], hi[u], hi[u]}
			kv := []float64{lo[v], lo[v], lo[v], hi[v], hi[v], hi[v]}
			s, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, ku, kv, nil)
			if e != nil {
				t.Fatal(e)
			}
			surfaces = append(surfaces, s)
			domains = append(domains, geom.ParameterRectangle{UMin: lo[u], UMax: hi[u], VMin: lo[v], VMax: hi[v]})
			reversed = append(reversed, side == 0)
		}
	}
	result, _, e := SewSplineRectangles(context.Background(), surfaces, domains, reversed, SewOptions{Tolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	return result
}

func curvedNativeBoxSeeds(t *testing.T, a, b Solid) []AlgebraicShellSectionSeed {
	t.Helper()
	var out []AlgebraicShellSectionSeed
	lows := [2][3]float64{{0, 0, 0}, {.5, .5, -.5}}
	highs := [2][3]float64{{1, 1, 1}, {1.5, 1.5, .5}}
	for ai := 0; ai < 6; ai++ {
		for bi := 0; bi < 6; bi++ {
			ax, bx := ai/2, bi/2
			if ax == bx {
				continue
			}
			af := lows[0][ax]
			if ai%2 == 1 {
				af = highs[0][ax]
			}
			bf := lows[1][bx]
			if bi%2 == 1 {
				bf = highs[1][bx]
			}
			if af <= lows[1][ax] || af >= highs[1][ax] || bf <= lows[0][bx] || bf >= highs[0][bx] {
				continue
			}
			free := 3 - ax - bx
			lo, hi := max(lows[0][free], lows[1][free]), min(highs[0][free], highs[1][free])
			seed := AlgebraicShellSectionSeed{Faces: [2]int{ai, bi}}
			for piece := 0; piece < 4; piece++ {
				l, h := lo+(hi-lo)*float64(piece)/4, lo+(hi-lo)*float64(piece+1)/4
				owner := 0
				if piece < 2 && lo == lows[1][free] || piece >= 2 && hi == highs[1][free] {
					owner = 1
				}
				var domains [2]geom.ParameterRectangle
				axis := 0
				for operand, face := range []int{ai, bi} {
					fixedAxis := face / 2
					bounds := [2][2]float64{}
					for uv := 0; uv < 2; uv++ {
						coord := (fixedAxis + 1 + uv) % 3
						minV, maxV := l, h
						if coord != free {
							v := af
							if operand == 0 {
								v = bf
							}
							minV, maxV = v-1.0/16, v+1.0/16
						} else if operand == owner {
							axis = 2*operand + uv
						} else {
							minV -= 1.0 / 32
							maxV += 1.0 / 32
						}
						bounds[uv] = [2]float64{max(minV, lows[operand][coord]), min(maxV, highs[operand][coord])}
					}
					domains[operand] = geom.ParameterRectangle{UMin: bounds[0][0], UMax: bounds[0][1], VMin: bounds[1][0], VMax: bounds[1][1]}
				}
				pa, e := exactFaceSupportPatch(a.faces[ai], geom.SurfacePatchOptions{})
				if e != nil {
					t.Fatal(e)
				}
				pb, e := exactFaceSupportPatch(b.faces[bi], geom.SurfacePatchOptions{})
				if e != nil {
					t.Fatal(e)
				}
				pa, e = pa.Trim(domains[0])
				if e != nil {
					t.Fatal(e)
				}
				pb, e = pb.Trim(domains[1])
				if e != nil {
					t.Fatal(e)
				}
				proof, e := geom.CertifySurfacePairTube(pa, pb, geom.SurfacePairTubeOptions{FreeAxis: axis})
				if e != nil || !proof.ProvenBranch {
					t.Fatal(ai, bi, piece, proof, e)
				}
				seed.Tubes.Attempts = append(seed.Tubes.Attempts, intersect.SplineSeedCell{Cell: geom.SurfaceSurfaceCell{DomainA: domains[0], DomainB: domains[1]}, Tube: &proof})
			}
			out = append(out, seed)
		}
	}
	return out
}
