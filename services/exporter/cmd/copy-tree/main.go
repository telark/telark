package main

import (
	"os"

	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/storagecopy"
)

func main() {
	os.Exit(storagecopy.Run(os.Args[constants.DefaultIncrementValue:]))
}
