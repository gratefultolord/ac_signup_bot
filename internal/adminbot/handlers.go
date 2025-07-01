package adminbot

import (
	"fmt"
	"log"
	"strconv"

	"github.com/AlekSi/pointer"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/gratefultolord/ac_signup_bot/internal/db"
	"github.com/gratefultolord/ac_signup_bot/internal/files"
)

type BotService struct {
	botAPI           *tgbotapi.BotAPI
	registrationRepo *db.RegistrationRequestRepository
	userRepo         *db.UserRepository
	tokenRepo        *db.TokenRepository
	adminRepo        *db.AdminRepository
	fileService      *files.FileService
	adminStates      map[int64]*AdminState
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
		userRepo:         userRepo,
		tokenRepo:        tokenRepo,
		adminRepo:        adminRepo,
		fileService:      fileService,
		adminStates:      make(map[int64]*AdminState),
	}
}

func (b *BotService) Start(botToken string) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.botAPI.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		chatID := update.Message.Chat.ID
		text := update.Message.Text

		isAdmin, err := b.adminRepo.IsAdmin(chatID)
		if err != nil || !isAdmin {
			msg := tgbotapi.NewMessage(chatID, "Доступ запрещен")
			b.botAPI.Send(msg)
			continue
		}

		if _, exists := b.adminStates[chatID]; !exists {
			b.adminStates[chatID] = &AdminState{Step: StateMainMenu}
		}

		state := b.adminStates[chatID]

		if state.Step == StateMainMenu {
			switch text {
			case "/start", "Главное меню":
				b.handleMainMenu(chatID)
			case "Проверить заявки":
				b.handleCheckRequests(chatID)
			case "Сообщения пользователей":
				b.handleMessages(chatID)
			case "Добавить админа":
				b.handleAddAdmin(chatID)
			default:
				b.handleMainMenu(chatID)
			}
			continue
		}

		switch state.Step {
		case StateViewingRequest:
			b.handleViewRequest(chatID, text, botToken)

		case StateEnteringRejectReason:
			b.handleRejectReason(chatID, text, botToken)

		case StateEnteringRevisionReason:
			b.handleRevisionReason(chatID, text, botToken)

		case StateAddingAdmin:
			b.handleAddingAdmin(chatID, text)

		default:
			log.Printf("Unknown state %s for chatID %d", state.Step, chatID)
			b.handleMainMenu(chatID)
		}
	}
}

func (b *BotService) handleMainMenu(chatID int64) {
	b.adminStates[chatID] = &AdminState{Step: StateMainMenu}

	msg := tgbotapi.NewMessage(chatID, "Главное меню:")
	msg.ReplyMarkup = AdminMainMenu()
	b.botAPI.Send(msg)
}

func (b *BotService) handleCheckRequests(chatID int64) {
	req, err := b.registrationRepo.GetNextPending()
	if err != nil {
		log.Printf("Error loading pending request: %v\n", err)
		msg := tgbotapi.NewMessage(chatID, "Ошибка при получении заявок.")
		b.botAPI.Send(msg)
		return
	}

	if req == nil {
		msg := tgbotapi.NewMessage(chatID, "Нет новых заявок")
		msg.ReplyMarkup = AdminMainMenu()
		b.botAPI.Send(msg)
		b.adminStates[chatID] = &AdminState{Step: StateMainMenu}
		return
	}

	b.adminStates[chatID] = &AdminState{
		Step:      StateViewingRequest,
		RequestID: req.ID,
	}

	info := fmt.Sprintf(
		"Заявка #%d\nИмя: %s\nФамилия:%s\nДата рождения: %s\nСтатус: %s\nТелефон: %s",
		req.ID, req.FirstName, req.LastName, req.BirthDate.Format("02.01.2006"), req.UserStatus, req.PhoneNumber,
	)

	msg := tgbotapi.NewMessage(chatID, info)
	msg.ReplyMarkup = RequestActionButtons()
	b.botAPI.Send(msg)

	if req.DocumentPath != "" {
		doc := tgbotapi.NewDocument(chatID, tgbotapi.FilePath(req.DocumentPath))
		b.botAPI.Send(doc)
	}
}

func (b *BotService) handleMessages(chatID int64) {
	messages, err := b.adminRepo.GetLatestMessages()
	if err != nil {
		log.Printf("Error loading messages: %v\n", err)
		msg := tgbotapi.NewMessage(chatID, "Ошибка при получении сообщений")
		b.botAPI.Send(msg)
		return
	}

	if len(messages) == 0 {
		msg := tgbotapi.NewMessage(chatID, "Сообщений от пользователей пока нет")
		msg.ReplyMarkup = AdminMainMenu()
		b.botAPI.Send(msg)
		return
	}

	for _, m := range messages {
		info := fmt.Sprintf(
			"От пользователя %s %s (user_id %d)\nСообщение: %s\n---",
			m.FirstName, m.LastName, m.TelegramUserID, m.Message,
		)

		msg := tgbotapi.NewMessage(chatID, info)
		b.botAPI.Send(msg)
	}

	msg := tgbotapi.NewMessage(chatID, "Возвращаемся в меню")
	msg.ReplyMarkup = AdminMainMenu()
	b.botAPI.Send(msg)
}

func (b *BotService) handleAddAdmin(chatID int64) {
	b.adminStates[chatID].Step = StateAddingAdmin

	msg := tgbotapi.NewMessage(chatID, "Введите chat_id нового админа")
	msg.ReplyMarkup = CancelMenu()
	b.botAPI.Send(msg)
}

func (b *BotService) handleViewRequest(chatID int64, text string, botToken string) {
	state := b.adminStates[chatID]

	switch text {
	case "Одобрить":
		err := b.registrationRepo.UpdateStatus(state.RequestID, "approved", nil)
		if err != nil {
			log.Printf("Error approving request: %v\n", err)
			b.handleMainMenu(chatID)
			return
		}

		msg := tgbotapi.NewMessage(chatID, "Заявка одобрена")
		b.botAPI.Send(msg)

		req, err := b.registrationRepo.GetByID(state.RequestID)
		if err == nil {
			userBotApi, _ := tgbotapi.NewBotAPI(botToken)

			msg := tgbotapi.NewMessage(req.TelegramUserID, "Ваша заявка одобрена! Пожалуйста, оплатите подписку")
			msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
				tgbotapi.NewKeyboardButtonRow(
					tgbotapi.NewKeyboardButton("Оплатить"),
					tgbotapi.NewKeyboardButton("Написать админу"),
				),
			)

			userBotApi.Send(msg)
		}

		b.handleCheckRequests(chatID)

	case "Отклонить":
		b.adminStates[chatID].Step = StateEnteringRejectReason
		msg := tgbotapi.NewMessage(chatID, "Введите причину отклонения")
		msg.ReplyMarkup = CancelMenu()
		b.botAPI.Send(msg)

	case "На доработку":
		b.adminStates[chatID].Step = StateEnteringRevisionReason
		msg := tgbotapi.NewMessage(chatID, "Введите причину отправки на доработку")
		msg.ReplyMarkup = CancelMenu()
		b.botAPI.Send(msg)

	case "Главное меню":
		b.handleMainMenu(chatID)

	default:
		msg := tgbotapi.NewMessage(chatID, "Выберите действие")
		msg.ReplyMarkup = RequestActionButtons()
		b.botAPI.Send(msg)
	}
}

func (b *BotService) handleRejectReason(chatID int64, text string, botToken string) {
	if text == "Отмена" {
		b.handleCheckRequests(chatID)
		return
	}

	state := b.adminStates[chatID]

	err := b.registrationRepo.UpdateStatus(state.RequestID, "rejected", pointer.ToString(text))
	if err != nil {
		log.Printf("Error rejecting request: %v\n", err)
		b.handleMainMenu(chatID)
		return
	}

	msg := tgbotapi.NewMessage(chatID, "Заявка отклонена")
	b.botAPI.Send(msg)

	req, err := b.registrationRepo.GetByID(state.RequestID)
	if err == nil {
		userBotApi, _ := tgbotapi.NewBotAPI(botToken)
		info := fmt.Sprintf("Ваша заявка отклонена! Причина: %s", text)

		msg := tgbotapi.NewMessage(req.TelegramUserID, info)
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Написать админу"),
			),
		)

		userBotApi.Send(msg)
	}

	b.handleCheckRequests(chatID)
}

func (b *BotService) handleRevisionReason(chatID int64, text string, botToken string) {
	if text == "Отмена" {
		b.handleCheckRequests(chatID)
		return
	}

	state := b.adminStates[chatID]

	err := b.registrationRepo.UpdateStatus(state.RequestID, "needs_revision", pointer.ToString(text))
	if err != nil {
		log.Printf("Error updating to needs_revision status: %v\n", err)
		b.handleMainMenu(chatID)
		return
	}

	msg := tgbotapi.NewMessage(chatID, "Заявка отправлена на доработку")
	b.botAPI.Send(msg)

	req, err := b.registrationRepo.GetByID(state.RequestID)
	if err == nil {
		userBotApi, _ := tgbotapi.NewBotAPI(botToken)
		info := fmt.Sprintf("Ваша заявка требует доработки! Причина: %s", text)

		msg := tgbotapi.NewMessage(req.TelegramUserID, info)
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Загрузить новый документ"),
				tgbotapi.NewKeyboardButton("Написать админу"),
			),
		)

		userBotApi.Send(msg)
	}

	b.handleCheckRequests(chatID)
}

func (b *BotService) handleAddingAdmin(chatID int64, text string) {
	if text == "Отмена" {
		b.handleMainMenu(chatID)
		return
	}

	newChatID, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "Некорректный chat_id. Введите еще раз")
		msg.ReplyMarkup = CancelMenu()
		b.botAPI.Send(msg)
		return
	}

	err = b.adminRepo.Create(newChatID)
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "Ошибка при добавлении админа")
		b.botAPI.Send(msg)
	} else {
		msg := tgbotapi.NewMessage(chatID, "Админ успешно добавлен")
		b.botAPI.Send(msg)
	}

	b.handleMainMenu(chatID)
}
