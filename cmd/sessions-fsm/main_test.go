package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPlusTimer(t *testing.T) {
	var plusTimerTests = []struct {
		initial time.Duration
		plusTimes,
		expectErrAt int
	}{
		{
			initial:     time.Duration(0),
			plusTimes:   1,
			expectErrAt: -1,
		},
		{
			initial:     maxRemainingTime - remainingTimerAdjustmentInterval,
			plusTimes:   1,
			expectErrAt: -1,
		},
		{
			initial:     maxRemainingTime - remainingTimerAdjustmentInterval,
			plusTimes:   2,
			expectErrAt: 1,
		},
	}
	for _, tt := range plusTimerTests {
		t.Run(
			fmt.Sprintf("initial %v plusTimes %v", tt.initial, tt.plusTimes),
			func(t *testing.T) {
				s := newStateMachine(tt.initial)

				for i := range tt.plusTimes {
					err := s.PlusTimer(t.Context())
					if i == tt.expectErrAt {
						require.Error(t, err)
					} else {
						require.NoError(t, err)
					}
				}
			},
		)
	}
}

func TestMinusTimer(t *testing.T) {
	var minusTimerTests = []struct {
		initial time.Duration
		minusTimes,
		expectErrAt int
	}{
		{
			initial:     maxRemainingTime,
			minusTimes:  1,
			expectErrAt: -1,
		},
		{
			initial:     minRemainingTime + remainingTimerAdjustmentInterval,
			minusTimes:  1,
			expectErrAt: -1,
		},
		{
			initial:     minRemainingTime + remainingTimerAdjustmentInterval,
			minusTimes:  2,
			expectErrAt: 1,
		},
	}
	for _, tt := range minusTimerTests {
		t.Run(
			fmt.Sprintf("initial %v plusTimes %v", tt.initial, tt.minusTimes),
			func(t *testing.T) {
				s := newStateMachine(tt.initial)

				for i := range tt.minusTimes {
					err := s.MinusTimer(t.Context())
					if i == tt.expectErrAt {
						require.Error(t, err)
					} else {
						require.NoError(t, err)
					}
				}
			},
		)
	}
}

func TestStartDragging(t *testing.T) {
	var startDraggingTests = []struct {
		name      string
		prepare   func(*stateMachine) error
		expectErr bool
	}{
		{
			name: "can transition from initial state to dragging",
			prepare: func(sm *stateMachine) error {
				return nil
			},
			expectErr: false,
		},
		{
			name: "can not transition from dragging state to dragging",
			prepare: func(sm *stateMachine) error {
				return sm.StartDragging(t.Context())
			},
			expectErr: true,
		},
	}
	for _, tt := range startDraggingTests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				s := newStateMachine(0)

				require.NoError(t, tt.prepare(s))

				err := s.StartDragging(t.Context())
				if tt.expectErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			},
		)
	}
}

func TestStopDragging(t *testing.T) {
	var stopDraggingTests = []struct {
		name          string
		remainingTime time.Duration
		prepare       func(*stateMachine) error
		expectErr     bool
	}{
		{
			name:          "can transition from dragging state to counting down state with valid initial remaining time",
			remainingTime: initialRemainingTime,
			prepare: func(sm *stateMachine) error {
				return sm.StartDragging(t.Context())
			},
			expectErr: false,
		},
		{
			name:          "can not transition from dragging state to counting down state with initial remaining time below minimum remaining time",
			remainingTime: minRemainingTime - remainingTimerAdjustmentInterval,
			prepare: func(sm *stateMachine) error {
				return sm.StartDragging(t.Context())
			},
			expectErr: true,
		},
		{
			name:          "can not transition from dragging state to counting down state with initial remaining time above maximum remaining time",
			remainingTime: maxRemainingTime + remainingTimerAdjustmentInterval,
			prepare: func(sm *stateMachine) error {
				return sm.StartDragging(t.Context())
			},
			expectErr: true,
		},
		{
			name:          "can not transition from dragging state to counting down state with initial remaining time that's not divisible by remainingTimerAdjustmentInterval",
			remainingTime: initialRemainingTime + time.Millisecond*50,
			prepare: func(sm *stateMachine) error {
				return sm.StartDragging(t.Context())
			},
			expectErr: true,
		},
		{
			name:          "can not transition from initial to counting down state",
			remainingTime: initialRemainingTime,
			prepare: func(sm *stateMachine) error {
				return nil
			},
			expectErr: true,
		},
	}
	for _, tt := range stopDraggingTests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				s := newStateMachine(0)

				require.NoError(t, tt.prepare(s))

				err := s.StopDragging(t.Context(), tt.remainingTime)
				if tt.expectErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			},
		)
	}
}
