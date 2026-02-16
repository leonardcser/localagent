package main

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"localagent/pkg/agent"
	"localagent/pkg/bus"
	"localagent/pkg/channels"
	"localagent/pkg/config"
	"localagent/pkg/cron"
	"localagent/pkg/health"
	"localagent/pkg/heartbeat"
	"localagent/pkg/logger"
	"localagent/pkg/providers"
	"localagent/pkg/tools"
	"localagent/pkg/webchat"
)

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "onboard":
		onboardCmd()
	case "agent":
		agentCmd()
	case "gateway":
		gatewayCmd()
	case "status":
		statusCmd()
	case "version", "--version", "-v":
		fmt.Printf("localagent %s\n", version)
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printHelp()
		os.Exit(1)
	}
}

var version = "dev"

func printHelp() {
	fmt.Printf("localagent - Personal AI Agent v%s\n\n", version)
	fmt.Println("Usage: localagent <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  onboard     Initialize configuration and workspace")
	fmt.Println("  agent       Interact with the agent directly")
	fmt.Println("  gateway     Start localagent gateway (channels, heartbeat, health)")
	fmt.Println("  status      Show localagent status")
	fmt.Println("  version     Show version information")
}

func getConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".localagent", "config.json")
}

func loadConfig() (*config.Config, error) {
	return config.LoadConfig(getConfigPath())
}

func onboardCmd() {
	configPath := getConfigPath()

	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("Config already exists at %s\n", configPath)
		fmt.Print("Overwrite? (y/n): ")
		var response string
		fmt.Scanln(&response)
		if response != "y" {
			fmt.Println("Aborted.")
			return
		}
	}

	cfg := config.DefaultConfig()
	if err := config.SaveConfig(configPath, cfg); err != nil {
		fmt.Printf("Error saving config: %v\n", err)
		os.Exit(1)
	}

	workspace := cfg.WorkspacePath()
	os.MkdirAll(workspace, 0755)

	fmt.Println("localagent is ready!")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Edit config:", configPath)
	fmt.Println("  2. Chat: localagent agent -m \"Hello!\"")
}

func agentCmd() {
	message := ""
	sessionKey := "cli:default"

	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--debug", "-d":
			logger.Init(logger.LevelDebug)
		case "-m", "--message":
			if i+1 < len(args) {
				message = args[i+1]
				i++
			}
		case "-s", "--session":
			if i+1 < len(args) {
				sessionKey = args[i+1]
				i++
			}
		}
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	provider := providers.NewHTTPProvider(
		cfg.Provider.ResolveAPIKey(),
		cfg.Provider.APIBase,
		cfg.Provider.Proxy,
	)

	msgBus := bus.NewMessageBus()
	agentLoop := agent.NewAgentLoop(cfg, msgBus, provider)

	startupInfo := agentLoop.GetStartupInfo()
	logger.Info("agent initialized: tools=%d", startupInfo["tools"].(map[string]any)["count"])

	if message != "" {
		ctx := context.Background()
		response, err := agentLoop.ProcessDirect(ctx, message, sessionKey)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(response)
	} else {
		fmt.Println("localagent interactive mode (type 'exit' to quit)")
		interactiveMode(agentLoop, sessionKey)
	}
}

func interactiveMode(agentLoop *agent.AgentLoop, sessionKey string) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("\nGoodbye!")
			return
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}
		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			return
		}

		ctx := context.Background()
		response, err := agentLoop.ProcessDirect(ctx, input, sessionKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			continue
		}
		fmt.Println(response)
	}
}

func gatewayCmd() {
	args := os.Args[2:]
	for _, arg := range args {
		if arg == "--debug" || arg == "-d" {
			logger.Init(logger.LevelDebug)
			break
		}
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	provider := providers.NewHTTPProvider(
		cfg.Provider.ResolveAPIKey(),
		cfg.Provider.APIBase,
		cfg.Provider.Proxy,
	)

	msgBus := bus.NewMessageBus()
	agentLoop := agent.NewAgentLoop(cfg, msgBus, provider)

	startupInfo := agentLoop.GetStartupInfo()
	toolsInfo := startupInfo["tools"].(map[string]any)
	skillsInfo := startupInfo["skills"].(map[string]any)
	fmt.Printf("Agent: tools=%d skills=%d/%d\n", toolsInfo["count"], skillsInfo["available"], skillsInfo["total"])

	cronService := setupCronTool(agentLoop, msgBus, cfg.WorkspacePath())

	heartbeatService := heartbeat.NewHeartbeatService(
		cfg.WorkspacePath(),
		cfg.Heartbeat.Interval,
		cfg.Heartbeat.Enabled,
	)
	heartbeatService.SetBus(msgBus)
	heartbeatService.SetHandler(func(prompt, channel, chatID string) *tools.ToolResult {
		if channel == "" || chatID == "" {
			channel, chatID = "cli", "direct"
		}
		response, err := agentLoop.ProcessHeartbeat(context.Background(), prompt, channel, chatID)
		if err != nil {
			return tools.ErrorResult(fmt.Sprintf("Heartbeat error: %v", err))
		}
		if response == "HEARTBEAT_OK" {
			return tools.SilentResult("Heartbeat OK")
		}
		return tools.SilentResult(response)
	})

	channelManager, err := channels.NewManager(cfg, msgBus)
	if err != nil {
		fmt.Printf("Error creating channel manager: %v\n", err)
		os.Exit(1)
	}

	webCh := webchat.NewWebChatChannel(&cfg.WebChat, msgBus)
	webCh.SetSessionManager(agentLoop.GetSessionManager())
	channelManager.RegisterChannel("web", webCh)
	agentLoop.SetActivityEmitter(webCh)

	enabledChannels := channelManager.GetEnabledChannels()
	if len(enabledChannels) > 0 {
		fmt.Printf("Channels enabled: %s\n", enabledChannels)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := cronService.Start(); err != nil {
		fmt.Printf("Error starting cron service: %v\n", err)
	}

	if err := heartbeatService.Start(); err != nil {
		fmt.Printf("Error starting heartbeat service: %v\n", err)
	}

	if err := channelManager.StartAll(ctx); err != nil {
		fmt.Printf("Error starting channels: %v\n", err)
	}

	healthServer := health.NewServer(cfg.Gateway.Host, cfg.Gateway.Port)
	go func() {
		if err := healthServer.Start(); err != nil && err != http.ErrServerClosed {
			logger.Error("health server error: %v", err)
		}
	}()

	fmt.Printf("Gateway started on %s:%d\n", cfg.Gateway.Host, cfg.Gateway.Port)
	fmt.Println("Press Ctrl+C to stop")

	go agentLoop.Run(ctx)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	<-sigChan

	fmt.Println("\nShutting down...")
	cancel()
	healthServer.Stop(context.Background())
	heartbeatService.Stop()
	cronService.Stop()
	agentLoop.Stop()
	channelManager.StopAll(ctx)
	fmt.Println("Gateway stopped")
}

func statusCmd() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	configPath := getConfigPath()

	fmt.Printf("localagent v%s\n\n", version)

	if _, err := os.Stat(configPath); err == nil {
		fmt.Println("Config:", configPath)
	} else {
		fmt.Println("Config: not found")
	}

	workspace := cfg.WorkspacePath()
	if _, err := os.Stat(workspace); err == nil {
		fmt.Println("Workspace:", workspace)
	} else {
		fmt.Println("Workspace: not found")
	}

	fmt.Printf("Model: %s\n", cfg.Agents.Defaults.Model)
	fmt.Printf("API Base: %s\n", cfg.Provider.APIBase)

	hasKey := cfg.Provider.ResolveAPIKey() != ""
	if hasKey {
		fmt.Println("API Key: configured")
	} else {
		fmt.Println("API Key: not set")
	}
}

func setupCronTool(agentLoop *agent.AgentLoop, msgBus *bus.MessageBus, workspace string) *cron.CronService {
	cronStorePath := filepath.Join(workspace, "cron", "jobs.json")

	cronService := cron.NewCronService(cronStorePath, nil)

	cronTool := tools.NewCronTool(cronService, agentLoop, msgBus, workspace)
	agentLoop.RegisterTool(cronTool)

	cronService.SetOnJob(func(job *cron.CronJob) (string, error) {
		result := cronTool.ExecuteJob(context.Background(), job)
		return result, nil
	})

	return cronService
}
