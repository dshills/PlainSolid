package plaincadbridge

import (
	"context"
	"math"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type roundedBoreSite struct {
	center geom.Vec2
	lower  int
	upper  int
	span   float64
	steps  []trimmed.AxialBoreStep
}

type roundedBores struct {
	base  *analyticBody
	sites []roundedBoreSite
}

func (h *roundedBores) withBase(solid trimmed.Solid, volume, area float64) *roundedBores {
	history, base := *h, *h.base
	base.solid = solid
	base.volume += volume
	base.area += area
	history.base = &base
	return &history
}

func overlapRoundedBore(a, tool stored) (float64, bool, error) {
	if a.analytic == nil || a.analytic.rounded == nil || a.analytic.rounded.stack == nil || tool.cylinder == nil {
		return 0, false, nil
	}
	stock := a.analytic.rounded
	site, step, err := stock.boreSite(tool.cylinder)
	if err != nil {
		return 0, true, err
	}
	base := a.analytic
	before := 0.
	if history := stock.stack.bores; history != nil {
		for _, prior := range history.sites {
			if prior.center == site.center && prior.lower == site.lower && prior.upper == site.upper {
				base = history.base
				site = prior
				before, _ = prior.measures()
				break
			}
		}
	}
	site.steps = append(append([]trimmed.AxialBoreStep(nil), site.steps...), step)
	err = trimmed.ProvePlanarSteppedBore(context.Background(), base.solid, site.lower, site.upper, site.center, site.steps)
	if err != nil {
		return 0, true, err
	}
	after, _ := site.measures()
	return max(0, after-before), true, nil
}

func cutRoundedBores(a stored, tools []stored) (*analyticBody, float64, bool, error) {
	if a.analytic == nil || a.analytic.rounded == nil || a.analytic.rounded.stack == nil {
		return nil, 0, false, nil
	}
	stock := a.analytic.rounded
	for _, tool := range tools {
		if tool.cylinder == nil {
			return nil, 0, false, nil
		}
	}
	history := roundedBores{base: a.analytic}
	if stock.stack.bores != nil {
		history = *stock.stack.bores
		history.sites = append([]roundedBoreSite(nil), history.sites...)
	}
	start := len(history.sites)
	for _, tool := range tools {
		site, step, err := stock.boreSite(tool.cylinder)
		if err != nil {
			return nil, 0, true, err
		}
		index := -1
		for i, existing := range history.sites {
			if existing.center == site.center && existing.upper == site.upper && existing.lower == site.lower {
				index = i
				break
			}
		}
		if index < 0 {
			index = len(history.sites)
			history.sites = append(history.sites, site)
		} else if index < start {
			start = 0
		}
		p := &history.sites[index]
		p.steps = append(append([]trimmed.AxialBoreStep(nil), p.steps...), step)
	}
	if len(history.sites) > 32 {
		return nil, 0, true, &failureError{"LIMIT", "rounded stack supports at most 32 axial bore sites"}
	}
	result := *a.analytic
	if start == 0 {
		result = *history.base
	}
	sites := make([]trimmed.PlanarSteppedBore, 0, len(history.sites)-start)
	for _, site := range history.sites[start:] {
		sites = append(sites, trimmed.PlanarSteppedBore{
			LowerCap: site.lower, UpperCap: site.upper, Center: site.center, Steps: site.steps,
		})
		volume, area := site.measures()
		result.volume -= volume
		result.area += area
	}
	geometry, err := trimmed.SewPlanarSteppedBores(context.Background(), result.solid, sites)
	if err != nil {
		return nil, 0, true, err
	}
	result.solid = geometry
	removed := a.analytic.volume - result.volume
	if !removesMaterial(a.analytic.volume, result.volume) {
		return nil, 0, true, &failureError{"MISMATCH", "rounded stack bore does not remove current material"}
	}
	recipe, stack := *stock, *stock.stack
	stack.bores = &history
	recipe.stack, recipe.cut = &stack, true
	result.rounded = &recipe
	return &result, removed, true, nil
}

func (p roundedStock) boreSite(c *cylinderBody) (roundedBoreSite, trimmed.AxialBoreStep, error) {
	start := p.frame.ToLocal(c.frame.Origin())
	axis, normal := c.frame.ZAxis(), p.frame.ZAxis()
	if web := p.stack.web; web != nil && (axis == web.frame.ZAxis() || axis == web.frame.ZAxis().Scale(-1)) {
		return web.throughSite(c)
	}
	fail := func() (roundedBoreSite, trimmed.AxialBoreStep, error) {
		return roundedBoreSite{}, trimmed.AxialBoreStep{}, geom.ErrDomain
	}
	if (axis != normal && axis != normal.Scale(-1)) || p.frame.ToWorld(start) != c.frame.Origin() {
		return fail()
	}
	end := start.Z + c.height*axis.Dot(normal)
	if !finite(end) {
		return fail()
	}
	lo, hi := min(start.Z, end), max(start.Z, end)
	site := roundedBoreSite{center: geom.Vec2{X: start.X, Y: start.Y}, lower: 0, upper: 1, span: p.height}
	step := trimmed.AxialBoreStep{Radius: c.radius}
	stack := p.stack
	if lo > p.height {
		site.lower, site.upper = -1, stack.upper
		if hi < stack.top || lo >= stack.top {
			return fail()
		}
		step.Depth = stack.top - lo
		return site, step, nil
	}
	if site.center == (geom.Vec2{}) && stack.pocketCap >= 0 && hi >= stack.floor {
		site.upper, site.span = stack.pocketCap, stack.floor
	}
	if hi < site.span || lo >= site.span {
		return fail()
	}
	if lo > 0 {
		step.Depth = site.span - lo
	}
	return site, step, nil
}

func (s roundedBoreSite) measures() (volume, area float64) {
	last, exit := 0., 0.
	for {
		radius, next := 0., math.Inf(1)
		for _, step := range s.steps {
			depth := step.Depth
			if depth == 0 {
				depth = s.span
			}
			if depth > last {
				radius = max(radius, step.Radius)
				next = min(next, depth)
			}
		}
		if radius == 0 {
			break
		}
		volume += math.Pi * radius * radius * (next - last)
		area += 2 * math.Pi * radius * (next - last)
		last, exit = next, radius
	}
	// The descending coaxial radii telescope across the annular floors.
	// A through bore also removes its exit opening.
	if s.lower >= 0 && last == s.span {
		area -= 2 * math.Pi * exit * exit
	}
	return volume, area
}
