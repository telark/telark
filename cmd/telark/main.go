package main

import (
	"context"
	"os"

	"github.com/telark/telark/cmd/telark/internal/constants"
	"github.com/telark/telark/cmd/telark/internal/root"

	// Registers the kubeconfig auth providers, as kubectl, helm and flux do.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func main() {
	os.Exit(root.Run(context.Background(), os.Stdout, os.Stderr, os.Args[constants.DefaultIncrementValue:]))
}
