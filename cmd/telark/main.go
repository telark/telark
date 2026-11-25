package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/telark/telark/cmd/telark/internal/constants"
	"github.com/telark/telark/cmd/telark/internal/root"

	// Registers the kubeconfig auth providers, as kubectl, helm and flux do.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func main() {
	// Ctrl-C cancels the context, so Helm marks an upgrade failed and a stopped exporter starts again; a second one
	// ends the CLI at once.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	context.AfterFunc(ctx, stop)
	os.Exit(root.Run(ctx, os.Stdin, os.Stdout, os.Stderr, os.Args[constants.DefaultIncrementValue:]))
}
