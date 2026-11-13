// Package testgraphfixture supplies expensive, independently verified test proposals.
package testgraphfixture

import (
	"context"
	"fmt"
	"math"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// NestedCircleSeeds proposes circles centered at (.5,.5) with radii 5/32 and 5/16
// on the supplied original graphs. Every slab is independently tube-certified;
// callers must still prove full pair coverage and branch topology.
func NestedCircleSeeds(ctx context.Context, a, b *geom.BSplineSurface) (intersect.SplineSeedReport, error) {
	var e error
	var seeds intersect.SplineSeedReport
	if ctx == nil || a == nil || b == nil {
		return seeds, geom.ErrInvalid
	}
	for _, radius := range []float64{5.0 / 32, 5.0 / 16} {
		for _, sx := range []float64{-1, 1} {
			for _, sy := range []float64{-1, 1} {
				for axis := 0; axis < 2; axis++ {
					limit := radius * .6
					if axis == 1 {
						limit = radius * .8
					}
					step := 4
					for part := 0; part < 64; {
						if e := ctx.Err(); e != nil {
							return seeds, e
						}
						if step > 64-part {
							step = 64 - part
						}
						lo, hi := limit*float64(part)/64, limit*float64(part+step)/64
						depLo, depHi := math.Sqrt(radius*radius-hi*hi), math.Sqrt(radius*radius-lo*lo)
						padding := .003
						if step == 2 {
							padding = .004
						}
						if step == 1 {
							padding = .002
						}
						freeLo, freeHi := .5+sx*lo, .5+sx*hi
						depMin, depMax := .5+sy*depLo, .5+sy*depHi
						if axis == 1 {
							freeLo, freeHi = .5+sy*lo, .5+sy*hi
							depMin, depMax = .5+sx*depLo, .5+sx*depHi
						}
						if freeLo > freeHi {
							freeLo, freeHi = freeHi, freeLo
						}
						if depMin > depMax {
							depMin, depMax = depMax, depMin
						}
						box := geom.ParameterRectangle{UMin: freeLo, UMax: freeHi, VMin: depMin - padding, VMax: depMax + padding}
						if axis == 1 {
							box = geom.ParameterRectangle{UMin: depMin - padding, UMax: depMax + padding, VMin: freeLo, VMax: freeHi}
						}
						other := box
						if axis == 0 {
							other.UMin -= padding
							other.UMax += padding
						} else {
							other.VMin -= padding
							other.VMax += padding
						}
						xa, _ := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
						xb, _ := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
						xa, e = xa.Trim(box)
						if e != nil {
							return seeds, e
						}
						xb, e = xb.Trim(other)
						if e != nil {
							return seeds, e
						}
						proof, e := geom.CertifySurfacePairTube(xa, xb, geom.SurfacePairTubeOptions{FreeAxis: axis, MaxOperations: 8000000})
						if e != nil || !proof.ProvenBranch {
							if step > 1 {
								step /= 2
								continue
							}
							return seeds, fmt.Errorf("tube radius=%g axis=%d part=%d: %v", radius, axis, part, e)
						}
						seeds.Attempts = append(seeds.Attempts, intersect.SplineSeedCell{Cell: geom.SurfaceSurfaceCell{DomainA: box, DomainB: other}, Tube: &proof})
						part += step
						step = 4
					}
				}
			}
		}
	}

	return seeds, nil
}
