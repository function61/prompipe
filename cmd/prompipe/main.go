package main

import (
	"fmt"
	"github.com/function61/gokit/dynversion"
	"github.com/spf13/cobra"
	"os"
)

func main() {
	rootCmd := &cobra.Command{
		Use:     os.Args[0],
		Short:   "Prometheus pipe",
		Version: dynversion.Version,
	}

	rootCmd.AddCommand(receiverEntry())
	rootCmd.AddCommand(senderEntry())

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
