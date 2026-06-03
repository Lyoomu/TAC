package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Lyoomu/TAC/Agent/internal/agent"
	"github.com/Lyoomu/TAC/Agent/internal/component"
	"github.com/Lyoomu/TAC/Agent/internal/config"
	"github.com/Lyoomu/TAC/Agent/internal/crypto"
	"github.com/Lyoomu/TAC/Agent/internal/db"
	"github.com/Lyoomu/TAC/Agent/internal/models"
	"github.com/Lyoomu/TAC/Agent/internal/repository"
	"github.com/Lyoomu/TAC/Agent/internal/role"
	"github.com/Lyoomu/TAC/Agent/internal/tool"
)

type AppContext struct {
	Config          *config.Config
	DB              *sql.DB
	Encrypter       *crypto.Encrypter
	ComponentEngine *component.Engine
	ModelsEngine    *models.Engine
	RoleEngine      *role.Engine
	ToolEngine      *tool.Engine
	AgentManager    *agent.Manager
}

var appCtx *AppContext

func getExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func initContext() error {
	if appCtx != nil {
		return nil
	}

	exeDir := getExeDir()
	configDir := config.DefaultConfigDir()

	configPath := ""
	if rootCmd != nil {
		configPath, _ = rootCmd.Flags().GetString("config")
	}
	if configPath == "" {
		configPath = filepath.Join(configDir, "properties.yaml")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			configPath = filepath.Join(exeDir, "properties.yaml")
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				configPath = "properties.yaml"
			}
		}
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	baseDir := configDir
	if !filepath.IsAbs(cfg.WorkPath.DB) {
		cfg.WorkPath.DB = filepath.Join(baseDir, cfg.WorkPath.DB)
	}
	if !filepath.IsAbs(cfg.WorkPath.Tool) {
		cfg.WorkPath.Tool = filepath.Join(baseDir, cfg.WorkPath.Tool)
	}
	if !filepath.IsAbs(cfg.WorkPath.Source.Pic) {
		cfg.WorkPath.Source.Pic = filepath.Join(baseDir, cfg.WorkPath.Source.Pic)
	}
	if !filepath.IsAbs(cfg.WorkPath.Source.Video) {
		cfg.WorkPath.Source.Video = filepath.Join(baseDir, cfg.WorkPath.Source.Video)
	}
	if !filepath.IsAbs(cfg.WorkPath.Source.Sound) {
		cfg.WorkPath.Source.Sound = filepath.Join(baseDir, cfg.WorkPath.Source.Sound)
	}

	envPath := filepath.Join(configDir, ".env")
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		envPath = filepath.Join(exeDir, ".env")
		if _, err := os.Stat(envPath); os.IsNotExist(err) {
			envPath = ".env"
		}
	}
	envMap, err := config.LoadEnv(envPath)
	if err != nil {
		return fmt.Errorf("load env: %w", err)
	}
	encryptionKey := envMap["encryption_key"]
	if encryptionKey == "" {
		return fmt.Errorf("encryption_key not set in .env")
	}
	encrypter := crypto.New(encryptionKey)

	dbConn, err := db.Open(cfg.WorkPath.DB)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	if err := db.MigrateUp(dbConn); err != nil {
		return fmt.Errorf("migrate db: %w", err)
	}

	componentRepo := repository.NewComponentRepo(dbConn)
	modelRepo := repository.NewModelRepo(dbConn, encrypter)
	roleRepo := repository.NewRoleRepo(dbConn)

	componentEngine := component.NewEngine(componentRepo)
	modelsEngine := models.NewEngine(modelRepo)
	roleEngine := role.NewEngine(roleRepo, componentEngine)
	toolEngine := tool.NewEngine(dbConn, cfg.WorkPath.Tool)

	if err := toolEngine.Register(); err != nil {
		fmt.Printf("warn: tool register failed: %v\n", err)
	}

	agentManager := agent.NewManager(roleEngine, toolEngine, modelsEngine, cfg)

	appCtx = &AppContext{
		Config:          cfg,
		DB:              dbConn,
		Encrypter:       encrypter,
		ComponentEngine: componentEngine,
		ModelsEngine:    modelsEngine,
		RoleEngine:      roleEngine,
		ToolEngine:      toolEngine,
		AgentManager:    agentManager,
	}
	return nil
}

func closeContext() {
	if appCtx != nil && appCtx.DB != nil {
		appCtx.DB.Close()
	}
}
