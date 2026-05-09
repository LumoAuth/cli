package cmd

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var (
	tunnelTo     string
	tunnelEvents []string
)

func init() {
	rootCmd.AddCommand(tunnelCmd)
	tunnelCmd.Flags().StringVar(&tunnelTo, "to", "http://localhost:3000/webhooks", "Local URL to forward webhook events to")
	tunnelCmd.Flags().StringArrayVar(&tunnelEvents, "filter", nil, "Only forward these event types (repeatable)")
}

var tunnelCmd = &cobra.Command{
	Use:   "tunnel",
	Short: "Forward webhook events to a local URL (like 'stripe listen')",
	Long: `Open a temporary tunnel that forwards every webhook event for the current
tenant to a local URL. Useful for developing webhook handlers without ngrok
or a public hostname.

How it works:
  1. The CLI asks the server to register a transient 'tunnel://' webhook
     subscribed to all events.
  2. The CLI opens a Server-Sent Events stream of that webhook's deliveries.
  3. Each event is replayed as a POST to the --to URL with the original
     event name in the X-LumoAuth-Event header.
  4. On Ctrl+C, the tunnel webhook is deleted server-side.

The tunnel is per-CLI-session — running 'lumo tunnel' twice gives two
independent tunnels that each receive every event.`,
	RunE: runTunnel,
}

type tunnelStartResponse struct {
	Data struct {
		WebhookID int    `json:"webhook_id"`
		SessionID string `json:"session_id"`
	} `json:"data"`
}

func runTunnel(cmd *cobra.Command, args []string) error {
	cfg, err := getConfigValidated()
	if err != nil {
		return err
	}
	auth, err := authHeader(cfg)
	if err != nil {
		return err
	}

	transport := &http.Transport{}
	if cfg.Insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	hc := &http.Client{Transport: transport, Timeout: 30 * time.Second}

	// 1. Start the tunnel webhook.
	startURL := fmt.Sprintf("%s/orgs/%s/api/v1/admin/webhooks/tunnel/start",
		strings.TrimRight(cfg.BaseURL, "/"), cfg.OrgID)
	req, _ := http.NewRequest("POST", startURL, nil)
	req.Header.Set("Authorization", auth)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("tunnel start: %w", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("tunnel start failed (HTTP %d): %s", resp.StatusCode, string(body))
	}
	var startBody tunnelStartResponse
	if err := json.Unmarshal(body, &startBody); err != nil {
		return fmt.Errorf("decode tunnel start: %w", err)
	}
	webhookID := startBody.Data.WebhookID
	sessionID := startBody.Data.SessionID

	// Ensure cleanup runs no matter how we exit.
	cleanup := func() {
		stopURL := fmt.Sprintf("%s/orgs/%s/api/v1/admin/webhooks/tunnel/%d/stop",
			strings.TrimRight(cfg.BaseURL, "/"), cfg.OrgID, webhookID)
		stopReq, _ := http.NewRequest("POST", stopURL, nil)
		stopReq.Header.Set("Authorization", auth)
		stopReq.Header.Set("X-Requested-With", "XMLHttpRequest")
		_, _ = hc.Do(stopReq)
	}
	defer cleanup()

	// Catch Ctrl+C so the deferred cleanup actually runs.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	stopCh := make(chan struct{})
	go func() { <-sigCh; close(stopCh) }()

	fmt.Fprintf(os.Stderr, "✓ Tunnel open (session %s)\n", sessionID)
	fmt.Fprintf(os.Stderr, "  Forwarding events from %s → %s\n", cfg.OrgID, tunnelTo)
	if len(tunnelEvents) > 0 {
		fmt.Fprintf(os.Stderr, "  Filter: %s\n", strings.Join(tunnelEvents, ", "))
	}
	fmt.Fprintln(os.Stderr, "  Press Ctrl+C to close.")
	fmt.Fprintln(os.Stderr)

	streamURL := fmt.Sprintf("%s/orgs/%s/api/v1/admin/webhooks/tunnel/%d/stream",
		strings.TrimRight(cfg.BaseURL, "/"), cfg.OrgID, webhookID)

	// Reconnect loop: SSE servers cap stream lifetime to avoid stuck connections.
	for {
		select {
		case <-stopCh:
			fmt.Fprintln(os.Stderr, "\n✓ Tunnel closed")
			return nil
		default:
		}
		err := tunnelStreamOnce(stopCh, hc, streamURL, auth)
		if err != nil {
			select {
			case <-stopCh:
				return nil
			default:
			}
			fmt.Fprintf(os.Stderr, "  stream dropped (%v) — reconnecting in 2s\n", err)
			time.Sleep(2 * time.Second)
			continue
		}
	}
}

type tunnelEvent struct {
	DeliveryID string                 `json:"delivery_id"`
	EventName  string                 `json:"event_name"`
	Payload    map[string]interface{} `json:"payload"`
	ReceivedAt string                 `json:"received_at"`
}

func tunnelStreamOnce(stop <-chan struct{}, hc *http.Client, url, auth string) error {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", auth)

	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	reader := bufio.NewReader(resp.Body)
	var dataBuf strings.Builder

	for {
		select {
		case <-stop:
			return nil
		default:
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		line = strings.TrimRight(line, "\r\n")

		switch {
		case line == "":
			if dataBuf.Len() > 0 {
				handleTunnelEvent(hc, dataBuf.String())
				dataBuf.Reset()
			}
		case strings.HasPrefix(line, ":"):
			// heartbeat / comment
		case strings.HasPrefix(line, "data:"):
			if dataBuf.Len() > 0 {
				dataBuf.WriteString("\n")
			}
			dataBuf.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		case strings.HasPrefix(line, "event:"):
			// only one event type
		}
	}
}

func handleTunnelEvent(hc *http.Client, raw string) {
	var ev tunnelEvent
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		fmt.Fprintf(os.Stderr, "  ⚠ malformed event: %v\n", err)
		return
	}
	if !matchesFilter(ev.EventName) {
		return
	}

	body, _ := json.Marshal(ev.Payload)
	req, err := http.NewRequest("POST", tunnelTo, bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ⚠ build relay request: %v\n", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-LumoAuth-Event", ev.EventName)
	req.Header.Set("X-LumoAuth-Delivery", ev.DeliveryID)

	start := time.Now()
	resp, err := hc.Do(req)
	dur := time.Since(start)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ✗ %-28s  -> %s (%v)\n", ev.EventName, tunnelTo, err)
		return
	}
	resp.Body.Close()

	statusMark := "✓"
	if resp.StatusCode >= 400 {
		statusMark = "✗"
	}
	fmt.Fprintf(os.Stderr, "  %s %-28s  → HTTP %d  (%s)\n",
		statusMark, ev.EventName, resp.StatusCode, dur.Round(time.Millisecond))
}

func matchesFilter(name string) bool {
	if len(tunnelEvents) == 0 {
		return true
	}
	for _, f := range tunnelEvents {
		if f == name {
			return true
		}
	}
	return false
}
