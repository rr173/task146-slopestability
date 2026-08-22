package geotech

import (
	"math"

	"task146-slopestability/internal/model"
)

// GridBounds defines the centre/radius search grid for a critical-surface
// search. Step values of 0 default to a coarse auto step.
type GridBounds struct {
	CxMin, CxMax, CzMin, CzMax, RMin, RMax float64
	CxStep, CzStep, RStep                  float64
}

// SearchCritical enumerates every (cx, cz, R) on the grid, solves the requested
// method for each legal surface, and returns the minimum-F surface plus coverage
// counters (total / evaluated / skipped) — illegal geometries (no underground
// arc) are counted as skipped, never silently dropped (boundary constraint #4).
//
// The method is honoured so a Janbu/Fellenius critical search ranks surfaces by
// that method's F, not Bishop's; an empty method falls back to Bishop for
// backward compatibility.
func SearchCritical(in SolveInput, grid GridBounds, method model.AnalysisMethod) (model.SearchSummary, error) {
	if method == "" {
		method = model.MethodBishop
	}
	solve := solverFor(method)
	if solve == nil {
		return model.SearchSummary{}, ErrUnknownMethod
	}
	if grid.CxStep <= 0 {
		grid.CxStep = (grid.CxMax - grid.CxMin) / 4
	}
	if grid.CzStep <= 0 {
		grid.CzStep = (grid.CzMax - grid.CzMin) / 4
	}
	if grid.RStep <= 0 {
		grid.RStep = (grid.RMax - grid.RMin) / 4
	}
	if grid.CxStep <= 0 {
		grid.CxStep = 1
	}
	if grid.CzStep <= 0 {
		grid.CzStep = 1
	}
	if grid.RStep <= 0 {
		grid.RStep = 1
	}

	var summary model.SearchSummary
	minF := math.MaxFloat64
	for cx := grid.CxMin; cx <= grid.CxMax+Epsilon; cx += grid.CxStep {
		for cz := grid.CzMin; cz <= grid.CzMax+Epsilon; cz += grid.CzStep {
			for r := grid.RMin; r <= grid.RMax+Epsilon; r += grid.RStep {
				summary.Total++
				run := in
				run.Cx, run.Cz, run.R = cx, cz, r
				run.Reinforcements = nil // baseline surfaces in the search
				res, err := solve(run)
				if err != nil || !res.Converged {
					summary.Skipped++
					continue
				}
				summary.Evaluated++
				if res.F < minF {
					minF = res.F
					summary.MinF = res.F
					summary.BestCx = cx
					summary.BestCz = cz
					summary.BestR = r
					summary.Converged = true
				}
			}
		}
	}
	if !summary.Converged {
		return summary, ErrGeometry
	}
	return summary, nil
}
