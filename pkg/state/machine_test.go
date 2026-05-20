package state

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/neilotoole/slogt"
	"github.com/stretchr/testify/require"
)

func newTestingStateMachine(t *testing.T, remainingTime time.Duration, hooks *Hooks) *StateMachine {
	return NewStateMachine(t.Context(), remainingTime, slogt.New(t), hooks)
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
			initial:     maxRemainingTime - RemainingTimerAdjustmentInterval,
			plusTimes:   1,
			expectErrAt: -1,
		},
		{
			initial:     maxRemainingTime - RemainingTimerAdjustmentInterval,
			plusTimes:   2,
			expectErrAt: 1,
		},
	}
	for _, tt := range plusTimerTests {
		for _, fromCountingDown := range []bool{true, false} {
			t.Run(
				fmt.Sprintf("initial %v plusTimes %v fromCountingDown %v", tt.initial, tt.plusTimes, fromCountingDown),
				func(t *testing.T) {
					var (
						onBeforeStartingTimerCalled = 0
						onAfterStartingTimerCalled  = 0

						internalInitialRemainingTime = time.Duration(0)
					)
					s := newTestingStateMachine(
						t,
						tt.initial,
						&Hooks{
							OnBeforeStartingTimer: func(ctx context.Context) error {
								onBeforeStartingTimerCalled++

								return nil
							},
							OnAfterStartingTimer: func(ctx context.Context) error {
								onAfterStartingTimerCalled++

								return nil
							},

							OnInitialRemainingTimeChange: func(ctx context.Context, initialRemainingTime time.Duration) error {
								internalInitialRemainingTime = initialRemainingTime

								return nil
							},
							OnCurrentRemainingTimeTick: func(ctx context.Context, currentRemainingTime time.Duration) error { return nil },

							OnBeforeStoppingTimer: func(ctx context.Context) error { return nil },
							OnAfterStoppingTimer:  func(ctx context.Context) error { return nil },

							OnStartAlarm: func(ctx context.Context) error { return nil },

							OnStopAlarm: func(ctx context.Context) error { return nil },
						},
					)

					if fromCountingDown {
						require.NoError(t, s.StartDragging(t.Context()))
						require.NoError(t, s.StopDragging(t.Context(), tt.initial))

						// After we stop dragging, the timer should be running.
						// We only assert the initial remaining time and assert whether the timer starts running
						// in TestEndToEnd, not here, this test does not have a race
						require.Equal(t, 1, onBeforeStartingTimerCalled)
						require.Equal(t, 1, onAfterStartingTimerCalled)
					}

					expectedInitialRemainingTime := tt.initial
					for i := range tt.plusTimes {
						err := s.PlusTimer(t.Context())
						if i == tt.expectErrAt {
							require.Error(t, err)
						} else {
							require.NoError(t, err)

							expectedInitialRemainingTime += RemainingTimerAdjustmentInterval
						}
					}

					if fromCountingDown {
						// The timer should still be running if we increased during the counting down phase
						require.Equal(t, 2, onBeforeStartingTimerCalled)
						require.Equal(t, 2, onAfterStartingTimerCalled)
					}

					require.Equal(t, expectedInitialRemainingTime, internalInitialRemainingTime)
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
			initial:     minRemainingTime + RemainingTimerAdjustmentInterval,
			minusTimes:  1,
			expectErrAt: -1,
		},
		{
			initial:     minRemainingTime + RemainingTimerAdjustmentInterval,
			minusTimes:  2,
			expectErrAt: 1,
		},
	}
	for _, tt := range minusTimerTests {
		for _, fromCountingDown := range []bool{true, false} {
			t.Run(
				fmt.Sprintf("initial %v plusTimes %v fromCountingDown %v", tt.initial, tt.minusTimes, fromCountingDown),
				func(t *testing.T) {
					var (
						onBeforeStartingTimerCalled = 0
						onAfterStartingTimerCalled  = 0

						internalInitialRemainingTime = time.Duration(0)
					)
					s := newTestingStateMachine(
						t,
						tt.initial,
						&Hooks{
							OnBeforeStartingTimer: func(ctx context.Context) error {
								onBeforeStartingTimerCalled++

								return nil
							},
							OnAfterStartingTimer: func(ctx context.Context) error {
								onAfterStartingTimerCalled++

								return nil
							},

							OnInitialRemainingTimeChange: func(ctx context.Context, initialRemainingTime time.Duration) error {
								internalInitialRemainingTime = initialRemainingTime

								return nil
							},
							OnCurrentRemainingTimeTick: func(ctx context.Context, currentRemainingTime time.Duration) error { return nil },

							OnBeforeStoppingTimer: func(ctx context.Context) error { return nil },
							OnAfterStoppingTimer:  func(ctx context.Context) error { return nil },

							OnStartAlarm: func(ctx context.Context) error { return nil },

							OnStopAlarm: func(ctx context.Context) error { return nil },
						},
					)

					if fromCountingDown {
						require.NoError(t, s.StartDragging(t.Context()))
						require.NoError(t, s.StopDragging(t.Context(), tt.initial))

						// After we stop dragging, the timer should be running.
						// We only assert the initial remaining time and assert whether the timer starts running
						// in TestEndToEnd, not here, this test does not have a race
						require.Equal(t, 1, onBeforeStartingTimerCalled)
						require.Equal(t, 1, onAfterStartingTimerCalled)
					}

					expectedInitialRemainingTime := tt.initial
					for i := range tt.minusTimes {
						err := s.MinusTimer(t.Context())
						if i == tt.expectErrAt {
							require.Error(t, err)
						} else {
							require.NoError(t, err)

							expectedInitialRemainingTime -= RemainingTimerAdjustmentInterval
						}
					}

					if fromCountingDown {
						// The timer should still be running if we increased during the counting down phase
						require.Equal(t, 2, onBeforeStartingTimerCalled)
						require.Equal(t, 2, onAfterStartingTimerCalled)
					}

					require.Equal(t, expectedInitialRemainingTime, internalInitialRemainingTime)
				},
			)
		}
	}
}

func TestStartDragging(t *testing.T) {
	var startDraggingTests = []struct {
		name         string
		prepare      func(*StateMachine) error
		expectErr    bool
		postRunCheck func(*StateMachine) error
	}{
		{
			name: "can transition from initial state to dragging",
			prepare: func(sm *StateMachine) error {
				return nil
			},
			expectErr: false,
		},
		{
			name: "can not transition from dragging state to dragging",
			prepare: func(sm *StateMachine) error {
				return sm.StartDragging(t.Context())
			},
			expectErr: true,
		},
		{
			name: "can transition from counting down state to dragging",
			prepare: func(sm *StateMachine) error {
				if err := sm.StartDragging(t.Context()); err != nil {
					return err
				}

				return sm.StopDragging(t.Context(), DefaultInitialRemainingTime)
			},
			expectErr: false,
		},
		{
			name: "can not transition from alarming state to dragging",
			prepare: func(sm *StateMachine) error {
				if err := sm.StartTimer(t.Context()); err != nil {
					return err
				}

				return sm.timerFinished(t.Context())
			},
			expectErr: true,
		},
		{
			name: "transitioning from counting down state to dragging cancels running timer",
			prepare: func(sm *StateMachine) error {
				if err := sm.StartDragging(t.Context()); err != nil {
					return err
				}

				return sm.StopDragging(t.Context(), DefaultInitialRemainingTime)
			},
			expectErr: false,
			postRunCheck: func(sm *StateMachine) error {
				if sm.tickerCtx.Err() == nil {
					return errors.New("timer is still running")
				}

				return nil
			},
		},
	}
	for _, tt := range startDraggingTests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				s := newTestingStateMachine(
					t,
					0,
					&Hooks{
						OnBeforeStartingTimer: func(ctx context.Context) error { return nil },
						OnAfterStartingTimer:  func(ctx context.Context) error { return nil },

						OnInitialRemainingTimeChange: func(ctx context.Context, initialRemainingTime time.Duration) error { return nil },
						OnCurrentRemainingTimeTick:   func(ctx context.Context, currentRemainingTime time.Duration) error { return nil },

						OnBeforeStoppingTimer: func(ctx context.Context) error { return nil },
						OnAfterStoppingTimer:  func(ctx context.Context) error { return nil },

						OnStartAlarm: func(ctx context.Context) error { return nil },

						OnStopAlarm: func(ctx context.Context) error { return nil },
					},
				)

				require.NoError(t, tt.prepare(s))

				err := s.StartDragging(t.Context())
				if tt.expectErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}

				if hook := tt.postRunCheck; hook != nil {
					require.NoError(t, hook(s))
				}
			},
		)
	}
}

func TestStopDragging(t *testing.T) {
	var stopDraggingTests = []struct {
		name          string
		remainingTime time.Duration
		prepare       func(*StateMachine) error
		expectErr     bool
		onBeforeStartingTimerCalled,
		onAfterStartingTimerCalled int
	}{
		{
			name:          "can transition from dragging state to counting down state with valid initial remaining time",
			remainingTime: DefaultInitialRemainingTime,
			prepare: func(sm *StateMachine) error {
				return sm.StartDragging(t.Context())
			},
			expectErr:                   false,
			onBeforeStartingTimerCalled: 1,
			onAfterStartingTimerCalled:  1,
		},
		{
			name:          "can not transition from dragging state to counting down state with initial remaining time below minimum remaining time",
			remainingTime: minRemainingTime - RemainingTimerAdjustmentInterval,
			prepare: func(sm *StateMachine) error {
				return sm.StartDragging(t.Context())
			},
			expectErr: true,
		},
		{
			name:          "can not transition from dragging state to counting down state with initial remaining time above maximum remaining time",
			remainingTime: maxRemainingTime + RemainingTimerAdjustmentInterval,
			prepare: func(sm *StateMachine) error {
				return sm.StartDragging(t.Context())
			},
			expectErr: true,
		},
		{
			name:          "can not transition from dragging state to counting down state with initial remaining time that's not divisible by remainingTimerAdjustmentInterval",
			remainingTime: DefaultInitialRemainingTime + time.Millisecond*50,
			prepare: func(sm *StateMachine) error {
				return sm.StartDragging(t.Context())
			},
			expectErr: true,
		},
		{
			name:          "can not transition from initial to counting down state",
			remainingTime: DefaultInitialRemainingTime,
			prepare: func(sm *StateMachine) error {
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

					internalInitialRemainingTime = time.Duration(0)
				)
				s := newTestingStateMachine(
					t,
					0,
					&Hooks{
						OnBeforeStartingTimer: func(ctx context.Context) error {
							onBeforeStartingTimerCalled++

							return nil
						},
						OnAfterStartingTimer: func(ctx context.Context) error {
							onAfterStartingTimerCalled++

							return nil
						},

						OnInitialRemainingTimeChange: func(ctx context.Context, initialRemainingTime time.Duration) error {
							internalInitialRemainingTime = initialRemainingTime

							return nil
						},
						OnCurrentRemainingTimeTick: func(ctx context.Context, currentRemainingTime time.Duration) error { return nil },

						OnBeforeStoppingTimer: func(ctx context.Context) error { return nil },
						OnAfterStoppingTimer:  func(ctx context.Context) error { return nil },

						OnStartAlarm: func(ctx context.Context) error { return nil },

						OnStopAlarm: func(ctx context.Context) error { return nil },
					},
				)

				require.NoError(t, tt.prepare(s))

				err := s.StopDragging(t.Context(), tt.remainingTime)
				if tt.expectErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)

					require.Equal(t, tt.remainingTime, internalInitialRemainingTime)
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
		prepare       func(*StateMachine) error
		expectErr     bool
		onBeforeStoppingTimerCalled,
		onAfterStoppingTimerCalled int
	}{
		{
			name: "can transition from counting down state to stopped state",
			prepare: func(sm *StateMachine) error {
				if err := sm.StartDragging(t.Context()); err != nil {
					return err
				}

				return sm.StopDragging(t.Context(), DefaultInitialRemainingTime)
			},
			expectErr:                   false,
			onBeforeStoppingTimerCalled: 1,
			onAfterStoppingTimerCalled:  1,
		},
		{
			name: "can not transition from dragging state to stopped state",
			prepare: func(sm *StateMachine) error {
				return sm.StartDragging(t.Context())
			},
			expectErr: true,
		},
		{
			name: "can not transition from alarming state to stopped state",
			prepare: func(sm *StateMachine) error {
				if err := sm.StartTimer(t.Context()); err != nil {
					return err
				}

				return sm.timerFinished(t.Context())
			},
			expectErr:                   true,
			onBeforeStoppingTimerCalled: 1,
			onAfterStoppingTimerCalled:  1,
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
					&Hooks{
						OnBeforeStartingTimer: func(ctx context.Context) error { return nil },
						OnAfterStartingTimer:  func(ctx context.Context) error { return nil },

						OnInitialRemainingTimeChange: func(ctx context.Context, initialRemainingTime time.Duration) error { return nil },
						OnCurrentRemainingTimeTick:   func(ctx context.Context, currentRemainingTime time.Duration) error { return nil },

						OnBeforeStoppingTimer: func(ctx context.Context) error {
							onBeforeStoppingTimerCalled++

							return nil
						},
						OnAfterStoppingTimer: func(ctx context.Context) error {
							onAfterStoppingTimerCalled++

							return nil
						},

						OnStartAlarm: func(ctx context.Context) error { return nil },

						OnStopAlarm: func(ctx context.Context) error { return nil },
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
		prepare   func(*StateMachine) error
		expectErr bool
		onBeforeStartingTimerCalled,
		onAfterStartingTimerCalled int
	}{
		{
			name: "can transition from stopped state to counting down state",
			prepare: func(sm *StateMachine) error {
				return nil
			},
			expectErr:                   false,
			onBeforeStartingTimerCalled: 1,
			onAfterStartingTimerCalled:  1,
		},
		{
			name: "can not transition from dragging state to counting down state",
			prepare: func(sm *StateMachine) error {
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
					&Hooks{
						OnBeforeStartingTimer: func(ctx context.Context) error {
							onBeforeStartingTimerCalled++

							return nil
						},
						OnAfterStartingTimer: func(ctx context.Context) error {
							onAfterStartingTimerCalled++

							return nil
						},

						OnInitialRemainingTimeChange: func(ctx context.Context, initialRemainingTime time.Duration) error { return nil },
						OnCurrentRemainingTimeTick:   func(ctx context.Context, currentRemainingTime time.Duration) error { return nil },

						OnBeforeStoppingTimer: func(ctx context.Context) error { return nil },
						OnAfterStoppingTimer:  func(ctx context.Context) error { return nil },

						OnStartAlarm: func(ctx context.Context) error { return nil },

						OnStopAlarm: func(ctx context.Context) error { return nil },
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
		prepare            func(*StateMachine) error
		expectErr          bool
		onStartAlarmCalled int
	}{
		{
			name: "can transition from counting down state to alarming state",
			prepare: func(sm *StateMachine) error {
				return sm.StartTimer(t.Context())
			},
			expectErr:          false,
			onStartAlarmCalled: 1,
		},
		{
			name: "can not transition from dragging state to alarming state",
			prepare: func(sm *StateMachine) error {
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
					&Hooks{
						OnBeforeStartingTimer: func(ctx context.Context) error { return nil },
						OnAfterStartingTimer:  func(ctx context.Context) error { return nil },

						OnInitialRemainingTimeChange: func(ctx context.Context, initialRemainingTime time.Duration) error { return nil },
						OnCurrentRemainingTimeTick:   func(ctx context.Context, currentRemainingTime time.Duration) error { return nil },

						OnBeforeStoppingTimer: func(ctx context.Context) error { return nil },
						OnAfterStoppingTimer:  func(ctx context.Context) error { return nil },

						OnStartAlarm: func(ctx context.Context) error {
							onStartAlarmCalled++

							return nil
						},

						OnStopAlarm: func(ctx context.Context) error { return nil },
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
		name      string
		prepare   func(*StateMachine) error
		expectErr bool
		onStopAlarmCalled,
		onBeforeStoppingTimerCalled,
		onAfterStoppingTimerCalled int
	}{
		{
			name: "can transition from alarming state state to stopped state",
			prepare: func(sm *StateMachine) error {
				if err := sm.StartTimer(t.Context()); err != nil {
					return err
				}

				return sm.timerFinished(t.Context())
			},
			expectErr:                   false,
			onStopAlarmCalled:           1,
			onBeforeStoppingTimerCalled: 1,
			onAfterStoppingTimerCalled:  1,
		},
		{
			name: "can not transition from counting down state to stopped state",
			prepare: func(sm *StateMachine) error {
				return sm.StartTimer(t.Context())
			},
			expectErr: true,
		},
	}
	for _, tt := range stopAlarmingTests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				var (
					onStopAlarmCalled           = 0
					onBeforeStoppingTimerCalled = 0
					onAfterStoppingTimerCalled  = 0
				)
				s := newTestingStateMachine(
					t,
					0,
					&Hooks{
						OnBeforeStartingTimer: func(ctx context.Context) error { return nil },
						OnAfterStartingTimer:  func(ctx context.Context) error { return nil },

						OnInitialRemainingTimeChange: func(ctx context.Context, initialRemainingTime time.Duration) error { return nil },
						OnCurrentRemainingTimeTick:   func(ctx context.Context, currentRemainingTime time.Duration) error { return nil },

						OnBeforeStoppingTimer: func(ctx context.Context) error {
							onBeforeStoppingTimerCalled++

							return nil
						},
						OnAfterStoppingTimer: func(ctx context.Context) error {
							onAfterStoppingTimerCalled++

							return nil
						},

						OnStartAlarm: func(ctx context.Context) error { return nil },

						OnStopAlarm: func(ctx context.Context) error {
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
				require.Equal(t, tt.onBeforeStoppingTimerCalled, onBeforeStoppingTimerCalled)
				require.Equal(t, tt.onAfterStoppingTimerCalled, onAfterStoppingTimerCalled)
			},
		)
	}
}

func TestEndToEnd(t *testing.T) {
	var endToEndTests = []struct {
		name string

		runScenario func(t *testing.T, s *StateMachine)

		onBeforeStartingTimerCalled,
		onAfterStartingTimerCalled int

		internalInitialRemainingTime time.Duration

		onBeforeStoppingTimerCalled,
		onAfterStoppingTimerCalled,

		onStartAlarmCalled,

		onStopAlarmCalled int
	}{
		{
			name: "can set an alarm for 60s, wait for it to finish, and stop the alarm",

			runScenario: func(t *testing.T, s *StateMachine) {
				require.NoError(t, s.PlusTimer(t.Context()))
				require.NoError(t, s.PlusTimer(t.Context()))

				require.NoError(t, s.StartTimer(t.Context()))

				time.Sleep(RemainingTimerAdjustmentInterval * 3) // We run this *3, not *2, to make sure that `onCurrentRemainingTimeTick` no longer fires after stopping

				require.NoError(t, s.StopAlarming(t.Context()))
			},

			onBeforeStartingTimerCalled: 1,
			onAfterStartingTimerCalled:  1,

			internalInitialRemainingTime: RemainingTimerAdjustmentInterval * 2,

			onBeforeStoppingTimerCalled: 1,
			onAfterStoppingTimerCalled:  1,

			onStartAlarmCalled: 1,

			onStopAlarmCalled: 1,
		},
		{
			name: "can set an alarm for 120s, wait for it to finish, and stop the alarm",

			runScenario: func(t *testing.T, s *StateMachine) {
				require.NoError(t, s.PlusTimer(t.Context()))
				require.NoError(t, s.PlusTimer(t.Context()))
				require.NoError(t, s.PlusTimer(t.Context()))
				require.NoError(t, s.PlusTimer(t.Context()))

				require.NoError(t, s.StartTimer(t.Context()))

				time.Sleep(RemainingTimerAdjustmentInterval * 5) // We run this *5, not *4, to make sure that `onCurrentRemainingTimeTick` no longer fires after stopping

				require.NoError(t, s.StopAlarming(t.Context()))
			},

			onBeforeStartingTimerCalled: 1,
			onAfterStartingTimerCalled:  1,

			internalInitialRemainingTime: RemainingTimerAdjustmentInterval * 4,

			onBeforeStoppingTimerCalled: 1,
			onAfterStoppingTimerCalled:  1,

			onStartAlarmCalled: 1,

			onStopAlarmCalled: 1,
		},
		{
			name: "can set an alarm for 120s, wait for it to finish, and keep the alarm running",

			runScenario: func(t *testing.T, s *StateMachine) {
				require.NoError(t, s.PlusTimer(t.Context()))
				require.NoError(t, s.PlusTimer(t.Context()))
				require.NoError(t, s.PlusTimer(t.Context()))
				require.NoError(t, s.PlusTimer(t.Context()))

				require.NoError(t, s.StartTimer(t.Context()))

				time.Sleep(RemainingTimerAdjustmentInterval * 5) // We run this *5, not *4, to make sure that `onCurrentRemainingTimeTick` no longer fires after stopping
			},

			onBeforeStartingTimerCalled: 1,
			onAfterStartingTimerCalled:  1,

			internalInitialRemainingTime: RemainingTimerAdjustmentInterval * 4,

			onBeforeStoppingTimerCalled: 1,
			onAfterStoppingTimerCalled:  1,

			onStartAlarmCalled: 1,

			onStopAlarmCalled: 0,
		},
	}
	for _, tt := range endToEndTests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					var (
						onCurrentRemainingTimeTickCallArguments         = []time.Duration{}
						expectedOnCurrentRemainingTimeTickCallArguments = []time.Duration{}
					)

					for i := tt.internalInitialRemainingTime - tickerInterval; i >= 0; i -= tickerInterval {
						expectedOnCurrentRemainingTimeTickCallArguments = append(expectedOnCurrentRemainingTimeTickCallArguments, i)
					}

					var (
						onBeforeStartingTimerCalled = 0
						onAfterStartingTimerCalled  = 0

						internalInitialRemainingTime = time.Duration(0)

						onBeforeStoppingTimerCalled = 0
						onAfterStoppingTimerCalled  = 0

						onStartAlarmCalled = 0

						onStopAlarmCalled = 0
					)
					s := newTestingStateMachine(
						t,
						0,
						&Hooks{
							OnBeforeStartingTimer: func(ctx context.Context) error {
								onBeforeStartingTimerCalled++

								return nil
							},
							OnAfterStartingTimer: func(ctx context.Context) error {
								onAfterStartingTimerCalled++

								return nil
							},

							OnInitialRemainingTimeChange: func(ctx context.Context, initialRemainingTime time.Duration) error {
								internalInitialRemainingTime = initialRemainingTime

								return nil
							},
							OnCurrentRemainingTimeTick: func(ctx context.Context, currentRemainingTime time.Duration) error {
								onCurrentRemainingTimeTickCallArguments = append(onCurrentRemainingTimeTickCallArguments, currentRemainingTime)

								return nil
							},

							OnBeforeStoppingTimer: func(ctx context.Context) error {
								onBeforeStoppingTimerCalled++

								return nil
							},
							OnAfterStoppingTimer: func(ctx context.Context) error {
								onAfterStoppingTimerCalled++

								return nil
							},

							OnStartAlarm: func(ctx context.Context) error {
								onStartAlarmCalled++

								return nil
							},

							OnStopAlarm: func(ctx context.Context) error {
								onStopAlarmCalled++

								return nil
							},
						},
					)

					tt.runScenario(t, s)

					require.Equal(t, tt.onBeforeStartingTimerCalled, onBeforeStartingTimerCalled)
					require.Equal(t, tt.onAfterStartingTimerCalled, onAfterStartingTimerCalled)

					require.Equal(t, tt.internalInitialRemainingTime, internalInitialRemainingTime)

					require.Equal(t, expectedOnCurrentRemainingTimeTickCallArguments, onCurrentRemainingTimeTickCallArguments)

					require.Equal(t, tt.onBeforeStoppingTimerCalled, onBeforeStoppingTimerCalled)
					require.Equal(t, tt.onAfterStoppingTimerCalled, onAfterStoppingTimerCalled)

					require.Equal(t, tt.onStartAlarmCalled, onStartAlarmCalled)
					require.Equal(t, tt.onStopAlarmCalled, onStopAlarmCalled)
				})
			},
		)
	}
}

func TestGetInitialRemainingTimeFromCurrentRemainingTime(t *testing.T) {
	var getRemainingTimeTests = []struct {
		name                 string
		currentRemainingTime time.Duration
		intervalsToAdd       int
		newRemainingTime     time.Duration
	}{
		{
			name:                 "0s plus 1 interval results in 30s",
			currentRemainingTime: 0,
			intervalsToAdd:       1,
			newRemainingTime:     RemainingTimerAdjustmentInterval,
		},
		{
			name:                 "0s plus 2 intervals results in 60s",
			currentRemainingTime: 0,
			intervalsToAdd:       2,
			newRemainingTime:     RemainingTimerAdjustmentInterval * 2,
		},
		{
			name:                 "5s plus 2 intervals results in 60s",
			currentRemainingTime: time.Second * 5,
			intervalsToAdd:       2,
			newRemainingTime:     RemainingTimerAdjustmentInterval * 2,
		},
		{
			name:                 "14s plus 2 intervals results in 60s",
			currentRemainingTime: time.Second * 14,
			intervalsToAdd:       2,
			newRemainingTime:     RemainingTimerAdjustmentInterval * 2,
		},
		{
			name:                 "15s plus 2 intervals results in 90s",
			currentRemainingTime: time.Second * 15,
			intervalsToAdd:       2,
			newRemainingTime:     RemainingTimerAdjustmentInterval * 3,
		},
		{
			name:                 "30s plus 2 intervals results in 90s",
			currentRemainingTime: RemainingTimerAdjustmentInterval,
			intervalsToAdd:       2,
			newRemainingTime:     RemainingTimerAdjustmentInterval * 3,
		},
	}
	for _, tt := range getRemainingTimeTests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				require.Equal(
					t,
					tt.newRemainingTime,
					getInitialRemainingTimeFromCurrentRemainingTime(
						tt.currentRemainingTime,
						tt.intervalsToAdd,
					),
				)
			},
		)
	}
}
