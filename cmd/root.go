/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"

	"nathanbeddoewebdev/vpsm/cmd/commands/auth"
	cfgcmd "nathanbeddoewebdev/vpsm/cmd/commands/config"
	"nathanbeddoewebdev/vpsm/cmd/commands/server"
	"nathanbeddoewebdev/vpsm/internal/providers"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
func rootCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "vpsm",
		Short: "A brief description of your application",
		Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
		// Uncomment the following line if your bare application
		// has an action associated with it:
		// Run: func(cmd *cobra.Command, args []string) {
		// },
	}

	cmd.AddCommand(auth.NewCommand())
	cmd.AddCommand(cfgcmd.NewCommand())
	cmd.AddCommand(server.NewCommand())

	return cmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	providers.RegisterHetzner()

	var root = rootCmd()
	err := root.Execute()
	if err != nil {
		os.Exit(1)
	}
}
