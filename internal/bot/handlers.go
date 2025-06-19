package bot

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/gratefultolord/ac_signup_bot/internal/db"
	"github.com/gratefultolord/ac_signup_bot/internal/files"
)

type BotService struct {
	botAPI           *tgbotapi.BotAPI
	registrationRepo *db.RegistrationRequestRepository
	usersRepo        *db.UserRepository
	tokenRepo        *db.TokenRepository
	adminRepo        *db.AdminRepository
	fileService      *files.FileService
	userStates       map[int64]*UserState
}

func New(
	botAPI *tgbotapi.BotAPI,
	registrationRepo *db.RegistrationRequestRepository,
	userRepo *db.UserRepository,
	tokenRepo *db.TokenRepository,
	adminRepo *db.AdminRepository,
	fileService *files.FileService,
) *BotService {
	return &BotService{
		botAPI:           botAPI,
		registrationRepo: registrationRepo,
		usersRepo:        userRepo,
		tokenRepo:        tokenRepo,
		adminRepo:        adminRepo,
		fileService:      fileService,
	}
}

func (b *BotService) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.botAPI.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		chatID := update.Message.Chat.ID
		text := update.Message.Text

		// Инициализируем state для нового юзера
		if _, exists := b.userStates[chatID]; !exists {
			b.userStates[chatID] = &UserState{Step: "start"}
		}

		state := b.userStates[chatID]

		// Главное меню
		if state.Step == "start" {
			if text == "Начать регистрацию" {
				b.handleStartButton(chatID)
				continue
			} else if text == "Написать админу" && b.hasRegistrationRequest(chatID) {
				b.handleWriteAdmin(chatID)
				continue
			} else if text == "Написать админу" && !b.hasRegistrationRequest(chatID) {
				msg := tgbotapi.NewMessage(chatID, "Вы сможете написать админу после отправки заявки.")
				b.botAPI.Send(msg)
				continue
			}
		}
	}
}

func (b *BotService) handleStartButton(chatID int64) error {
	return nil
}

func (b *BotService) hasRegistrationRequest(chatID int64) bool { return false }

func (b *BotService) handleWriteAdmin(chatID int64) error { return nil }
