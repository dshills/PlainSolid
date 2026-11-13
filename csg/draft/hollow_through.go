package draft

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/csg/poly"
)

// ThroughHollowOptions selects two complete unique separated supporting
// faces as openings. OpenFaces refers to original face indices; thickness is
// zero at those faces and strictly positive elsewhere. CoordinateThickness
// and independent reconstruction/Boolean budgets have HollowOptions semantics.
type ThroughHollowOptions struct {
	OpenFaces           [2]int
	CoordinateThickness bool
	Reconstruction      Options
	Boolean             poly.Options
}

// ThroughHollowResult adds ideal exact material volume and explicit original
// opening-to-interior face identities. ExactVolume is independent of the
// checked rounded display; its rational is caller-owned.
type ThroughHollowResult struct {
	HollowResult
	ExactVolume          *big.Rat
	OpenFaces            [2]int
	InteriorOpeningFaces [2]int
}

// HollowThrough subtracts an exact support-offset cavity joining two separated
// end faces of a convex planar solid. All other supports remain
// strictly outside the cavity, including after display conversion. Both end
// openings must survive with exact original-plane coincidence. The returned
// material has closed walls/rims; this is a through duct, not an open mesh.
// Each reconstruction, support check and Boolean pass has its own configured
// budget. No partial result is returned on failure. Input arrays remain owned
// by the caller. Adjacent, touching or overlapping openings are unsupported.
func HollowThrough(ctx context.Context, s brep.Solid, thickness []float64, o ThroughHollowOptions) (ThroughHollowResult, error) {
	r, e := HollowPorts(ctx, s, thickness, PortHollowOptions{OpenFaces: o.OpenFaces[:], CoordinateThickness: o.CoordinateThickness, Reconstruction: o.Reconstruction, Boolean: o.Boolean})
	if e != nil {
		return ThroughHollowResult{}, e
	}
	return ThroughHollowResult{HollowResult: r.HollowResult, ExactVolume: r.ExactVolume, OpenFaces: o.OpenFaces, InteriorOpeningFaces: [2]int{r.InteriorOpeningFaces[0], r.InteriorOpeningFaces[1]}}, nil
}
