package main

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"time"

	"github.com/qmuntal/stateless"
)

type state string

const (
	stateStopped      state = "stopped"
	stateDragging     state = "dragging"
	stateCountingDown state = "countingDown"
	stateAlarming     state = "alarming"
)

type trigger string

const (
	triggerPlusTimer  trigger = "plusTimer"
	triggerMinusTimer trigger = "minusTimer"

	triggerStartDragging trigger = "startDragging"
	triggerStopDragging  trigger = "stopDragging"

	triggerStartTimer trigger = "startTimer"
	triggerStopTimer  trigger = "stopTimer"

	triggerTimerFinished trigger = "timerFinished"
	triggerStopAlarming  trigger = "stopAlarming"
)

const (
	initialRemainingTime             = time.Minute * 5
	minRemainingTime                 = time.Duration(0)
	maxRemainingTime                 = time.Hour
	remainingTimerAdjustmentInterval = time.Second * 30
	tickerInterval                   = time.Second
)

type hooks struct {
	onBeforeStartingTimer func(ctx context.Context) error
	onAfterStartingTimer  func(ctx context.Context) error

	onInitialRemainingTimeChange func(ctx context.Context, initialRemainingTime time.Duration) error
	onCurrentRemainingTimeTick   func(ctx context.Context, currentRemainingTime time.Duration) error

	onBeforeStoppingTimer func(ctx context.Context) error
	onAfterStoppingTimer  func(ctx context.Context) error

	onStartAlarm func(ctx context.Context) error

	onStopAlarm func(ctx context.Context) error
}

type stateMachine struct {
	initialRemainingTime,
	currentRemainingTime time.Duration
	ctx   context.Context
	log   *slog.Logger
	hooks *hooks

	machine         *stateless.StateMachine
	ticker          *time.Ticker
	tickerCtx       context.Context
	cancelTickerCtx context.CancelFunc
}

func newStateMachine(
	ctx context.Context,
	remainingTime time.Duration,
	log *slog.Logger,
	hooks *hooks,
) *stateMachine {
	s := &stateMachine{
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
	// time, same as for the stopped state
	s.machine.
		Configure(stateCountingDown).
		PermitReentry(triggerPlusTimer, s.mustBeBelowMaxInitialRemainingTime).
		OnEntryFrom(triggerPlusTimer, s.increaseInitialRemainingTime)
	s.machine.
		Configure(stateCountingDown).
		PermitReentry(triggerMinusTimer, s.mustBeAboveMinInitialRemainingTime).
		OnEntryFrom(triggerMinusTimer, s.decreaseInitialRemainingTime)

	// TODO: When incrementing/decrementing from the counting down state, we need to round the current remaining
	// time down to the nearest lower multiple of remainingTimerAdjustmentInterval, increment from there, set that as the
	// new initial time (& maybe call the onInitialRemainingTimeChange hook? Might not be necessary, we will update the UI with the onCurrentRemainingTimeTick),
	// stop the _internal_ timer (without calling the hooks/changing state), manually call onCurrentRemainingTimeTick to reflect the new position. We do not
	// need to manually re-start the internal timer, that should automatically happen when entering stateCountingDown already

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

func (s *stateMachine) increaseInitialRemainingTime(ctx context.Context, args ...any) error {
	s.initialRemainingTime += remainingTimerAdjustmentInterval

	s.log.InfoContext(
		s.ctx, "Calling onInitialRemainingTimeChange hook",
		"initialRemainingTime", s.initialRemainingTime,
	)
	if err := s.hooks.onInitialRemainingTimeChange(ctx, s.initialRemainingTime); err != nil {
		return err
	}

	return nil
}

func (s *stateMachine) mustBeBelowMaxInitialRemainingTime(ctx context.Context, args ...any) bool {
	newInitialRemainingTime := s.initialRemainingTime + remainingTimerAdjustmentInterval
	if newInitialRemainingTime > maxRemainingTime {
		return false
	}

	return true
}

func (s *stateMachine) decreaseInitialRemainingTime(ctx context.Context, args ...any) error {
	s.initialRemainingTime -= remainingTimerAdjustmentInterval

	s.log.InfoContext(
		s.ctx, "Calling onInitialRemainingTimeChange hook",
		"initialRemainingTime", s.initialRemainingTime,
	)
	if err := s.hooks.onInitialRemainingTimeChange(ctx, s.initialRemainingTime); err != nil {
		return err
	}

	return nil
}

func (s *stateMachine) mustBeAboveMinInitialRemainingTime(ctx context.Context, args ...any) bool {
	newInitialRemainingTime := s.initialRemainingTime - remainingTimerAdjustmentInterval
	if newInitialRemainingTime < minRemainingTime {
		return false
	}

	return true
}

func (s *stateMachine) validInitialRemainingTime(ctx context.Context, args ...any) bool {
	newInitialRemainingTime := args[0].(time.Duration)
	if newInitialRemainingTime < minRemainingTime ||
		newInitialRemainingTime > maxRemainingTime ||
		newInitialRemainingTime%remainingTimerAdjustmentInterval != 0 {
		return false
	}

	return true
}

func (s *stateMachine) setInitialRemainingTime(ctx context.Context, args ...any) error {
	newInitialRemainingTime := args[0].(time.Duration)

	s.initialRemainingTime = newInitialRemainingTime

	s.log.InfoContext(
		s.ctx, "Calling onInitialRemainingTimeChange hook",
		"initialRemainingTime", s.initialRemainingTime,
	)
	if err := s.hooks.onInitialRemainingTimeChange(ctx, s.initialRemainingTime); err != nil {
		return err
	}

	return nil
}

func (s *stateMachine) PlusTimer(ctx context.Context) error {
	return s.machine.FireCtx(ctx, triggerPlusTimer)
}

func (s *stateMachine) MinusTimer(ctx context.Context) error {
	return s.machine.FireCtx(ctx, triggerMinusTimer)
}

func (s *stateMachine) StartDragging(ctx context.Context) error {
	return s.machine.FireCtx(ctx, triggerStartDragging)
}

func (s *stateMachine) StopDragging(ctx context.Context, remainingTime time.Duration) error {
	return s.machine.FireCtx(ctx, triggerStopDragging, remainingTime)
}

func (s *stateMachine) StopTimer(ctx context.Context) error {
	return s.machine.FireCtx(ctx, triggerStopTimer)
}

func (s *stateMachine) StartTimer(ctx context.Context) error {
	return s.machine.FireCtx(ctx, triggerStartTimer)
}

func (s *stateMachine) timerFinished(ctx context.Context) error {
	return s.machine.FireCtx(ctx, triggerTimerFinished)
}

func (s *stateMachine) StopAlarming(ctx context.Context) error {
	return s.machine.FireCtx(ctx, triggerStopAlarming)
}

func (s *stateMachine) startTimer(ctx context.Context, args ...any) error {
	s.log.InfoContext(ctx, "Calling onBeforeStartingTimer hook")
	if err := s.hooks.onBeforeStartingTimer(ctx); err != nil {
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
				if err := s.hooks.onCurrentRemainingTimeTick(ctx, s.currentRemainingTime); err != nil {
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
	if err := s.hooks.onAfterStartingTimer(ctx); err != nil {
		return err
	}

	return nil
}

func (s *stateMachine) stopTimer(ctx context.Context, args ...any) error {
	s.log.InfoContext(ctx, "Calling onBeforeStoppingTimer hook")
	if err := s.hooks.onBeforeStoppingTimer(ctx); err != nil {
		return err
	}

	if err := s.stopTimerWithoutHooks(ctx, args...); err != nil {
		return err
	}

	s.log.InfoContext(ctx, "Calling onAfterStoppingTimer hook")
	if err := s.hooks.onAfterStoppingTimer(ctx); err != nil {
		return err
	}

	return nil
}

func (s *stateMachine) stopTimerWithoutHooks(ctx context.Context, args ...any) error {
	s.ticker.Stop()
	s.cancelTickerCtx()

	return nil
}

func (s *stateMachine) startAlarm(ctx context.Context, args ...any) error {
	s.log.InfoContext(ctx, "Calling onStartAlarm hook")
	if err := s.hooks.onStartAlarm(ctx); err != nil {
		return err
	}

	return nil
}

func (s *stateMachine) stopAlarm(ctx context.Context, args ...any) error {
	s.log.InfoContext(ctx, "Calling onStopAlarm hook")
	if err := s.hooks.onStopAlarm(ctx); err != nil {
		return err
	}

	return nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := newStateMachine(
		ctx,
		initialRemainingTime,
		slog.Default(),
		&hooks{
			onBeforeStartingTimer: func(ctx context.Context) error { return nil },
			onAfterStartingTimer:  func(ctx context.Context) error { return nil },

			onInitialRemainingTimeChange: func(ctx context.Context, initialRemainingTime time.Duration) error { return nil },
			onCurrentRemainingTimeTick:   func(ctx context.Context, currentRemainingTime time.Duration) error { return nil },

			onBeforeStoppingTimer: func(ctx context.Context) error { return nil },
			onAfterStoppingTimer:  func(ctx context.Context) error { return nil },

			onStartAlarm: func(ctx context.Context) error { return nil },

			onStopAlarm: func(ctx context.Context) error { return nil },
		},
	)

	fmt.Println(s.machine.ToGraph())

	for range 5 {
		if err := s.PlusTimer(ctx); err != nil {
			panic(err)
		}
	}

	for range 3 {
		if err := s.MinusTimer(ctx); err != nil {
			panic(err)
		}
	}

	if err := s.StartDragging(ctx); err != nil {
		panic(err)
	}

	if err := s.StopDragging(ctx, initialRemainingTime+remainingTimerAdjustmentInterval*3); err != nil {
		panic(err)
	}

	if err := s.StartDragging(ctx); err != nil {
		panic(err)
	}

	if err := s.StopDragging(ctx, initialRemainingTime+remainingTimerAdjustmentInterval*2); err != nil {
		panic(err)
	}

	for range 5 {
		if err := s.PlusTimer(ctx); err != nil {
			panic(err)
		}
	}

	for range 3 {
		if err := s.MinusTimer(ctx); err != nil {
			panic(err)
		}
	}

	if err := s.StopTimer(ctx); err != nil {
		panic(err)
	}

	if err := s.StartTimer(ctx); err != nil {
		panic(err)
	}

	if err := s.timerFinished(ctx); err != nil {
		panic(err)
	}

	if err := s.StopAlarming(ctx); err != nil {
		panic(err)
	}
}
