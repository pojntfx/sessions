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
	DefaultInitialRemainingTime      = time.Minute * 5
	RemainingTimerAdjustmentInterval = time.Second * 30

	minRemainingTime = time.Duration(0)
	maxRemainingTime = time.Hour
	tickerInterval   = time.Second
)
