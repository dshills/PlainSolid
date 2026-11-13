package geom

import (
	"math/big"
	"math/rand"
	"sort"
	"testing"
)

// Build Bernstein coefficients from a product of known linear factors using
// exact arithmetic, then multiply by degree! so every coefficient is integer.
func TestSplinePlaneKnownRootOracle(t *testing.T) {
	random := rand.New(rand.NewSource(79213))
	plane, _ := NewPlane(Vec3{}, Vec3{0, 0, 1})
	for trial := 0; trial < 80; trial++ {
		degree := 1 + random.Intn(6)
		polynomial := []int64{1}
		multiplicities := map[int]int{}
		for factor := 0; factor < degree; factor++ {
			numerator := 1 + random.Intn(7)
			multiplicities[numerator]++
			next := make([]int64, len(polynomial)+1)
			for i, c := range polynomial {
				next[i] -= int64(numerator) * c
				next[i+1] += 8 * c
			}
			polynomial = next
		}
		factorial := int64(1)
		for i := 2; i <= degree; i++ {
			factorial *= int64(i)
		}
		poles := make([]Vec3, degree+1)
		for i := range poles {
			coefficient := new(big.Rat)
			for k := 0; k <= i; k++ {
				term := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(k)), new(big.Int).Binomial(int64(degree), int64(k)))
				term.Mul(term, big.NewRat(polynomial[k]*factorial, 1))
				coefficient.Add(coefficient, term)
			}
			if !coefficient.IsInt() {
				t.Fatal("oracle construction")
			}
			z, _ := coefficient.Float64()
			poles[i] = Vec3{float64(i), 0, z}
		}
		curve, e := NewBezier(poles, nil)
		if e != nil {
			t.Fatal(e)
		}
		result, e := IntersectSplinePlane(curve, plane, SplinePlaneOptions{DistanceTolerance: 1e-6})
		if e != nil || len(result.Hits) != len(multiplicities) {
			t.Fatal(trial, result, e, multiplicities)
		}
		keys := make([]int, 0, len(multiplicities))
		for k := range multiplicities {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		for i, k := range keys {
			hit := result.Hits[i]
			want := float64(k) / 8
			if hit.Parameter != want || hit.ParameterMin != want || hit.ParameterMax != want || hit.Multiplicity != multiplicities[k] {
				t.Fatal(trial, hit, k, multiplicities[k])
			}
		}
	}
}
