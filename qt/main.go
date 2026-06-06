package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/mappu/miqt/qt6"
	"github.com/mappu/miqt/qt6/qml"
	"github.com/pojntfx/sessions/pkg/state"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := slog.Default()

	qt6.NewQApplication(os.Args)

	engine := qml.NewQQmlApplicationEngine()

	url := qt6.QUrl_FromLocalFile(filepath.Join("qt", "main.qml"))

	engine.Load(url)

	s := state.NewStateMachine(
		ctx,
		time.Minute*5,
		log,
		&state.Hooks{
			OnStartTimer:                 func(ctx context.Context) error { return nil },
			OnStopTimer:                  func(ctx context.Context) error { return nil },
			OnInitialRemainingTimeChange: func(ctx context.Context, initialRemainingTime time.Duration) error { return nil },
			OnCurrentRemainingTimeTick:   func(ctx context.Context, currentRemainingTime time.Duration) error { return nil },
			OnStartAlarm:                 func(ctx context.Context) error { return nil },
			OnStopAlarm:                  func(ctx context.Context) error { return nil },
			OnPermittedTriggersChange:    func(ctx context.Context, permittedTriggers []state.Trigger) error { return nil },
		},
	)
	s.FlushPermittedTriggers(ctx)

	qt6.QApplication_Exec()
}
