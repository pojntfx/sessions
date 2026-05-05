package main

import (
	"context"
	"fmt"
	"time"

	"github.com/qmuntal/stateless"
)

type state string

const (
	statePaused       state = "paused"
	stateDragging     state = "dragging"
	stateCountingDown state = "countingDown"
	stateAlarming     state = "alarming"
)

type trigger string

const (
	triggerPlusTimer  trigger = "plusTimer"
	triggerMinusTimer trigger = "minusTimer"
	triggerStartTimer trigger = "startTimer"
	triggerStopTimer  trigger = "stopTimer"
)

const (
	initialRemainingTime             = time.Minute * 5
	minRemainingTime                 = time.Duration(0)
	maxRemainingTime                 = time.Hour
	remainingTimerAdjustmentInterval = time.Second * 30
)

type stateMachine struct {
	currentRemainingTime time.Duration
	machine              *stateless.StateMachine
}

func newStateMachine(remainingTime time.Duration) *stateMachine {
	s := &stateMachine{
		currentRemainingTime: remainingTime,
		machine:              stateless.NewStateMachine(statePaused),
	}

	s.machine.
		Configure(statePaused).
		PermitReentry(triggerPlusTimer, s.mustBeBelowMaxRemainingTime).
		OnEntryFrom(triggerPlusTimer, s.increaseRemainingTime)

	return s
}

func (s *stateMachine) increaseRemainingTime(ctx context.Context, args ...any) error {
	s.currentRemainingTime += remainingTimerAdjustmentInterval

	return nil
}

func (s *stateMachine) mustBeBelowMaxRemainingTime(ctx context.Context, args ...any) bool {
	newRemainingTime := s.currentRemainingTime + remainingTimerAdjustmentInterval
	if newRemainingTime > maxRemainingTime {
		return false
	}

	return true
}

func (s *stateMachine) PlusTimer(ctx context.Context) error {
	return s.machine.FireCtx(ctx, triggerPlusTimer)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := newStateMachine(initialRemainingTime)

	fmt.Println(s.machine.ToGraph())

	for range 50 {
		if err := s.PlusTimer(ctx); err != nil {
			panic(err)
		}
	}
}
