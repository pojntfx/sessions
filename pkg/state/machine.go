package state

import (
	"context"
	"log/slog"
	"math"
	"reflect"
	"time"

	"github.com/qmuntal/stateless"
)

type Hooks struct {
	OnBeforeStartingTimer func(ctx context.Context) error
	OnAfterStartingTimer  func(ctx context.Context) error

	OnInitialRemainingTimeChange func(ctx context.Context, initialRemainingTime time.Duration) error
	OnCurrentRemainingTimeTick   func(ctx context.Context, currentRemainingTime time.Duration) error

	OnBeforeStoppingTimer func(ctx context.Context) error
	OnAfterStoppingTimer  func(ctx context.Context) error

	OnStartAlarm func(ctx context.Context) error

	OnStopAlarm func(ctx context.Context) error
}

type StateMachine struct {
	initialRemainingTime,
	currentRemainingTime time.Duration
	ctx   context.Context
	log   *slog.Logger
	hooks *Hooks

	machine         *stateless.StateMachine
	ticker          *time.Ticker
	tickerCtx       context.Context
	cancelTickerCtx context.CancelFunc
}

func NewStateMachine(
	ctx context.Context,
	remainingTime time.Duration,
	log *slog.Logger,
	hooks *Hooks,
) *StateMachine {
	s := &StateMachine{
		ctx:                  ctx,
		initialRemainingTime: remainingTime,
		log:                  log,
		hooks:                hooks,

		machine: stateless.NewStateMachine(stateStopped),
	}

	// From stopped state, we can increment and decrement the initial remaining time
	// as long as it is higher than the minimum initial remaining time and higher
	// than the maximum remaining time
	s.machine.
		Configure(stateStopped).
		PermitReentry(triggerPlusTimer, s.mustBeBelowMaxInitialRemainingTime).
		OnEntryFrom(triggerPlusTimer, s.increaseInitialRemainingTime)
	s.machine.
		Configure(stateStopped).
		PermitReentry(triggerMinusTimer, s.mustBeAboveMinInitialRemainingTime).
		OnEntryFrom(triggerMinusTimer, s.decreaseInitialRemainingTime)

	// From counting down state, we can also increment and decrement the initial remaining
	// time, same as for the stopped state. When we increment while in counting down state, we
	// stop the timer first; it will be restarted automatically again once we transition back into
	// the counting down state with the new value
	s.machine.
		Configure(stateCountingDown).
		PermitReentry(triggerPlusTimer, s.mustBeBelowMaxCurrentRemainingTime).
		OnExitWith(triggerPlusTimer, s.stopTimerWithoutHooks).
		OnEntryFrom(triggerPlusTimer, s.increaseInitialRemainingTimeFromCurrentRemainingTime)
	s.machine.
		Configure(stateCountingDown).
		PermitReentry(triggerMinusTimer, s.mustBeAboveMinCurrentRemainingTime).
		OnExitWith(triggerMinusTimer, s.stopTimerWithoutHooks).
		OnEntryFrom(triggerMinusTimer, s.decreaseInitialRemainingTimeFromCurrentRemainingTime)

	// From stopped state, we can start dragging
	s.machine.Configure(stateStopped).Permit(triggerStartDragging, stateDragging)

	// When we stop dragging, before entering into counting down state,
	// we validate whether the new initial remaining time is valid
	s.machine.SetTriggerParameters(triggerStopDragging, reflect.TypeFor[time.Duration]())
	s.machine.
		Configure(stateDragging).
		Permit(triggerStopDragging, stateCountingDown, s.validInitialRemainingTime)

	// When we enter the counting down state by stopping to drag, we set the initial remaining time
	s.machine.Configure(stateCountingDown).OnEntryFrom(triggerStopDragging, s.setInitialRemainingTime)

	// From counting down state, we can start dragging as well. When we start dragging while in counting down state,
	// we stop the timer
	s.machine.
		Configure(stateCountingDown).
		Permit(triggerStartDragging, stateDragging).
		OnExitWith(triggerStartDragging, s.stopTimerWithoutHooks)

	// From counting down state, we can stop/reset and then restart the timer
	s.machine.Configure(stateCountingDown).Permit(triggerStopTimer, stateStopped)
	s.machine.Configure(stateStopped).Permit(triggerStartTimer, stateCountingDown)

	// From counting down state, we can go into alarming state when the timer has finished
	s.machine.Configure(stateCountingDown).Permit(triggerTimerFinished, stateAlarming)

	// From alarming state, we can return to stopped state when the alarm is stopped
	s.machine.Configure(stateAlarming).Permit(triggerStopAlarming, stateStopped)

	// When we enter the counting down state, we start the timer
	s.machine.Configure(stateCountingDown).OnEntry(s.startTimer)
	// When we enter the alarming state, we stop the timer and start the alarm
	s.machine.Configure(stateAlarming).
		OnEntry(s.stopTimer).
		OnEntry(s.startAlarm)
	// When we enter the stopped state, we stop the alarm or timer
	s.machine.
		Configure(stateStopped).
		// This won't fire when entering from alarming state since the trigger there is
		// triggerStopAlarming, not triggerStopTimer
		OnEntryFrom(triggerStopTimer, s.stopTimer).
		OnEntryFrom(triggerStopAlarming, s.stopAlarm)

	return s
}

func (s *StateMachine) ToGraph() string {
	return s.machine.ToGraph()
}

func (s *StateMachine) String() string {
	return s.machine.String()
}

func (s *StateMachine) increaseInitialRemainingTime(ctx context.Context, args ...any) error {
	s.initialRemainingTime += RemainingTimerAdjustmentInterval

	s.log.InfoContext(
		s.ctx, "Calling onInitialRemainingTimeChange hook",
		"initialRemainingTime", s.initialRemainingTime,
	)
	if err := s.hooks.OnInitialRemainingTimeChange(ctx, s.initialRemainingTime); err != nil {
		return err
	}

	return nil
}

func (s *StateMachine) mustBeBelowMaxInitialRemainingTime(ctx context.Context, args ...any) bool {
	newInitialRemainingTime := s.initialRemainingTime + RemainingTimerAdjustmentInterval
	if newInitialRemainingTime > maxRemainingTime {
		return false
	}

	return true
}

func (s *StateMachine) decreaseInitialRemainingTime(ctx context.Context, args ...any) error {
	s.initialRemainingTime -= RemainingTimerAdjustmentInterval

	s.log.InfoContext(
		s.ctx, "Calling onInitialRemainingTimeChange hook",
		"initialRemainingTime", s.initialRemainingTime,
	)
	if err := s.hooks.OnInitialRemainingTimeChange(ctx, s.initialRemainingTime); err != nil {
		return err
	}

	return nil
}

func (s *StateMachine) mustBeAboveMinInitialRemainingTime(ctx context.Context, args ...any) bool {
	newInitialRemainingTime := s.initialRemainingTime - RemainingTimerAdjustmentInterval
	if newInitialRemainingTime < minRemainingTime {
		return false
	}

	return true
}

func (s *StateMachine) validInitialRemainingTime(ctx context.Context, args ...any) bool {
	newInitialRemainingTime := args[0].(time.Duration)
	if newInitialRemainingTime < minRemainingTime ||
		newInitialRemainingTime > maxRemainingTime ||
		newInitialRemainingTime%RemainingTimerAdjustmentInterval != 0 {
		return false
	}

	return true
}

func (s *StateMachine) setInitialRemainingTime(ctx context.Context, args ...any) error {
	newInitialRemainingTime := args[0].(time.Duration)

	s.initialRemainingTime = newInitialRemainingTime

	s.log.InfoContext(
		s.ctx, "Calling onInitialRemainingTimeChange hook",
		"initialRemainingTime", s.initialRemainingTime,
	)
	if err := s.hooks.OnInitialRemainingTimeChange(ctx, s.initialRemainingTime); err != nil {
		return err
	}

	return nil
}

func getInitialRemainingTimeFromCurrentRemainingTime(currentRemainingTime time.Duration, intervalsToAdd int) time.Duration {
	intervals := time.Duration(
		math.Round(
			float64(currentRemainingTime)/float64(RemainingTimerAdjustmentInterval),
		) + float64(intervalsToAdd),
	)

	return intervals * RemainingTimerAdjustmentInterval
}

func (s *StateMachine) increaseInitialRemainingTimeFromCurrentRemainingTime(ctx context.Context, args ...any) error {
	s.initialRemainingTime = getInitialRemainingTimeFromCurrentRemainingTime(s.currentRemainingTime, 1)

	s.log.InfoContext(
		s.ctx, "Calling onInitialRemainingTimeChange hook",
		"initialRemainingTime", s.initialRemainingTime,
	)
	if err := s.hooks.OnInitialRemainingTimeChange(ctx, s.initialRemainingTime); err != nil {
		return err
	}

	return nil
}

func (s *StateMachine) mustBeBelowMaxCurrentRemainingTime(ctx context.Context, args ...any) bool {
	newInitialRemainingTime := getInitialRemainingTimeFromCurrentRemainingTime(s.currentRemainingTime, 1)
	if newInitialRemainingTime > maxRemainingTime {
		return false
	}

	return true
}

func (s *StateMachine) decreaseInitialRemainingTimeFromCurrentRemainingTime(ctx context.Context, args ...any) error {
	s.initialRemainingTime = getInitialRemainingTimeFromCurrentRemainingTime(s.currentRemainingTime, -1)

	s.log.InfoContext(
		s.ctx, "Calling onInitialRemainingTimeChange hook",
		"initialRemainingTime", s.initialRemainingTime,
	)
	if err := s.hooks.OnInitialRemainingTimeChange(ctx, s.initialRemainingTime); err != nil {
		return err
	}

	return nil
}

func (s *StateMachine) mustBeAboveMinCurrentRemainingTime(ctx context.Context, args ...any) bool {
	newInitialRemainingTime := getInitialRemainingTimeFromCurrentRemainingTime(s.currentRemainingTime, -1)
	if newInitialRemainingTime < minRemainingTime {
		return false
	}

	return true
}

func (s *StateMachine) PlusTimer(ctx context.Context) error {
	return s.machine.FireCtx(ctx, triggerPlusTimer)
}

func (s *StateMachine) MinusTimer(ctx context.Context) error {
	return s.machine.FireCtx(ctx, triggerMinusTimer)
}

func (s *StateMachine) StartDragging(ctx context.Context) error {
	return s.machine.FireCtx(ctx, triggerStartDragging)
}

func (s *StateMachine) StopDragging(ctx context.Context, remainingTime time.Duration) error {
	return s.machine.FireCtx(ctx, triggerStopDragging, remainingTime)
}

// CanStopDragging exists because onPermittedTriggersChange can't correctly report whether
// you can stop dragging without knowing what the new remainingTime would be
func (s *StateMachine) CanStopDragging(ctx context.Context, remainingTime time.Duration) (bool, error) {
	return s.machine.CanFireCtx(ctx, triggerStopDragging, remainingTime)
}

func (s *StateMachine) StopTimer(ctx context.Context) error {
	return s.machine.FireCtx(ctx, triggerStopTimer)
}

func (s *StateMachine) StartTimer(ctx context.Context) error {
	return s.machine.FireCtx(ctx, triggerStartTimer)
}

func (s *StateMachine) timerFinished(ctx context.Context) error {
	return s.machine.FireCtx(ctx, triggerTimerFinished)
}

func (s *StateMachine) StopAlarming(ctx context.Context) error {
	return s.machine.FireCtx(ctx, triggerStopAlarming)
}

func (s *StateMachine) startTimer(ctx context.Context, args ...any) error {
	s.log.InfoContext(ctx, "Calling onBeforeStartingTimer hook")
	if err := s.hooks.OnBeforeStartingTimer(ctx); err != nil {
		return err
	}

	s.currentRemainingTime = s.initialRemainingTime
	s.ticker = time.NewTicker(tickerInterval)
	s.tickerCtx, s.cancelTickerCtx = context.WithCancel(s.ctx)

	go func() {
		for {
			select {
			case <-s.tickerCtx.Done(): // tickerCtx derives from s.ctx so this catches both
				return

			case <-s.ticker.C:
				s.currentRemainingTime -= tickerInterval

				s.log.InfoContext(
					s.ctx, "Calling onCurrentRemainingTimeTick hook",
					"currentRemainingTime", s.currentRemainingTime,
				)
				if err := s.hooks.OnCurrentRemainingTimeTick(ctx, s.currentRemainingTime); err != nil {
					s.log.ErrorContext(s.ctx, "Could not call onCurrentRemainingTimeTick hook", "err", err)
				}

				if s.currentRemainingTime == 0 {
					if err := s.timerFinished(s.ctx); err != nil {
						s.log.ErrorContext(s.ctx, "Could not call handler to finish timer", "err", err)
					}
				}
			}
		}
	}()

	s.log.InfoContext(ctx, "Calling onAfterStartingTimer hook")
	if err := s.hooks.OnAfterStartingTimer(ctx); err != nil {
		return err
	}

	return nil
}

func (s *StateMachine) stopTimer(ctx context.Context, args ...any) error {
	s.log.InfoContext(ctx, "Calling onBeforeStoppingTimer hook")
	if err := s.hooks.OnBeforeStoppingTimer(ctx); err != nil {
		return err
	}

	if err := s.stopTimerWithoutHooks(ctx, args...); err != nil {
		return err
	}

	s.log.InfoContext(ctx, "Calling onAfterStoppingTimer hook")
	if err := s.hooks.OnAfterStoppingTimer(ctx); err != nil {
		return err
	}

	return nil
}

func (s *StateMachine) stopTimerWithoutHooks(ctx context.Context, args ...any) error {
	s.ticker.Stop()
	s.cancelTickerCtx()

	return nil
}

func (s *StateMachine) startAlarm(ctx context.Context, args ...any) error {
	s.log.InfoContext(ctx, "Calling onStartAlarm hook")
	if err := s.hooks.OnStartAlarm(ctx); err != nil {
		return err
	}

	return nil
}

func (s *StateMachine) stopAlarm(ctx context.Context, args ...any) error {
	s.log.InfoContext(ctx, "Calling onStopAlarm hook")
	if err := s.hooks.OnStopAlarm(ctx); err != nil {
		return err
	}

	return nil
}
