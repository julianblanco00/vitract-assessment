package scoring

import (
	"math"
)

func Shannon(abundances []float32) float64 {
	if len(abundances) == 0 {
		return 0
	}

	var total float64
	for _, a := range abundances {
		total += float64(a)
	}

	if total == 0 {
		return 0
	}

	var entropy float64
	for _, a := range abundances {
		p := float64(a) / total
		if p > 0 {
			entropy += p * math.Log(p)
		}
	}

	return -entropy
}
