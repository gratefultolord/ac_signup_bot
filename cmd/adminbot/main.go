package main

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/gratefultolord/ac_signup_bot/internal/adminbot"
	"github.com/gratefultolord/ac_signup_bot/internal/config"
	"github.com/gratefultolord/ac_signup_bot/internal/db"
	"github.com/gratefultolord/ac_signup_bot/internal/files"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Error loading config: %v\n", err)
	}

	database, err := db.New(cfg)
	if err != nil {
		log.Fatalf("Error connecting to database: %v\n", err)
	}
	defer database.Close()

	botApi, err := tgbotapi.NewBotAPI(cfg.AdminBotToken)
	if err != nil {
		log.Fatalf("Error creating Telegram bot: %v\n", err)
	}

	registrationRepo := db.NewRegistrationRequestRepository(database.Conn)
	userRepo := db.NewUsersRepository(database.Conn)
	tokenRepo := db.NewTokenRepository(database.Conn)
	adminRepo := db.NewAdminRepository(database.Conn)

	fileService, err := files.NewFileService(botApi, "doc_files")
	if err != nil {
		log.Fatalf("Error creating FileService: %v\n", err)
	}

	adminBotService := adminbot.New(
		botApi,
		registrationRepo,
		userRepo,
		tokenRepo,
		adminRepo,
		fileService,
	)

	log.Printf("Admin bot started as @%s\n", botApi.Self.UserName)

	adminBotService.Start(cfg.BotToken)
}
