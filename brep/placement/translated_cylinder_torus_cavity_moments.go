package placement

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// BoundTranslatedCylinderTorusCavityMoments translates the exact shifted-chart
// coefficient proof back to original coordinates. Source coefficients are in
// (x,y,z-centerZ); returned first/second tensors and inertia are in the authored
// world chart. Source and transform budgets retain their separate contracts.
func BoundTranslatedCylinderTorusCavityMoments(ctx context.Context, s geom.TranslatedCylinderTorusCavityMaterial, o TorusCylinderOccurrenceMomentOptions) (EccentricCylinderTorusCavityOccurrenceMoments, error) {
	source, z, e := s.ShiftedSource()
	if e != nil {
		return EccentricCylinderTorusCavityOccurrenceMoments{}, e
	}
	var m [3][4]*big.Rat
	for i := range m {
		for j := range m[i] {
			m[i][j] = new(big.Rat)
		}
		m[i][i].SetInt64(1)
	}
	m[2][3] = z
	pose, e := NewRationalRigid(m, 4096)
	if e != nil {
		return EccentricCylinderTorusCavityOccurrenceMoments{}, e
	}
	return BoundEccentricCylinderTorusCavityMoments(ctx, source, pose, o)
}
