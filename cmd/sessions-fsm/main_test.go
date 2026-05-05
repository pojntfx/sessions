package main

import (
	"fmt"
	"testing"
	"time"
)

func TestPlusTimer(t *testing.T) {
	var plusTimerTests = []struct {
		initial   time.Duration
		plusTimes int
		expectErr bool
	}{
		{
			initial:   time.Duration(0),
			plusTimes: 1,
			expectErr: false,
		},
		{
			initial:   maxRemainingTime - remainingTimerAdjustmentInterval,
			plusTimes: 1,
			expectErr: false,
		},
		{
			initial:   maxRemainingTime - remainingTimerAdjustmentInterval,
			plusTimes: 2,
			expectErr: true,
		},
	}
	for _, tt := range plusTimerTests {
		t.Run(
			fmt.Sprintf("initial %v plusTimes %v", tt.initial, tt.plusTimes),
			func(t *testing.T) {
				s := newStateMachine(tt.initial)

				for i := range tt.plusTimes {
					err := s.PlusTimer(t.Context())
					if err != nil && !tt.expectErr {
						t.Errorf("i=%v got err=%v, want expectErr=%v", i, err, tt.expectErr)
					}
				}
			},
		)
	}
}

func TestMinusTimer(t *testing.T) {
	var minusTimerTests = []struct {
		initial    time.Duration
		minusTimes int
		expectErr  bool
	}{
		{
			initial:    maxRemainingTime,
			minusTimes: 1,
			expectErr:  false,
		},
		{
			initial:    minRemainingTime + remainingTimerAdjustmentInterval,
			minusTimes: 1,
			expectErr:  false,
		},
		{
			initial:    minRemainingTime + remainingTimerAdjustmentInterval,
			minusTimes: 2,
			expectErr:  true,
		},
	}
	for _, tt := range minusTimerTests {
		t.Run(
			fmt.Sprintf("initial %v plusTimes %v", tt.initial, tt.minusTimes),
			func(t *testing.T) {
				s := newStateMachine(tt.initial)

				for i := range tt.minusTimes {
					err := s.MinusTimer(t.Context())
					if err != nil && !tt.expectErr {
						t.Errorf("i=%v got err=%v, want expectErr=%v", i, err, tt.expectErr)
					}
				}
			},
		)
	}
}
