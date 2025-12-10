//go:build wireinject
// +build wireinject

package main

import (
	"context"

	"github.com/google/wire"
)

// BuildApp wires the server components using Google Wire.
func BuildApp(ctx context.Context) (*App, error) {
	wire.Build(
		provideConfig,
		provideLogger,
		provideHub,
		provideStorage,
		provideService,
		provideHandler,
		provideServer,
		wire.Struct(new(App), "*"),
	)
	return nil, nil
}
