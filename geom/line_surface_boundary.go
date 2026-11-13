package geom

func trimPatchBudget(p *SurfacePatch, r ParameterRectangle, b *polynomialBudget) (*SurfacePatch, error) {
	d := p.Domain()
	current := p
	var e error
	if r.UMin > d.UMin {
		_, current, e = current.split(r.UMin, true, b)
		if e != nil {
			return nil, e
		}
	}
	if r.UMax < d.UMax {
		current, _, e = current.split(r.UMax, true, b)
		if e != nil {
			return nil, e
		}
	}
	if r.VMin > d.VMin {
		_, current, e = current.split(r.VMin, false, b)
		if e != nil {
			return nil, e
		}
	}
	if r.VMax < d.VMax {
		current, _, e = current.split(r.VMax, false, b)
		if e != nil {
			return nil, e
		}
	}
	return current, nil
}
func lineHitRootInside(a, b LineSplineSurfaceHit) bool {
	return a.UV.Min.X >= b.Cell.UMin && a.UV.Max.X <= b.Cell.UMax && a.UV.Min.Y >= b.Cell.VMin && a.UV.Max.Y <= b.Cell.VMax && a.ParameterMin >= b.LineRange.Start && a.ParameterMax <= b.LineRange.End
}
func lineHitCellsOverlap(a, b LineSplineSurfaceHit) bool {
	return a.Cell.UMin <= b.Cell.UMax && b.Cell.UMin <= a.Cell.UMax && a.Cell.VMin <= b.Cell.VMax && b.Cell.VMin <= a.Cell.VMax && a.LineRange.Start <= b.LineRange.End && b.LineRange.Start <= a.LineRange.End
}

// Rescue internal split-boundary roots in overlapping neighborhoods. A new
// root is identified with an existing one only when its full root enclosure
// lies inside the other's certified uniqueness box (or conversely).
func rescueLineSurfaceBoundaries(result *LineSplineSurfaceResult, patch, linePatch *SurfacePatch, o LineSplineSurfaceOptions) error {
	pending := result.Pending
	result.Pending = nil
	domain, ld := patch.Domain(), linePatch.Domain()
	for index, cell := range pending {
		if result.ExactOperations >= o.MaxOperations {
			result.Pending = append(result.Pending, pending[index:]...)
			return ErrConvergence
		}
		expand := func(lo, hi, minValue, maxValue float64) (float64, float64) {
			width := hi - lo
			return max(minValue, lo-width), min(maxValue, hi+width)
		}
		u0, u1 := expand(cell.Cell.UMin, cell.Cell.UMax, domain.UMin, domain.UMax)
		v0, v1 := expand(cell.Cell.VMin, cell.Cell.VMax, domain.VMin, domain.VMax)
		t0, t1 := expand(cell.LineRange.Start, cell.LineRange.End, ld.VMin, ld.VMax)
		budget := &polynomialBudget{maxOperations: o.MaxOperations - result.ExactOperations, maxBits: o.MaxCoefficientBits}
		p, e := trimPatchBudget(patch, ParameterRectangle{u0, u1, v0, v1}, budget)
		if e == nil {
			var l *SurfacePatch
			l, e = trimPatchBudget(linePatch, ParameterRectangle{ld.UMin, ld.UMax, t0, t1}, budget)
			result.ExactOperations += budget.operations
			if e == nil && p.countU == p.degreeU+1 && p.countV == p.degreeV+1 {
				if result.ExactOperations >= o.MaxOperations {
					result.Pending = append(result.Pending, pending[index:]...)
					return ErrConvergence
				}
				certificate, err := CertifySurfacePairSection(p, l, SurfacePairIntervalOptions{UseFixedParameter: true, FixedAxis: 2, FixedParameter: 0, MaxOperations: o.MaxOperations - result.ExactOperations, MaxCoefficientBits: o.MaxCoefficientBits})
				result.ExactOperations += certificate.ExactOperations
				if err != nil {
					result.Pending = append(result.Pending, pending[index:]...)
					return err
				}
				if certificate.ExcludesContact {
					continue
				}
				if certificate.ProvenUnique {
					r := certificate.KrawczykBounds
					bounds, ok := p.bounds.Intersect(l.bounds)
					if !ok {
						result.Pending = append(result.Pending, pending[index:]...)
						return ErrInvalid
					}
					hit := LineSplineSurfaceHit{Cell: p.Domain(), LineRange: ParameterSpan{Start: t0, End: t1}, UV: UVBox{Vec2{r[0].Min, r[1].Min}, Vec2{r[0].Max, r[1].Max}}, ParameterMin: r[2].Min, ParameterMax: r[2].Max, Bounds: bounds}
					same, ambiguous := false, false
					for _, previous := range result.Hits {
						if lineHitRootInside(hit, previous) || lineHitRootInside(previous, hit) {
							same = true
						} else if lineHitCellsOverlap(hit, previous) {
							ambiguous = true
						}
					}
					if same {
						continue
					}
					if !ambiguous {
						result.Hits = append(result.Hits, hit)
						continue
					}
				}
			}
		} else {
			result.ExactOperations += budget.operations
		}
		if e != nil {
			result.Pending = append(result.Pending, pending[index:]...)
			return e
		}
		result.Pending = append(result.Pending, cell)
	}
	return nil
}
