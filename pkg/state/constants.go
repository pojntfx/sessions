package state

import (
	"time"
)

type state string

const (
	stateStopped      state = "stopped"
	stateDragging     state = "dragging"
	stateCountingDown state = "countingDown"
	stateAlarming     state = "alarming"
)

type Trigger string

const (
	TriggerPlusTimer  Trigger = "plusTimer"
	TriggerMinusTimer Trigger = "minusTimer"

	TriggerStartDragging Trigger = "startDragging"
	// Checking whether this trigger can be used depends on what the new `remainingTime`
	// would be, use `CanStopDragging` instead
	triggerStopDragging Trigger = "stopDragging"

	TriggerStartTimer Trigger = "startTimer"
	TriggerStopTimer  Trigger = "stopTimer"

	// This one is only called from within the state machine
	triggerTimerFinished Trigger = "timerFinished"
	TriggerStopAlarming  Trigger = "stopAlarming"
)

const (
	DefaultInitialRemainingTime      = time.Minute * 5
	RemainingTimerAdjustmentInterval = time.Second * 30

	MinInitialRemainingTime = RemainingTimerAdjustmentInterval
	MaxInitialRemainingTime = time.Hour

	tickerInterval = time.Second
)
