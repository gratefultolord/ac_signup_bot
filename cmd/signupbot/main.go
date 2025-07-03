package main

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "github.com/lib/pq"

	"github.com/gratefultolord/ac_signup_bot/internal/bot"
	"github.com/gratefultolord/ac_signup_bot/internal/config"
	"github.com/gratefultolord/ac_signup_bot/internal/db"
	"github.com/gratefultolord/ac_signup_bot/internal/files"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Error loading .env: %v", err)
	}

	database, err := db.New(cfg)
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	defer database.Close()

	err = db.RunMigrations(database.Conn, "db_scripts/init.sql", "db_scripts/admin.sql")
	if err != nil {
		log.Fatalf("Error running migrations: %v", err)
	}

	botAPI, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		log.Fatalf("Error creating telegram bot: %v", err)
	}

	registrationRepo := db.NewRegistrationRequestRepository(database.Conn)
	userRepo := db.NewUsersRepository(database.Conn)
	adminRepo := db.NewAdminRepository(database.Conn)
	tokenRepo := db.NewTokenRepository(database.Conn)

	fileService, err := files.NewFileService(botAPI, "doc_files")
	if err != nil {
		log.Fatalf("Error creating FileService: %v", err)
	}

	botService := bot.New(
		botAPI,
		registrationRepo,
		userRepo,
		tokenRepo,
		adminRepo,
		fileService,
		cfg.TelegramProviderToken,
	)

	log.Printf("Bot started as @%s", botAPI.Self.UserName)

	botService.Start()
}
