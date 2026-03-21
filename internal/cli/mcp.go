package cli

import (
	"os"

	"github.com/kogungor/bifrost/internal/mcp"
	"github.com/spf13/cobra"
)

var mcpServeCmd = &cobra.Command{
	Use:    "mcp-serve",
	Short:  "Run Bifrost as an MCP server (stdio JSON-RPC)",
	Hidden: true,
	RunE:   runMCPServe,
}

func init() {
	rootCmd.AddCommand(mcpServeCmd)
}

func runMCPServe(cmd *cobra.Command, args []string) error {
	projectRoot, err := resolveProject()
	if err != nil {
		return err
	}

	return mcp.Serve(projectRoot, Version, os.Stdin, os.Stdout)
}
