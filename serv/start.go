package serv

import (
	"context"

	"github.com/spf13/cobra"
)

type ServerStartOptions struct {
	id     string
	listen string
}

func NewServerStartCmd(ctx context.Context, version string) *cobra.Command {
	opts := &ServerStartOptions{}

	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Start the server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunServerStart(ctx, version, opts)
		},
	}

	cmd.PersistentFlags().StringVarP(&opts.id, "serverId", "i", "demo", "Server ID")
	cmd.PersistentFlags().StringVarP(&opts.listen, "listen", "l", ":8080", "Listen address")

	return cmd
}

func RunServerStart(ctx context.Context, version string, opts *ServerStartOptions) error {
	s := NewServer(opts.id, opts.listen)
	defer s.Shutdown()

	return s.Start()
}



