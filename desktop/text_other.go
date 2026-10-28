//go:build !windows

package main

import (
	"context"
	"fmt"
)

func executeTextPuzzle(p Puzzle) error {
	return fmt.Errorf("текстові дії потребують Windows")
}

func executeTextPuzzleContext(ctx context.Context, p Puzzle) error { return executeTextPuzzle(p) }
