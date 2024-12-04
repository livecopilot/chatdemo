package main

import (
	"context"
	"my-chat-demo/serv"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const version = "v1"

func main() {
	root := &cobra.Command{
		Use:     "chat",
		Version: version,
		Short:   "Chat is a simple chat server",
	}

	ctx := context.Background()

	root.AddCommand(serv.NewServerStartCmd(ctx, version))

	if err := root.Execute(); err != nil {
		logrus.WithError(err).Error("Failed to execute command")
	}
}
