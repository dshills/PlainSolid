package plaincadbridge

import (
	"context"

	"plainsolid/csg/poly"
)

// legacyBoolean serves cut, fuse and common by the paths that each take one
// family of stock. It is the default bridge's first answer; orGeneral says
// what happens where it refuses.
func (b *Bridge) legacyBoolean(ctx context.Context, a, c stored, op string) (any, error) {
	if a.tangent != nil || c.tangent != nil {
		return nil, unsupportedEdge("Boolean operations on side-tangent blind bores are outside the supported domain")
	}
	if op == "cut" {
		if body, ok := tangentBlindBoreRecipe(a, c); ok {
			return b.putTangentBlindBore(body)
		}
	}
	if op != "common" {
		if result, removed, handled, err := retainedPlanarBoolean(a, c, op); handled {
			if err != nil {
				return nil, err
			}
			if op == "cut" && removed <= 0 {
				return nil, unsupportedEdge("Boolean cut removed no volume")
			}
			return b.putAnalyticBody(result)
		}
	}

	if op == "cut" {
		if result, handled, err := b.cutSideOpeningFamilies(a, []stored{c}); handled {
			return result, err
		}
		if a.analytic != nil && a.analytic.sideNotch != nil {
			if result, handled, err := b.cutSideNotchBores(a, []stored{c}); handled {
				return result, err
			}
		}
		if result, handled, err := b.rectangularIslandPocket(a, c); handled {
			return result, err
		}
		if result, handled, err := b.cutRoundedStack(a, c); handled {
			return result, err
		}
		if result, _, handled, err := cutRoundedBores(a, []stored{c}); handled {
			if err != nil {
				return nil, err
			}
			return b.putAnalyticBody(result)
		}
		if result, _, handled, err := cutThroughProfiles(a, []stored{c}); handled {
			if err != nil {
				return nil, err
			}
			return b.putAnalyticBody(result)
		}
		if c.planar() {
			if result, handled, err := b.rectangularPocket(a, c); handled {
				return result, err
			}
			if !a.planar() {
				stock, converted, err := a.planarCutBoundary()
				if err != nil {
					return nil, err
				}
				if converted {
					if pocket, ok := openPlanarPocket(stock, c.solid); ok {
						return b.publish(pocket)
					}
					solid, err := poly.Combine(ctx, stock, c.solid, poly.Difference, poly.Options{})
					if err != nil {
						return nil, err
					}
					return b.put(solid)
				}
			}
		}
		if body, removed, handled, err := cutPocketCylinder(a, c.cylinder); handled {
			if err != nil {
				return nil, err
			}
			if removed <= 0 {
				return nil, unsupportedEdge("Boolean cut removed no volume")
			}
			return b.putAnalyticBody(body)
		}
		if a.analytic != nil && a.analytic.shaft != nil && c.planar() {
			return b.cutShaftSocket(*a.analytic.shaft, c.solid)
		}
		if sector, ok := coaxialThrough(a, c.cylinder); ok {
			sector.inner = c.cylinder.radius
			return b.putRadialSector(sector)
		}
		if result, handled, err := b.cutCircularStockBores(a, []stored{c}); handled {
			return result, err
		}
	}
	if op == "fuse" {
		if result, handled, err := b.fuseRoundedStack(a, c); handled {
			return result, err
		}
		if result, handled, err := b.fuseAnnularPedestal(a, c); handled {
			return result, err
		}
		if result, handled, err := b.fuseCoaxial(a, c); handled {
			return result, err
		}
		if a.planar() && c.cylinder != nil {
			if result, handled, err := b.fusePlainCircularBoss(a, c.cylinder); handled {
				return result, err
			}
		}
	}
	if a.analytic != nil || c.analytic != nil {
		return nil, &failureError{"UNSUPPORTED", "Boolean operation is outside the supported analytic stock domain"}
	}
	if result, handled, err := b.cornerCylinderBoolean(a, c, op); handled {
		return result, err
	}
	if op == "fuse" && a.bore != nil && c.cylinder != nil {
		if result, handled, err := b.fuseCircularBoss(a.bore, c.cylinder); handled {
			return result, err
		}
	}
	if op == "fuse" && a.bore != nil && c.planar() {
		if result, handled, err := b.fuseBoredStock(a.bore, c.solid); handled {
			return result, err
		}
	}
	if a.treated != nil || c.treated != nil {
		return b.treatedBoolean(a, c, op)
	}
	if a.cylinder != nil || c.cylinder != nil || a.bore != nil || c.bore != nil {
		return b.curvedBoolean(a, c, op)
	}
	if op == "fuse" {
		if box, ok := joinedRectangularStock(a.solid, c.solid); ok {
			return b.put(box)
		}
	}
	planar := poly.Difference
	if op == "fuse" {
		planar = poly.Union
	}
	if op == "common" {
		planar = poly.Intersection
	}
	s, err := poly.Combine(ctx, a.solid, c.solid, planar, poly.Options{})
	if err != nil {
		return nil, err
	}
	return b.put(s)
}
