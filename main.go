package main

import (
	"fmt"
	"os"

	"github.com/castyapp/cli/cmds"
	"github.com/spf13/cobra"
)

func main() {

	rootCmd := &cobra.Command{
		Use: "casty",
		Long: `
    ______           __       
   / ________ ______/ /___  __
  / /   / __  / ___/ __/ / / /
 / /___/ /_/ (__  / /_/ /_/ / 
 \____/\____/____/\__/\__  /  
                     /____/`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	rootCmd.SetArgs(os.Args[1:])
	rootCmd.AddCommand(cmds.NewServerCommand())
	rootCmd.AddCommand(cmds.NewJoinVoiceChannelCommand())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

}
