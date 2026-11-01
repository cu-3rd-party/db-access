// Package cmd holds the CLI entrypoints.
package cmd

import "github.com/urfave/cli/v3"

// NewMainApp builds the root CLI command.
func NewMainApp() *cli.Command {
	return &cli.Command{
		Name:  "dbaccess",
		Usage: "Выдача доступов к учебным базам данных",
		Commands: []*cli.Command{
			cmdWeb(),
			cmdMigrate(),
		},
		DefaultCommand: "web",
	}
}
