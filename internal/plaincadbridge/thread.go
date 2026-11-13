package plaincadbridge

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type threadRequest struct {
	Side           string `json:"side"`
	Hand           string `json:"hand"`
	Radius         number `json:"radius"`
	Pitch          number `json:"pitch"`
	Length         number `json:"length"`
	Offset         number `json:"offset"`
	InnerRadius    number `json:"innerRadius"`
	OuterRadius    number `json:"outerRadius"`
	InnerHalfWidth number `json:"innerHalfWidth"`
	OuterHalfWidth number `json:"outerHalfWidth"`
}

func (b *Bridge) threadBody(s stored, q threadRequest) (Shape, error) {
	radius, pitch, length, offset := float64(q.Radius), float64(q.Pitch), float64(q.Length), float64(q.Offset)
	inner, outer := float64(q.InnerRadius), float64(q.OuterRadius)
	a, c := float64(q.InnerHalfWidth), float64(q.OuterHalfWidth)
	if radius <= 0 || pitch <= 0 || length < pitch || length > 12*pitch+1e-9 || offset < 0 ||
		inner <= 0 || inner >= outer || min(a, c) <= 0 || max(a, c)*2 >= pitch {
		return Shape{}, errors.New("invalid thread band, pitch, length or profile")
	}
	hand := geom.RightHanded
	switch q.Hand {
	case "right":
	case "left":
		hand = geom.LeftHanded
	default:
		return Shape{}, errors.New("thread requires right or left hand")
	}
	var recipe trimmed.HelicalGrooveRecipe
	var head *trimmed.SteppedShaft
	entryRadius := inner - pitch*.02
	switch q.Side {
	case "external":
		if radius != outer {
			return Shape{}, errors.New("external thread radius must match the shaft")
		}
		if s.cylinder != nil && s.cylinder.radius == radius {
			recipe.Frame, recipe.Height = s.cylinder.frame, s.cylinder.height
		} else if s.analytic != nil && s.analytic.shaft != nil && s.analytic.shaft.ShaftRadius == radius {
			head = s.analytic.shaft
			f := head.Frame
			frame, err := geom.NewFrame(f.ToWorld(geom.Vec3{Z: -head.ShaftLength}), f.XAxis(), f.ZAxis())
			if err != nil {
				return Shape{}, err
			}
			recipe.Frame, recipe.Height = frame, head.ShaftLength
		} else {
			return Shape{}, &failureError{"UNSUPPORTED", "Select one complete nominal-diameter shaft for the modeled thread"}
		}
		recipe.OuterRadius = radius
	case "internal":
		if radius != inner {
			return Shape{}, errors.New("internal thread radius must match the bore")
		}
		band := s.analytic
		if band == nil || band.sector == nil || band.sector.inner != radius || band.sector.angle != 2*math.Pi {
			return Shape{}, &failureError{"UNSUPPORTED", "Select one complete coaxial tap-drill bore for the modeled thread"}
		}
		r := band.sector
		recipe.Frame, recipe.Height, recipe.BoreRadius, recipe.OuterRadius = r.frame, r.height, r.inner, r.radius
		entryRadius = outer + pitch*.02
	default:
		return Shape{}, errors.New("thread requires an external shaft or internal bore")
	}
	if !cardinalFrame(recipe.Frame) {
		return Shape{}, &failureError{"UNSUPPORTED", "thread requires a represented coordinate axis"}
	}
	margin := pitch * .55
	if length+offset+2*margin > recipe.Height+1e-9 {
		return Shape{}, errors.New("Thread length and offset exceed the cylinder after reserving runout space at both ends")
	}
	center := margin + offset
	recipe.Profile = threadProfile(inner, outer, center, a, c)
	recipe.Lead, recipe.End, recipe.Hand, recipe.Starts = pitch, length/pitch, hand, 1
	depth := center + pitch*.19
	ctx := context.Background()
	var solid trimmed.Solid
	var removedPi *big.Rat
	if head != nil {
		var err error
		solid, removedPi, err = trimmed.SteppedShaftWithGrooveAndEntryRelief(ctx, *head, recipe, entryRadius, depth, threadChartBudget(recipe))
		if err != nil {
			return Shape{}, fmt.Errorf("thread head attachment: %w", err)
		}
	} else {
		groove, err := trimmed.GrooveWithOverlappingEntryRelief(ctx, recipe, entryRadius, depth, threadChartBudget(recipe))
		if err != nil {
			return Shape{}, fmt.Errorf("thread groove: %w", err)
		}
		solid = groove.Solid
		removedPi = new(big.Rat).Add(groove.GrooveVolumePiCoefficient, groove.EntryVolumePiCoefficient)
	}
	removed, _ := removedPi.Float64()
	volume := s.result.GeometryAssertions.Volume - math.Pi*removed
	area, err := solid.HelicalSurfaceArea(ctx)
	if err != nil {
		return Shape{}, fmt.Errorf("thread surface measures: %w", err)
	}
	return b.putAnalyticBody(&analyticBody{solid: solid, volume: volume, area: area, helical: true})
}

func threadProfile(inner, outer, center, innerHalf, outerHalf float64) []geom.Vec2 {
	return []geom.Vec2{{X: inner, Y: center - innerHalf}, {X: outer, Y: center - outerHalf},
		{X: outer, Y: center + outerHalf}, {X: inner, Y: center + innerHalf}}
}

func threadChartBudget(r trimmed.HelicalGrooveRecipe) float64 {
	return min(1e-10, max(r.OuterRadius, r.Height)*1e-12)
}
