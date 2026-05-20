package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/pojntfx/sessions/pkg/state"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := state.NewStateMachine(
		ctx,
		state.DefaultInitialRemainingTime,
		slog.Default(),
		&state.Hooks{
			OnBeforeStartingTimer: func(ctx context.Context) error { return nil },
			OnAfterStartingTimer:  func(ctx context.Context) error { return nil },

			OnInitialRemainingTimeChange: func(ctx context.Context, initialRemainingTime time.Duration) error { return nil },
			OnCurrentRemainingTimeTick:   func(ctx context.Context, currentRemainingTime time.Duration) error { return nil },

			OnBeforeStoppingTimer: func(ctx context.Context) error { return nil },
			OnAfterStoppingTimer:  func(ctx context.Context) error { return nil },

			OnStartAlarm: func(ctx context.Context) error { return nil },

			OnStopAlarm: func(ctx context.Context) error { return nil },

			OnPermittedTriggersChange: func(ctx context.Context, permittedTriggers []state.Trigger) error { return nil },
		},
	)

	fmt.Println(s.ToGraph())

	fmt.Println(s)

	for range 5 {
		if err := s.PlusTimer(ctx); err != nil {
			panic(err)
		}
	}

	fmt.Println(s)

	for range 3 {
		if err := s.MinusTimer(ctx); err != nil {
			panic(err)
		}
	}

	fmt.Println(s)

	if err := s.StartDragging(ctx); err != nil {
		panic(err)
	}

	fmt.Println(s)

	if err := s.StopDragging(ctx, state.DefaultInitialRemainingTime+state.RemainingTimerAdjustmentInterval*3); err != nil {
		panic(err)
	}

	fmt.Println(s)

	if err := s.StartDragging(ctx); err != nil {
		panic(err)
	}

	fmt.Println(s)

	if err := s.StopDragging(ctx, state.DefaultInitialRemainingTime+state.RemainingTimerAdjustmentInterval*2); err != nil {
		panic(err)
	}

	fmt.Println(s)

	for range 5 {
		if err := s.PlusTimer(ctx); err != nil {
			panic(err)
		}
	}

	fmt.Println(s)

	for range 3 {
		if err := s.MinusTimer(ctx); err != nil {
			panic(err)
		}
	}

	fmt.Println(s)

	if err := s.StopTimer(ctx); err != nil {
		panic(err)
	}

	fmt.Println(s)

	if err := s.StartTimer(ctx); err != nil {
		panic(err)
	}

	fmt.Println(s)

	// These are disabled since there is no reason to trigger the transition from counting down to finished programmatically

	// if err := s.timerFinished(ctx); err != nil {
	// 	panic(err)
	// }

	// fmt.Println(s)

	// if err := s.StopAlarming(ctx); err != nil {
	// 	panic(err)
	// }

	// fmt.Println(s)
}
