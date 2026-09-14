package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// `lumo mcp` — wedge commands for testing the MCP authorization loop without
// leaving the terminal. The flagship is `mcp test` which closes the loop:
// register → mint token → call MCP → see response. Stripe analogue:
// `stripe trigger payment_intent.succeeded`.

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Test MCP server registration and authorization",
	Long: `Inspect MCP server discovery metadata, mint short-lived tokens,
and verify your MCP server's verification flow without leaving the terminal.`,
}

var mcpServersListCmd = &cobra.Command{
	Use:   "servers",
	Short: "List registered MCP servers",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		resp, err := c.Get("/mcp/servers", nil)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}

		var result struct {
			Data []struct {
				ServerID    string   `json:"server_id"`
				Name        string   `json:"name"`
				ResourceURI string   `json:"resource_uri"`
				Scopes      []string `json:"scopes_supported"`
			} `json:"data"`
		}
		json.Unmarshal(resp, &result)

		rows := make([][]string, len(result.Data))
		for i, s := range result.Data {
			rows[i] = []string{s.ServerID, s.Name, truncate(s.ResourceURI, 40), truncate(strings.Join(s.Scopes, ","), 30)}
		}
		p.PrintTable([]string{"Server ID", "Name", "Resource URI", "Scopes"}, rows)
		return nil
	},
}

var mcpDiscoveryCmd = &cobra.Command{
	Use:   "discovery <server-id>",
	Short: "Fetch the RFC 9728 discovery document for an MCP server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := getConfigValidated()
		if err != nil {
			return err
		}
		discoveryURL := fmt.Sprintf(
			"%s/orgs/%s/api/v1/.well-known/oauth-protected-resource/mcp/%s",
			strings.TrimRight(cfg.BaseURL, "/"),
			cfg.OrgID,
			url.PathEscape(args[0]),
		)
		// Discovery is public; no auth header.
		resp, err := http.Get(discoveryURL)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		fmt.Println(string(body))
		return nil
	},
}

var mcpTestCmd = &cobra.Command{
	Use:   "test <server-id> <agent-id>",
	Short: "End-to-end test: mint a token + call the MCP server",
	Long: `Closes the MCP authorization loop in one command:

  1. Mint a short-lived bearer for <agent-id> scoped to the MCP server.
  2. POST a default tools/list JSON-RPC request to the MCP server's
     endpoint URL with Authorization: Bearer.
  3. Print the status code, response body, and a verification hint.

Use --method to override the JSON-RPC method (default: tools/list).
Use --scope to narrow the minted token; defaults to the first scope
the MCP server advertises.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverID, agentID := args[0], args[1]
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		cfg, err := getConfigValidated()
		if err != nil {
			return err
		}

		// 1. Look up the MCP server to find its endpoint_url + scopes.
		serverResp, err := c.Get(fmt.Sprintf("/mcp/servers/%s", url.PathEscape(serverID)), nil)
		if err != nil {
			return fmt.Errorf("could not load MCP server %s: %w", serverID, err)
		}
		var server struct {
			Data struct {
				ServerID    string   `json:"server_id"`
				EndpointURL string   `json:"endpoint_url"`
				Scopes      []string `json:"scopes_supported"`
			} `json:"data"`
		}
		if err := json.Unmarshal(serverResp, &server); err != nil {
			return err
		}
		if server.Data.EndpointURL == "" {
			return fmt.Errorf("MCP server %s has no endpoint_url", serverID)
		}
		scope, _ := cmd.Flags().GetString("scope")
		if scope == "" && len(server.Data.Scopes) > 0 {
			scope = server.Data.Scopes[0]
		}

		// 2. Mint a token for the agent. Same admin endpoint as `lumo agents token`.
		ttl, _ := cmd.Flags().GetInt("ttl")
		if ttl <= 0 {
			ttl = 300
		}
		mintBody := map[string]interface{}{"ttl_seconds": ttl}
		if scope != "" {
			mintBody["scope"] = scope
		}
		tokenResp, err := c.Post(fmt.Sprintf("/agents/%s/token", url.PathEscape(agentID)), mintBody)
		if err != nil {
			return fmt.Errorf("could not mint agent token: %w", err)
		}
		var token struct {
			Data struct {
				AccessToken string `json:"access_token"`
				TokenType   string `json:"token_type"`
				ExpiresIn   int    `json:"expires_in"`
				Scope       string `json:"scope"`
			} `json:"data"`
		}
		if err := json.Unmarshal(tokenResp, &token); err != nil {
			return err
		}
		if token.Data.AccessToken == "" {
			return fmt.Errorf("token mint succeeded but access_token missing in response")
		}

		// 3. POST to the MCP server with the bearer.
		method, _ := cmd.Flags().GetString("method")
		if method == "" {
			method = "tools/list"
		}
		params := json.RawMessage(`{}`)
		if v, _ := cmd.Flags().GetString("params"); v != "" {
			if !json.Valid([]byte(v)) {
				return fmt.Errorf("--params must be a valid JSON object: %s", v)
			}
			params = json.RawMessage(v)
		}
		rpc := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  method,
			"params":  params,
		}
		rpcBytes, _ := json.Marshal(rpc)

		req, err := http.NewRequest("POST", server.Data.EndpointURL, bytes.NewReader(rpcBytes))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token.Data.AccessToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		hc := &http.Client{Timeout: 30 * time.Second}
		fmt.Fprintf(os.Stderr, "→ POST %s (scope=%s, ttl=%ds)\n", server.Data.EndpointURL, scope, ttl)
		resp2, err := hc.Do(req)
		if err != nil {
			return fmt.Errorf("MCP request failed: %w", err)
		}
		defer resp2.Body.Close()
		body, _ := io.ReadAll(resp2.Body)

		fmt.Fprintf(os.Stderr, "← HTTP %d\n", resp2.StatusCode)
		if resp2.StatusCode == http.StatusUnauthorized {
			fmt.Fprintln(os.Stderr,
				"  The MCP server rejected the token. Verify it's pointing at the right discovery URL:")
			fmt.Fprintf(os.Stderr,
				"    %s/orgs/%s/api/v1/.well-known/oauth-protected-resource/mcp/%s\n",
				strings.TrimRight(cfg.BaseURL, "/"), cfg.OrgID, serverID)
		}

		// Always emit the JSON response so users can pipe to jq.
		if json.Valid(body) {
			var pretty bytes.Buffer
			if err := json.Indent(&pretty, body, "", "  "); err == nil {
				fmt.Println(pretty.String())
				if p.IsTable() && resp2.StatusCode >= 200 && resp2.StatusCode < 300 {
					p.PrintSuccess(fmt.Sprintf("MCP server accepted the token (agent=%s, scope=%s)", agentID, scope))
				}
				return nil
			}
		}
		fmt.Println(string(body))
		return nil
	},
}

func init() {
	mcpTestCmd.Flags().String("scope", "", "Token scope (defaults to first MCP-advertised scope)")
	mcpTestCmd.Flags().Int("ttl", 300, "Token TTL in seconds")
	mcpTestCmd.Flags().String("method", "tools/list", "JSON-RPC method to call")
	mcpTestCmd.Flags().String("params", "", "JSON-RPC params as a JSON object (default: {})")

	mcpCmd.AddCommand(mcpServersListCmd, mcpDiscoveryCmd, mcpTestCmd)
	rootCmd.AddCommand(mcpCmd)
}
