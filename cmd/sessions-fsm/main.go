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

	onRemainingTimeTick func(ctx context.Context, currentRemainingTime time.Duration) error

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
		OnExitWith(triggerPlusTimer, s.increaseInitialRemainingTime)
	s.machine.
		Configure(stateStopped).
		PermitReentry(triggerMinusTimer, s.mustBeAboveMinInitialRemainingTime).
		OnExitWith(triggerMinusTimer, s.decreaseInitialRemainingTime)

	// From counting down state, we can also increment and decrement the initial remaining
	// time, same as for the stopped state
	s.machine.
		Configure(stateCountingDown).
		PermitReentry(triggerPlusTimer, s.mustBeBelowMaxInitialRemainingTime).
		OnExitWith(triggerPlusTimer, s.increaseInitialRemainingTime)
	s.machine.
		Configure(stateCountingDown).
		PermitReentry(triggerMinusTimer, s.mustBeAboveMinInitialRemainingTime).
		OnExitWith(triggerMinusTimer, s.decreaseInitialRemainingTime)

	// From stopped state, we can start dragging
	s.machine.Configure(stateStopped).Permit(triggerStartDragging, stateDragging)

	// When we stop dragging, before entering into counting down state,
	// we validate whether the new initial remaining time is valid, and if it
	// is, we set it
	s.machine.SetTriggerParameters(triggerStopDragging, reflect.TypeFor[time.Duration]())
	s.machine.
		Configure(stateDragging).
		Permit(triggerStopDragging, stateCountingDown, s.validInitialRemainingTime).
		OnExitWith(triggerStopDragging, s.setInitialRemainingTime)

	// From counting down state, we can start dragging as well
	s.machine.Configure(stateCountingDown).Permit(triggerStartDragging, stateDragging)

	// From counting down state, we can stop/reset and then restart the timer
	s.machine.Configure(stateCountingDown).Permit(triggerStopTimer, stateStopped)
	s.machine.Configure(stateStopped).Permit(triggerStartTimer, stateCountingDown)

	// From counting down state, we can go into alarming state when the timer has finished
	s.machine.Configure(stateCountingDown).Permit(triggerTimerFinished, stateAlarming)

	// From alarming state, we can return to stopped state when the alarm is stopped
	s.machine.Configure(stateAlarming).Permit(triggerStopAlarming, stateStopped)

	// When we enter the counting down state, we start the timer
	s.machine.Configure(stateCountingDown).OnEntry(s.onTimerStart)
	// When we enter the alarming state, we start the alarm
	s.machine.Configure(stateAlarming).OnEntry(s.onStartAlarm)
	// When we enter the stopped state, we stop the alarm or timer
	s.machine.
		Configure(stateStopped).
		OnEntryFrom(triggerStopTimer, s.onTimerStop).
		OnEntryFrom(triggerStopAlarming, s.onStopAlarm)

	return s
}

func (s *stateMachine) increaseInitialRemainingTime(ctx context.Context, args ...any) error {
	s.initialRemainingTime += remainingTimerAdjustmentInterval

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

func (s *stateMachine) onTimerStart(ctx context.Context, args ...any) error {
	s.log.InfoContext(ctx, "Calling onBeforeStartingTimer hook")
	if err := s.hooks.onBeforeStartingTimer(ctx); err != nil {
		return err
	}

	s.currentRemainingTime = s.initialRemainingTime
	s.ticker = time.NewTicker(tickerInterval)
	tickerCtx, cancelTickerCtx := context.WithCancel(s.ctx)
	s.cancelTickerCtx = cancelTickerCtx

	go func() {
		for {
			select {
			case <-tickerCtx.Done(): // tickerCtx derives from s.ctx so this catches both
				return

			case <-s.ticker.C:
				s.currentRemainingTime -= tickerInterval

				s.log.InfoContext(
					s.ctx, "Calling onRemainingTimeTick hook",
					"currentRemainingTime", s.currentRemainingTime,
				)
				if err := s.hooks.onRemainingTimeTick(ctx, s.currentRemainingTime); err != nil {
					s.log.ErrorContext(s.ctx, "Could not call onRemainingTimeTick hook", "err", err)
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

func (s *stateMachine) onTimerStop(ctx context.Context, args ...any) error {
	s.log.InfoContext(ctx, "Calling onBeforeStoppingTimer hook")
	if err := s.hooks.onBeforeStoppingTimer(ctx); err != nil {
		return err
	}

	s.ticker.Stop()
	s.cancelTickerCtx()

	s.log.InfoContext(ctx, "Calling onAfterStoppingTimer hook")
	if err := s.hooks.onAfterStoppingTimer(ctx); err != nil {
		return err
	}

	return nil
}

func (s *stateMachine) onStartAlarm(ctx context.Context, args ...any) error {
	s.log.InfoContext(ctx, "Calling onStartAlarm hook")
	if err := s.hooks.onStartAlarm(ctx); err != nil {
		return err
	}

	return nil
}

func (s *stateMachine) onStopAlarm(ctx context.Context, args ...any) error {
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

			onRemainingTimeTick: func(ctx context.Context, currentRemainingTime time.Duration) error { return nil },

			onBeforeStoppingTimer: func(ctx context.Context) error { return nil },
			onAfterStoppingTimer:  func(ctx context.Context) error { return nil },

			onStartAlarm: func(ctx context.Context) error { return nil },

			onStopAlarm: func(ctx context.Context) error { return nil },
		},
	)

	fmt.Println(s.machine.ToGraph())

	for range 50 {
		if err := s.PlusTimer(ctx); err != nil {
			panic(err)
		}
	}

	for range 30 {
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

	for range 50 {
		if err := s.PlusTimer(ctx); err != nil {
			panic(err)
		}
	}

	for range 30 {
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
