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

	machine *stateless.StateMachine
	ticker  *time.Ticker
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

	s.machine.
		Configure(stateStopped).
		PermitReentry(triggerPlusTimer, s.mustBeBelowMaxRemainingTime).
		OnEntryFrom(triggerPlusTimer, s.increaseRemainingTime)
	s.machine.
		Configure(stateStopped).
		PermitReentry(triggerMinusTimer, s.mustBeAboveMinRemainingTime).
		OnEntryFrom(triggerMinusTimer, s.decreaseRemainingTime)

	s.machine.Configure(stateStopped).Permit(triggerStartDragging, stateDragging)

	s.machine.SetTriggerParameters(triggerStopDragging, reflect.TypeFor[time.Duration]())
	s.machine.
		Configure(stateDragging).
		Permit(triggerStopDragging, stateCountingDown, s.validRemainingTime)

	s.machine.Configure(stateCountingDown).Permit(triggerStartDragging, stateDragging)

	s.machine.
		Configure(stateCountingDown).
		PermitReentry(triggerPlusTimer, s.mustBeBelowMaxRemainingTime).
		OnEntryFrom(triggerPlusTimer, s.increaseRemainingTime)
	s.machine.
		Configure(stateCountingDown).
		PermitReentry(triggerMinusTimer, s.mustBeAboveMinRemainingTime).
		OnEntryFrom(triggerMinusTimer, s.decreaseRemainingTime)

	s.machine.Configure(stateCountingDown).Permit(triggerStopTimer, stateStopped)
	s.machine.Configure(stateStopped).Permit(triggerStartTimer, stateCountingDown)

	s.machine.Configure(stateCountingDown).Permit(triggerTimerFinished, stateAlarming)

	s.machine.Configure(stateAlarming).Permit(triggerStopAlarming, stateStopped)

	s.machine.Configure(stateCountingDown).OnEntry(s.onTimerStart)
	s.machine.Configure(stateAlarming).OnEntry(s.onStartAlarm)
	s.machine.
		Configure(stateStopped).
		OnEntryFrom(triggerStopTimer, s.onTimerStop).
		OnEntryFrom(triggerStopAlarming, s.onStopAlarm)

	return s
}

func (s *stateMachine) increaseRemainingTime(ctx context.Context, args ...any) error {
	s.initialRemainingTime += remainingTimerAdjustmentInterval

	return nil
}

func (s *stateMachine) mustBeBelowMaxRemainingTime(ctx context.Context, args ...any) bool {
	newRemainingTime := s.initialRemainingTime + remainingTimerAdjustmentInterval
	if newRemainingTime > maxRemainingTime {
		return false
	}

	return true
}

func (s *stateMachine) decreaseRemainingTime(ctx context.Context, args ...any) error {
	s.initialRemainingTime -= remainingTimerAdjustmentInterval

	return nil
}

func (s *stateMachine) mustBeAboveMinRemainingTime(ctx context.Context, args ...any) bool {
	newRemainingTime := s.initialRemainingTime - remainingTimerAdjustmentInterval
	if newRemainingTime < minRemainingTime {
		return false
	}

	return true
}

func (s *stateMachine) validRemainingTime(ctx context.Context, args ...any) bool {
	newRemainingTime := args[0].(time.Duration)
	if newRemainingTime < minRemainingTime ||
		newRemainingTime > maxRemainingTime ||
		newRemainingTime%remainingTimerAdjustmentInterval != 0 {
		return false
	}

	return true
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

	go func() {
		for {
			select {
			case <-s.ctx.Done():
				return

			// TODO: Don't leak this goroutine by closing a goroutine or cancelling a context when the ticker has finished (see https://gobyexample.com/tickers)

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
