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

	"github.com/lumoauth/cli/internal/client"
	"github.com/spf13/cobra"
)

// `lumo mcp check-tool` asks the per-tool question exactly the way an MCP
// server must under MCP 2026-07-28: with the server's OWN client-credentials
// token (scope mcp:authorize), never with the token the caller sent.

const mcpAuthorizeScope = "mcp:authorize"

var mcpCheckToolCmd = &cobra.Command{
	Use:   "check-tool <server-id> <tool>",
	Short: "Ask whether a user or agent may run one tool of an MCP server",
	Long: `Runs the per-tool check that an MCP server makes before it runs a tool:

  1. Get a client-credentials token for the MCP server's own OAuth client,
     with scope mcp:authorize. The client must be listed under
     "Per-tool authorization" on the MCP server (tool_authorizer_client_ids).
  2. POST /orgs/{org}/api/v1/mcp/servers/<server-id>/tools/authorize with
     that token and the subject.

<server-id> is the registered server id (mcp_…), not the Resource URI.
--subject is agent:<agent id> or user:<user id>. The agent form also accepts
the token's sub claim as it is (agent_agt_…).

The client secret comes from --client-secret or the LUMO_MCP_CLIENT_SECRET
environment variable. The command exits with status 0 when the call is
allowed and 1 when it is denied.`,
	Example: `  LUMO_MCP_CLIENT_SECRET=... lumo mcp check-tool mcp_0123... send_reminder \
    --subject agent:agt_01HF... --client-id invoices-mcp-server`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverID, tool := args[0], args[1]
		cfg, err := getConfigValidated()
		if err != nil {
			return err
		}
		p := getPrinter()

		subjectType, subjectID, err := parseMcpSubject(mustString(cmd, "subject"))
		if err != nil {
			return err
		}
		clientID := mustString(cmd, "client-id")
		if clientID == "" {
			return fmt.Errorf("--client-id is required: the MCP server's own OAuth client")
		}
		secret := mustString(cmd, "client-secret")
		if secret == "" {
			secret = os.Getenv("LUMO_MCP_CLIENT_SECRET")
		}
		if secret == "" {
			return fmt.Errorf("pass --client-secret or set LUMO_MCP_CLIENT_SECRET")
		}

		base := fmt.Sprintf("%s/orgs/%s/api/v1", strings.TrimRight(cfg.BaseURL, "/"), url.PathEscape(cfg.OrgID))
		hc := client.NewHTTPClient(cfg.Insecure, 30*time.Second)

		// 1. The MCP server's own token. No `resource`: this token is for
		//    LumoAuth, not for an MCP server.
		form := url.Values{
			"grant_type":    {"client_credentials"},
			"client_id":     {clientID},
			"client_secret": {secret},
			"scope":         {mcpAuthorizeScope},
		}
		tokResp, err := hc.PostForm(base+"/oauth/token", form)
		if err != nil {
			return fmt.Errorf("token request failed: %w", err)
		}
		tokBody, _ := io.ReadAll(tokResp.Body)
		tokResp.Body.Close()
		var tok struct {
			AccessToken      string `json:"access_token"`
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		_ = json.Unmarshal(tokBody, &tok)
		if tokResp.StatusCode != http.StatusOK || tok.AccessToken == "" {
			return fmt.Errorf("could not get a client-credentials token (HTTP %d): %s %s", tokResp.StatusCode, tok.Error, tok.ErrorDescription)
		}

		// 2. The per-tool question.
		reqBody, _ := json.Marshal(map[string]interface{}{
			"subject": map[string]string{"type": subjectType, "id": subjectID},
			"tool":    tool,
		})
		req, err := http.NewRequest("POST", base+"/mcp/servers/"+url.PathEscape(serverID)+"/tools/authorize", bytes.NewReader(reqBody))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
		req.Header.Set("Content-Type", "application/json")
		resp, err := hc.Do(req)
		if err != nil {
			return fmt.Errorf("tool authorization request failed: %w", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		allowed, decided := mcpDecision(resp.StatusCode, body)
		if !decided {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		if !p.IsTable() {
			p.PrintResult(json.RawMessage(body))
		} else if allowed {
			p.PrintSuccess(fmt.Sprintf("Allowed: %s %s may run %s on %s", subjectType, subjectID, tool, serverID))
		} else {
			fmt.Fprintf(os.Stderr, "Denied: %s %s may not run %s on %s\n", subjectType, subjectID, tool, serverID)
		}
		if !allowed {
			os.Exit(1)
		}
		return nil
	},
}

func mustString(cmd *cobra.Command, name string) string {
	v, _ := cmd.Flags().GetString(name)
	return strings.TrimSpace(v)
}

// parseMcpSubject turns "agent:agt_x" / "user:<uuid>" into its parts.
func parseMcpSubject(raw string) (string, string, error) {
	kind, id, ok := strings.Cut(raw, ":")
	if !ok || id == "" || (kind != "agent" && kind != "user") {
		return "", "", fmt.Errorf("--subject must be agent:<agent id> or user:<user id>, got %q", raw)
	}
	return kind, id, nil
}

// mcpDecision reads the per-tool answer: 200 = allowed, 403 with
// "allowed": false = denied. Anything else is not a decision.
func mcpDecision(status int, body []byte) (allowed bool, decided bool) {
	var parsed map[string]interface{}
	if json.Unmarshal(body, &parsed) != nil {
		return false, false
	}
	v, has := parsed["allowed"].(bool)
	switch {
	case status == http.StatusOK && has && v:
		return true, true
	case status == http.StatusForbidden && has && !v:
		return false, true
	}
	return false, false
}

func init() {
	mcpCheckToolCmd.Flags().String("subject", "", "agent:<agent id> or user:<user id> (required)")
	mcpCheckToolCmd.Flags().String("client-id", "", "The MCP server's own OAuth client id (required)")
	mcpCheckToolCmd.Flags().String("client-secret", "", "Its client secret (or LUMO_MCP_CLIENT_SECRET)")
}
