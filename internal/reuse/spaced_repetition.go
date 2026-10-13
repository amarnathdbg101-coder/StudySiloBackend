package reuse

import (
	"math"
	"time"
)

// SM2Item holds spaced repetition state for an active recall flashcard or study topic
type SM2Item struct {
	Repetitions int       `json:"repetitions"`
	Interval    int       `json:"interval_days"`
	EaseFactor  float64   `json:"ease_factor"`
	NextReview  time.Time `json:"next_review"`
}

// CalculateSM2 calculates the next review interval using SuperMemo SM-2 algorithm based on user quality rating (0-5)
func CalculateSM2(item SM2Item, quality int) SM2Item {
	if quality < 0 {
		quality = 0
	}
	if quality > 5 {
		quality = 5
	}

	if item.EaseFactor < 1.3 {
		item.EaseFactor = 2.5
	}

	if quality >= 3 {
		if item.Repetitions == 0 {
			item.Interval = 1
		} else if item.Repetitions == 1 {
			item.Interval = 6
		} else {
			item.Interval = int(math.Round(float64(item.Interval) * item.EaseFactor))
		}
		item.Repetitions++
	} else {
		item.Repetitions = 0
		item.Interval = 1
	}

	qDiff := float64(5 - quality)
	item.EaseFactor = item.EaseFactor + (0.1 - qDiff*(0.08+qDiff*0.02))
	if item.EaseFactor < 1.3 {
		item.EaseFactor = 1.3
	}

	item.NextReview = time.Now().Add(time.Duration(item.Interval) * 24 * time.Hour)
	return item
}
