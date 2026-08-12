package scoring_test

import (
	"math"
	"testing"

	"vitract-assessment/packages/scoring"
)

func TestShannon(t *testing.T) {
	tests := []struct {
		name       string
		abundances []float32
		want       float64
	}{
		{"empty", nil, 0},
		{"all zero", []float32{0, 0, 0}, 0},
		{"single taxon", []float32{1}, 0},
		{"single taxon unnormalized", []float32{42}, 0},
		{"two equal taxa", []float32{0.5, 0.5}, math.Log(2)},
		{"four equal taxa", []float32{1, 1, 1, 1}, math.Log(4)},
		{"S1001 taxa (normalized)", []float32{0.40, 0.25, 0.20, 0.10, 0.05}, 1.41502258860058},
		{"S1001 taxa (unnormalized, same ratios)", []float32{40, 25, 20, 10, 5}, 1.41502258860058},
		{"zero abundance taxon ignored", []float32{0, 0.5, 0.5}, math.Log(2)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scoring.Shannon(tt.abundances)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("Shannon(%v) = %v, want %v", tt.abundances, got, tt.want)
			}
		})
	}
}
