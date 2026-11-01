package main

import (
	"context"
	"fmt"
	"os"

	"git.cu3rd.ru/cu3rd/db-access/cmd"
)

func main() {
	if err := cmd.NewMainApp().Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
