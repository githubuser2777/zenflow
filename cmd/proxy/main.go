package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"zenflow/pkg/config"
	"zenflow/pkg/filter"
	"zenflow/pkg/proxy"
	"zenflow/pkg/tui"
)

func main() {
	blocklistFlag := flag.String("blocklist", "hosts.txt", "Path or URL to the blocklist file")
	malwareBlocklistFlag := flag.String("malware-blocklist", "malware.txt", "Path or URL to the malware blocklist file")
	portFlag := flag.String("port", "8080", "Port to listen on")
	authFlag := flag.String("auth", "", "Basic auth credentials (user:pass)")
	rateLimitFlag := flag.String("rate-limit", "", "Rate limit in requests per second")
	noTUIFlag := flag.Bool("no-tui", false, "Disable TUI")
	privacyFlag := flag.Bool("privacy", true, "Enable privacy stripping")
	flag.Parse()

	if *authFlag != "" {
		parts := strings.SplitN(*authFlag, ":", 2)
		if len(parts) == 2 {
			os.Setenv("PROXY_USER", parts[0])
			os.Setenv("PROXY_PASS", parts[1])
		}
	}
	if *rateLimitFlag != "" {
		os.Setenv("PROXY_RATE_LIMIT", *rateLimitFlag)
	}

	logFile, err := os.OpenFile("proxy.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	log.SetOutput(logFile)
	defer logFile.Close()

	for {
		tui.RestartRequested = false
		runProxy(blocklistFlag, malwareBlocklistFlag, portFlag, noTUIFlag, privacyFlag)
		if !tui.RestartRequested {
			break
		}
		
		log.Println("Restarting ZenFlow Proxy in-process...")
	}
}

func runProxy(blocklistFlag, malwareBlocklistFlag, portFlag *string, noTUIFlag, privacyFlag *bool) {
	rootCtx, cancelRoot := context.WithCancel(context.Background())
	defer cancelRoot()

	log.Println("Starting ZenFlow HTTP Proxy...")

	// Initialize Ad Blocker and Config Manager
	var initialDomains []string
	if strings.HasPrefix(*blocklistFlag, "http://") || strings.HasPrefix(*blocklistFlag, "https://") {
		domains, err := config.FetchFromURL(rootCtx, *blocklistFlag)
		if err != nil {
			log.Printf("Failed to fetch initial blocklist from URL: %v", err)
		} else {
			initialDomains = domains
		}
	} else {
		domains, err := config.FetchFromFile(*blocklistFlag)
		if err != nil {
			log.Printf("Failed to read initial blocklist from file: %v", err)
		} else {
			initialDomains = domains
		}
	}

	blocker := filter.NewBlocker(initialDomains)
	log.Printf("Initialized filter with %d blocked domains", len(initialDomains))

	// Start file watcher or auto-refresh
	if strings.HasPrefix(*blocklistFlag, "http://") || strings.HasPrefix(*blocklistFlag, "https://") {
		config.StartAutoRefresh(rootCtx, *blocklistFlag, blocker, 24*time.Hour, log.Default())
	} else {
		config.WatchLocalFile(rootCtx, *blocklistFlag, blocker, 5*time.Second)
	}

	var malwareBlocker *filter.Blocker
	if *malwareBlocklistFlag != "" {
		var initialMalwareDomains []string
		if strings.HasPrefix(*malwareBlocklistFlag, "http://") || strings.HasPrefix(*malwareBlocklistFlag, "https://") {
			domains, err := config.FetchFromURL(rootCtx, *malwareBlocklistFlag)
			if err != nil {
				log.Printf("Failed to fetch initial malware blocklist from URL: %v", err)
			} else {
				initialMalwareDomains = domains
			}
		} else {
			domains, err := config.FetchFromFile(*malwareBlocklistFlag)
			if err != nil {
				log.Printf("Failed to read initial malware blocklist from file: %v", err)
			} else {
				initialMalwareDomains = domains
			}
		}

		malwareBlocker = filter.NewBlocker(initialMalwareDomains)
		log.Printf("Initialized malware filter with %d blocked domains", len(initialMalwareDomains))

		if strings.HasPrefix(*malwareBlocklistFlag, "http://") || strings.HasPrefix(*malwareBlocklistFlag, "https://") {
			config.StartAutoRefresh(rootCtx, *malwareBlocklistFlag, malwareBlocker, 24*time.Hour, log.Default())
		} else {
			config.WatchLocalFile(rootCtx, *malwareBlocklistFlag, malwareBlocker, 5*time.Second)
		}
	}

	proxyServer := proxy.NewServer(blocker, malwareBlocker, *privacyFlag)

	// Configure HTTP Server with timeouts to prevent resource leaks (RULES.md)
	addr := ":" + *portFlag
	server := &http.Server{
		Addr:         addr,
		Handler:      proxyServer,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// Graceful Shutdown setup
	go func() {
		log.Printf("Proxy listening on http://localhost%s", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Start TUI or wait for signal
	if noTUIFlag != nil && *noTUIFlag {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, os.Kill)
		<-quit
		log.Println("\nReceived interrupt signal, shutting down proxy server...")
	} else {
		p := tea.NewProgram(tui.New(blocker, malwareBlocker), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			log.Printf("Error starting TUI: %v", err)
		}
	}

	log.Println("Shutting down proxy server...")
	cancelRoot()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Proxy exited cleanly")
}
