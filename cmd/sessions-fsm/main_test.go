// TODO: In tests, assert whether the right hooks get called for each transition everywhere
// TODO: In tests, use synctest to test the timers correctly (see https://pkg.go.dev/testing/synctest)
package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/neilotoole/slogt"
	"github.com/stretchr/testify/require"
)

func newTestingStateMachine(t *testing.T, remainingTime time.Duration, hooks *hooks) *stateMachine {
	return newStateMachine(t.Context(), remainingTime, slogt.New(t), hooks)
}

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
		for _, fromCountingDown := range []bool{true, false} {
			t.Run(
				fmt.Sprintf("initial %v plusTimes %v fromCountingDown %v", tt.initial, tt.plusTimes, fromCountingDown),
				func(t *testing.T) {
					s := newTestingStateMachine(
						t,
						tt.initial,
						&hooks{
							onBeforeStartingTimer: func(ctx context.Context) error { return nil },
							onAfterStartingTimer:  func(ctx context.Context) error { return nil },

							onRemainingTimeTick: func(ctx context.Context, currentRemainingTime time.Duration) error { return nil },

							onBeforeStoppingTimer: func(ctx context.Context) error { return nil },
							onAfterStoppingTimer:  func(ctx context.Context) error { return nil },

							onStartAlarm: func(ctx context.Context) error { return nil },

							onStopAlarm: func(ctx context.Context) error { return nil },
						},
					)

					if fromCountingDown {
						require.NoError(t, s.StartDragging(t.Context()))
						require.NoError(t, s.StopDragging(t.Context(), tt.initial))
					}

					expectedInitialRemainingTime := tt.initial
					for i := range tt.plusTimes {
						err := s.PlusTimer(t.Context())
						if i == tt.expectErrAt {
							require.Error(t, err)
						} else {
							require.NoError(t, err)

							expectedInitialRemainingTime += remainingTimerAdjustmentInterval
						}
					}

					require.Equal(t, expectedInitialRemainingTime, s.initialRemainingTime)
				},
			)
		}
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
		for _, fromCountingDown := range []bool{true, false} {
			t.Run(
				fmt.Sprintf("initial %v plusTimes %v fromCountingDown %v", tt.initial, tt.minusTimes, fromCountingDown),
				func(t *testing.T) {
					s := newTestingStateMachine(
						t,
						tt.initial,
						&hooks{
							onBeforeStartingTimer: func(ctx context.Context) error { return nil },
							onAfterStartingTimer:  func(ctx context.Context) error { return nil },

							onRemainingTimeTick: func(ctx context.Context, currentRemainingTime time.Duration) error { return nil },

							onBeforeStoppingTimer: func(ctx context.Context) error { return nil },
							onAfterStoppingTimer:  func(ctx context.Context) error { return nil },

							onStartAlarm: func(ctx context.Context) error { return nil },

							onStopAlarm: func(ctx context.Context) error { return nil },
						},
					)

					if fromCountingDown {
						require.NoError(t, s.StartDragging(t.Context()))
						require.NoError(t, s.StopDragging(t.Context(), tt.initial))
					}

					expectedInitialRemainingTime := tt.initial
					for i := range tt.minusTimes {
						err := s.MinusTimer(t.Context())
						if i == tt.expectErrAt {
							require.Error(t, err)
						} else {
							require.NoError(t, err)

							expectedInitialRemainingTime -= remainingTimerAdjustmentInterval
						}
					}

					require.Equal(t, expectedInitialRemainingTime, s.initialRemainingTime)
				},
			)
		}
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
		{
			name: "can transition from counting down state to dragging",
			prepare: func(sm *stateMachine) error {
				if err := sm.StartDragging(t.Context()); err != nil {
					return err
				}

				return sm.StopDragging(t.Context(), initialRemainingTime)
			},
			expectErr: false,
		},
		{
			name: "can not transition from alarming state to dragging",
			prepare: func(sm *stateMachine) error {
				if err := sm.StartTimer(t.Context()); err != nil {
					return err
				}

				return sm.timerFinished(t.Context())
			},
			expectErr: true,
		},
	}
	for _, tt := range startDraggingTests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				s := newTestingStateMachine(
					t,
					0,
					&hooks{
						onBeforeStartingTimer: func(ctx context.Context) error { return nil },
						onAfterStartingTimer:  func(ctx context.Context) error { return nil },

						onRemainingTimeTick: func(ctx context.Context, currentRemainingTime time.Duration) error { return nil },

						onBeforeStoppingTimer: func(ctx context.Context) error { return nil },
						onAfterStoppingTimer:  func(ctx context.Context) error { return nil },

						onStartAlarm: func(ctx context.Context) error { return nil },

						onStopAlarm: func(ctx context.Context) error { return nil },
					},
				)

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
		onBeforeStartingTimerCalled,
		onAfterStartingTimerCalled int
	}{
		{
			name:          "can transition from dragging state to counting down state with valid initial remaining time",
			remainingTime: initialRemainingTime,
			prepare: func(sm *stateMachine) error {
				return sm.StartDragging(t.Context())
			},
			expectErr:                   false,
			onBeforeStartingTimerCalled: 1,
			onAfterStartingTimerCalled:  1,
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
				var (
					onBeforeStartingTimerCalled = 0
					onAfterStartingTimerCalled  = 0
				)
				s := newTestingStateMachine(
					t,
					0,
					&hooks{
						onBeforeStartingTimer: func(ctx context.Context) error {
							onBeforeStartingTimerCalled++

							return nil
						},
						onAfterStartingTimer: func(ctx context.Context) error {
							onAfterStartingTimerCalled++

							return nil
						},

						onRemainingTimeTick: func(ctx context.Context, currentRemainingTime time.Duration) error { return nil },

						onBeforeStoppingTimer: func(ctx context.Context) error { return nil },
						onAfterStoppingTimer:  func(ctx context.Context) error { return nil },

						onStartAlarm: func(ctx context.Context) error { return nil },

						onStopAlarm: func(ctx context.Context) error { return nil },
					},
				)

				require.NoError(t, tt.prepare(s))

				err := s.StopDragging(t.Context(), tt.remainingTime)
				if tt.expectErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)

					require.Equal(t, tt.remainingTime, s.initialRemainingTime)
				}

				require.Equal(t, tt.onBeforeStartingTimerCalled, onBeforeStartingTimerCalled)
				require.Equal(t, tt.onAfterStartingTimerCalled, onAfterStartingTimerCalled)
			},
		)
	}
}

func TestStopTimer(t *testing.T) {
	var stopTimerTests = []struct {
		name          string
		remainingTime time.Duration
		prepare       func(*stateMachine) error
		expectErr     bool
		onBeforeStoppingTimerCalled,
		onAfterStoppingTimerCalled int
	}{
		{
			name: "can transition from counting down state to stopped state",
			prepare: func(sm *stateMachine) error {
				if err := sm.StartDragging(t.Context()); err != nil {
					return err
				}

				return sm.StopDragging(t.Context(), initialRemainingTime)
			},
			expectErr:                   false,
			onBeforeStoppingTimerCalled: 1,
			onAfterStoppingTimerCalled:  1,
		},
		{
			name: "can not transition from dragging state to stopped state",
			prepare: func(sm *stateMachine) error {
				return sm.StartDragging(t.Context())
			},
			expectErr: true,
		},
		{
			name: "can not transition from alarming state to stopped state",
			prepare: func(sm *stateMachine) error {
				if err := sm.StartTimer(t.Context()); err != nil {
					return err
				}

				return sm.timerFinished(t.Context())
			},
			expectErr: true,
		},
	}
	for _, tt := range stopTimerTests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				var (
					onBeforeStoppingTimerCalled = 0
					onAfterStoppingTimerCalled  = 0
				)
				s := newTestingStateMachine(
					t,
					0,
					&hooks{
						onBeforeStartingTimer: func(ctx context.Context) error { return nil },
						onAfterStartingTimer:  func(ctx context.Context) error { return nil },

						onRemainingTimeTick: func(ctx context.Context, currentRemainingTime time.Duration) error { return nil },

						onBeforeStoppingTimer: func(ctx context.Context) error {
							onBeforeStoppingTimerCalled++

							return nil
						},
						onAfterStoppingTimer: func(ctx context.Context) error {
							onAfterStoppingTimerCalled++

							return nil
						},

						onStartAlarm: func(ctx context.Context) error { return nil },

						onStopAlarm: func(ctx context.Context) error { return nil },
					},
				)

				require.NoError(t, tt.prepare(s))

				err := s.StopTimer(t.Context())
				if tt.expectErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}

				require.Equal(t, tt.onBeforeStoppingTimerCalled, onBeforeStoppingTimerCalled)
				require.Equal(t, tt.onAfterStoppingTimerCalled, onAfterStoppingTimerCalled)
			},
		)
	}
}

func TestStartTimer(t *testing.T) {
	var startTimerTests = []struct {
		name      string
		prepare   func(*stateMachine) error
		expectErr bool
		onBeforeStartingTimerCalled,
		onAfterStartingTimerCalled int
	}{
		{
			name: "can transition from stopped state to counting down state",
			prepare: func(sm *stateMachine) error {
				return nil
			},
			expectErr:                   false,
			onBeforeStartingTimerCalled: 1,
			onAfterStartingTimerCalled:  1,
		},
		{
			name: "can not transition from dragging state to counting down state",
			prepare: func(sm *stateMachine) error {
				return sm.StartDragging(t.Context())
			},
			expectErr: true,
		},
	}
	for _, tt := range startTimerTests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				var (
					onBeforeStartingTimerCalled = 0
					onAfterStartingTimerCalled  = 0
				)
				s := newTestingStateMachine(
					t,
					0,
					&hooks{
						onBeforeStartingTimer: func(ctx context.Context) error {
							onBeforeStartingTimerCalled++

							return nil
						},
						onAfterStartingTimer: func(ctx context.Context) error {
							onAfterStartingTimerCalled++

							return nil
						},

						onRemainingTimeTick: func(ctx context.Context, currentRemainingTime time.Duration) error { return nil },

						onBeforeStoppingTimer: func(ctx context.Context) error { return nil },
						onAfterStoppingTimer:  func(ctx context.Context) error { return nil },

						onStartAlarm: func(ctx context.Context) error { return nil },

						onStopAlarm: func(ctx context.Context) error { return nil },
					},
				)

				require.NoError(t, tt.prepare(s))

				err := s.StartTimer(t.Context())
				if tt.expectErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}

				require.Equal(t, tt.onBeforeStartingTimerCalled, onBeforeStartingTimerCalled)
				require.Equal(t, tt.onAfterStartingTimerCalled, onAfterStartingTimerCalled)
			},
		)
	}
}

func TestTimerFinished(t *testing.T) {
	var timerFinishedTests = []struct {
		name               string
		prepare            func(*stateMachine) error
		expectErr          bool
		onStartAlarmCalled int
	}{
		{
			name: "can transition from counting down state to alarming state",
			prepare: func(sm *stateMachine) error {
				return sm.StartTimer(t.Context())
			},
			expectErr:          false,
			onStartAlarmCalled: 1,
		},
		{
			name: "can not transition from dragging state to alarming state",
			prepare: func(sm *stateMachine) error {
				return sm.StartDragging(t.Context())
			},
			expectErr: true,
		},
	}
	for _, tt := range timerFinishedTests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				onStartAlarmCalled := 0
				s := newTestingStateMachine(
					t,
					0,
					&hooks{
						onBeforeStartingTimer: func(ctx context.Context) error { return nil },
						onAfterStartingTimer:  func(ctx context.Context) error { return nil },

						onRemainingTimeTick: func(ctx context.Context, currentRemainingTime time.Duration) error { return nil },

						onBeforeStoppingTimer: func(ctx context.Context) error { return nil },
						onAfterStoppingTimer:  func(ctx context.Context) error { return nil },

						onStartAlarm: func(ctx context.Context) error {
							onStartAlarmCalled++

							return nil
						},

						onStopAlarm: func(ctx context.Context) error { return nil },
					},
				)

				require.NoError(t, tt.prepare(s))

				err := s.timerFinished(t.Context())
				if tt.expectErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}

				require.Equal(t, tt.onStartAlarmCalled, onStartAlarmCalled)
			},
		)
	}
}

func TestStopAlarming(t *testing.T) {
	var stopAlarmingTests = []struct {
		name              string
		prepare           func(*stateMachine) error
		expectErr         bool
		onStopAlarmCalled int
	}{
		{
			name: "can transition from alarming state state to stopped state",
			prepare: func(sm *stateMachine) error {
				if err := sm.StartTimer(t.Context()); err != nil {
					return err
				}

				return sm.timerFinished(t.Context())
			},
			expectErr:         false,
			onStopAlarmCalled: 1,
		},
		{
			name: "can not transition from counting down state to stopped state",
			prepare: func(sm *stateMachine) error {
				return sm.StartTimer(t.Context())
			},
			expectErr: true,
		},
	}
	for _, tt := range stopAlarmingTests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				onStopAlarmCalled := 0
				s := newTestingStateMachine(
					t,
					0,
					&hooks{
						onBeforeStartingTimer: func(ctx context.Context) error { return nil },
						onAfterStartingTimer:  func(ctx context.Context) error { return nil },

						onRemainingTimeTick: func(ctx context.Context, currentRemainingTime time.Duration) error { return nil },

						onBeforeStoppingTimer: func(ctx context.Context) error { return nil },
						onAfterStoppingTimer:  func(ctx context.Context) error { return nil },

						onStartAlarm: func(ctx context.Context) error { return nil },

						onStopAlarm: func(ctx context.Context) error {
							onStopAlarmCalled++

							return nil
						},
					},
				)

				require.NoError(t, tt.prepare(s))

				err := s.StopAlarming(t.Context())
				if tt.expectErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}

				require.Equal(t, tt.onStopAlarmCalled, onStopAlarmCalled)
			},
		)
	}
}
