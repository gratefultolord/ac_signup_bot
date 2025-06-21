package bot

import (
	"log"
	"time"

	"github.com/AlekSi/pointer"
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
		userStates:       make(map[int64]*UserState),
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
			req, err := b.registrationRepo.GetLatestByTelegramUserID(chatID)
			if err == nil && req != nil {
				switch req.Status {
				case "approved":
					b.userStates[chatID] = &UserState{Step: "awaiting_payment"}
				case "needs_revision":
					b.userStates[chatID] = &UserState{Step: "needs_revision", RequestID: req.ID}
				default:
					b.userStates[chatID] = &UserState{Step: "start"}
				}
			} else {
				b.userStates[chatID] = &UserState{Step: "start"}
			}
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

		switch state.Step {
		case "start":
			b.handleStartState(chatID)
		case "first_name":
			b.handleFirstName(chatID, text)
		case "last_name":
			b.handleLastName(chatID, text)
		case "birth_date":
			b.handleBirthDate(chatID, text)
		case "user_status":
			b.handleUserStatus(chatID, text)
		case "document":
			b.handleDocument(chatID, update.Message)
		case "phone_number":
			b.handlePhoneNumber(chatID, text)
		case "agreement":
			b.handleAgreement(chatID, text, update.Message.From.ID)
		case "write_admin":
			b.handleWriteAdminMessage(chatID, update.Message)
		case "awaiting_payment":
			b.handlePayment(chatID, text)
		default:
			log.Printf("Unknown state %s for chatID %d", state.Step, chatID)
		}
	}
}

func (b *BotService) handleStartState(chatID int64) {
	log.Printf("handleStartState for chatID %d", chatID)

	var keyboard [][]tgbotapi.KeyboardButton
	keyboard = append(keyboard, tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("Начать регистрацию"),
	))

	if b.hasRegistrationRequest(chatID) {
		keyboard = append(keyboard, tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Написать админу"),
		))
	}

	msg := tgbotapi.NewMessage(chatID, "Добро пожаловать! Выберите действие:")
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(keyboard...)
	b.botAPI.Send(msg)
}

func (b *BotService) handleStartButton(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Введите ваше имя:")
	msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	b.botAPI.Send(msg)

	b.userStates[chatID].Step = "first_name"
}

func (b *BotService) handleFirstName(chatID int64, firstName string) {
	if firstName == "" {
		msg := tgbotapi.NewMessage(chatID, "Пожалуйста, введите ваше имя:")
		b.botAPI.Send(msg)

		return
	}

	b.userStates[chatID].FirstName = firstName
	b.userStates[chatID].Step = "last_name"

	msg := tgbotapi.NewMessage(chatID, "Введите вашу фамилию:")
	b.botAPI.Send(msg)
}

func (b *BotService) handleLastName(chatID int64, lastName string) {
	if lastName == "" {
		msg := tgbotapi.NewMessage(chatID, "Пожалуйста, введите вашу фамилию:")
		b.botAPI.Send(msg)
		return
	}

	b.userStates[chatID].LastName = lastName
	b.userStates[chatID].Step = "birth_date"

	msg := tgbotapi.NewMessage(chatID, "Введите дату рождения в формате ДД.ММ.ГГГГ (например, 01.01.2000)")
	b.botAPI.Send(msg)
}

func (b *BotService) handleBirthDate(chatID int64, date string) {
	parsedDate, ok := IsValidDate(date)
	if !ok {
		msg := tgbotapi.NewMessage(chatID, "Неверный формат. Введите дату рождения в формате ДД.ММ.ГГГГ")
		b.botAPI.Send(msg)
		return
	}

	b.userStates[chatID].BirthDate = parsedDate
	b.userStates[chatID].Step = "user_status"

	msg := tgbotapi.NewMessage(chatID, "Выберите ваш статус:")
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Студент"),
			tgbotapi.NewKeyboardButton("Сотрудник"),
			tgbotapi.NewKeyboardButton("Выпускник"),
		),
	)
	b.botAPI.Send(msg)
}

func (b *BotService) handleUserStatus(chatID int64, status string) {
	textLower := NormalizeText(status)
	statusMap := map[string]string{
		"студент":   "student",
		"сотрудник": "employee",
		"выпускник": "graduate",
	}

	status, ok := statusMap[textLower]
	if !ok {
		msg := tgbotapi.NewMessage(chatID, "Пожалуйста, выберите один из вариантов: Студент, Сотрудник, Выпускник")
		b.botAPI.Send(msg)
		return
	}

	b.userStates[chatID].UserStatus = status
	b.userStates[chatID].Step = "document"

	docType := map[string]string{
		"student":  "▪️студенческого билета\n▪️пропуска",
		"employee": "▪️пропуска",
		"graduate": "▪️студенческого билета\n▪️карты выпускника",
	}

	msg := tgbotapi.NewMessage(chatID,
		"Пожалуйста, загрузите фото или скан\n"+docType[status]+"\nили другого документа, удостоверяющего принадлежность к МГИМО.")
	b.botAPI.Send(msg)
}

func (b *BotService) handlePhoneNumber(chatID int64, text string) {
	if !IsValidPhoneNumber(text) {
		msg := tgbotapi.NewMessage(chatID, "Неверный формат номера телефона. Пример: +79991234567")
		b.botAPI.Send(msg)
		return
	}

	b.userStates[chatID].PhoneNumber = text
	b.userStates[chatID].Step = "agreement"

	msg := tgbotapi.NewMessage(chatID, "Ознакомьтесь с согласием на обработку персональных данных")
	b.botAPI.Send(msg)

	// Заглушка — файл соглашения (можешь заменить на свой путь)
	doc := tgbotapi.NewDocument(chatID, tgbotapi.FilePath("agreements/personal_agreement.docx"))
	b.botAPI.Send(doc)

	confirmMsg := tgbotapi.NewMessage(chatID, "Подтвердите согласие")
	confirmMsg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Я согласен на обработку персональных данных"),
			tgbotapi.NewKeyboardButton("Не согласен на обработку персональных данных"),
		),
	)
	b.botAPI.Send(confirmMsg)
}

func (b *BotService) handleDocument(chatID int64, message *tgbotapi.Message) {
	var fileID string

	if message.Document != nil {
		fileID = message.Document.FileID
	} else if len(message.Photo) > 0 {
		fileID = message.Photo[len(message.Photo)-1].FileID
	} else {
		msg := tgbotapi.NewMessage(chatID, "Пожалуйста, загрузите документ или фото.")
		b.botAPI.Send(msg)
		return
	}

	filePath, err := b.fileService.SaveFile(fileID)
	if err != nil {
		log.Printf("Error saving file: %v", err)
		msg := tgbotapi.NewMessage(chatID, "Ошибка при сохранении файла. Попробуйте снова.")
		b.botAPI.Send(msg)
		return
	}

	b.userStates[chatID].DocumentPath = filePath
	b.userStates[chatID].Step = "phone_number"

	msg := tgbotapi.NewMessage(chatID, "Введите номер телефона (например, +79991234567):")
	b.botAPI.Send(msg)
}

func (b *BotService) handleAgreement(chatID int64, text string, telegramUserID int64) {
	if text != "Я согласен на обработку персональных данных" {
		msg := tgbotapi.NewMessage(chatID, "Пожалуйста, подтвердите согласие, нажав кнопку.")
		b.botAPI.Send(msg)
		return
	}

	state := b.userStates[chatID]

	req := db.RegistrationRequest{
		TelegramUserID: telegramUserID,
		FirstName:      state.FirstName,
		LastName:       state.LastName,
		BirthDate:      state.BirthDate,
		UserStatus:     state.UserStatus,
		DocumentPath:   state.DocumentPath,
		PhoneNumber:    state.PhoneNumber,
	}

	err := b.registrationRepo.Create(pointer.To(req))
	if err != nil {
		log.Printf("Error saving registration request: %v", err)
		msg := tgbotapi.NewMessage(chatID, "Произошла ошибка при сохранении заявки. Попробуйте снова.")
		b.botAPI.Send(msg)
		return
	}

	delete(b.userStates, chatID)

	var keyboard [][]tgbotapi.KeyboardButton
	keyboard = append(keyboard, tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("Начать регистрацию"),
	))
	if b.hasRegistrationRequest(chatID) {
		keyboard = append(keyboard, tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Написать админу"),
		))
	}

	msg := tgbotapi.NewMessage(chatID, "Заявка принята. Обработка займёт 24 часа.")
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(keyboard...)
	b.botAPI.Send(msg)
}

func (b *BotService) handleWriteAdmin(chatID int64) {
	b.userStates[chatID].Step = "write_admin"

	msg := tgbotapi.NewMessage(chatID, "Введите сообщение для администратора:")
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Отмена"),
		),
	)
	b.botAPI.Send(msg)
}

func (b *BotService) handleWriteAdminMessage(chatID int64, message *tgbotapi.Message) {
	if message.Text == "Отмена" {
		b.userStates[chatID] = &UserState{Step: "start"}
		b.handleStartState(chatID)
		return
	}

	if message.Text == "" {
		msg := tgbotapi.NewMessage(chatID, "Сообщение не может быть пустым. Введите текст:")
		b.botAPI.Send(msg)
		return
	}

	req, err := b.registrationRepo.GetLatestByTelegramUserID(chatID)
	if err != nil {
		log.Printf("Error fetching user request for admin message: %v", err)
		msg := tgbotapi.NewMessage(chatID, "Ошибка: у вас нет активной заявки. Сначала подайте заявку.")
		b.botAPI.Send(msg)
		b.userStates[chatID] = &UserState{Step: "start"}
		return
	}

	err = b.adminRepo.CreateMessage(chatID, req.FirstName, req.LastName, message.Text)
	if err != nil {
		log.Printf("Error saving admin message: %v", err)
		msg := tgbotapi.NewMessage(chatID, "Ошибка при сохранении сообщения. Попробуйте позже.")
		b.botAPI.Send(msg)
		return
	}

	var keyboard [][]tgbotapi.KeyboardButton
	keyboard = append(keyboard, tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("Начать регистрацию"),
	))
	if b.hasRegistrationRequest(chatID) {
		keyboard = append(keyboard, tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Написать админу"),
		))
	}

	msg := tgbotapi.NewMessage(chatID, "Сообщение отправлено администратору.")
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(keyboard...)
	b.botAPI.Send(msg)
}

func (b *BotService) handlePayment(chatID int64, text string) {
	if text == "Отмена" {
		b.userStates[chatID] = &UserState{Step: "start"}
		b.handleStartState(chatID)
		return
	}

	if text != "Оплатить" {
		msg := tgbotapi.NewMessage(chatID, "Пожалуйста, нажмите 'Оплатить' или 'Отмена'.")
		b.botAPI.Send(msg)
		return
	} else if text == "Оплатить" {
		authCode := GenerateAuthCode()

		req, err := b.registrationRepo.GetByTelegramID(chatID)
		if err != nil {
			log.Printf("Error fetching user for payment: %v", err)
			msg := tgbotapi.NewMessage(chatID, "Ошибка: пользователь не найден.")
			b.botAPI.Send(msg)
			b.userStates[chatID] = &UserState{Step: "start"}
			return
		}

		now := time.Now()

		err = b.usersRepo.Create(&db.UserShort{
			TelegramUserID: chatID,
			FirstName:      req.FirstName,
			LastName:       req.LastName,
			BirthDate:      req.BirthDate,
			Status:         req.UserStatus,
			PhoneNumber:    req.PhoneNumber,
			ExpiresAt:      now.AddDate(0, 1, 0),
			CreatedAt:      now,
			UpdatedAt:      now,
		})

		user, err := b.usersRepo.GetByTelegramUserID(chatID)

		tokenReq := db.Token{
			UserID:      user.ID,
			Token:       nil,
			Code:        authCode,
			PhoneNumber: user.PhoneNumber,
		}

		err = b.tokenRepo.Create(pointer.To(tokenReq))
		if err != nil {
			log.Printf("Error creating token: %v", err)
			msg := tgbotapi.NewMessage(chatID, "Ошибка при создании кода. Попробуйте позже.")
			b.botAPI.Send(msg)
			return
		}

		msg := tgbotapi.NewMessage(chatID,
			"Ваш код аутентификации: "+authCode+"\nПерейдите на сайт для завершения: https://your-site.ru")
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Начать регистрацию"),
				tgbotapi.NewKeyboardButton("Написать админу"),
			),
		)
		b.botAPI.Send(msg)
	}
}

func (b *BotService) hasRegistrationRequest(chatID int64) bool {
	req, err := b.registrationRepo.GetLatestByTelegramUserID(chatID)
	if err != nil {
		log.Printf("hasRegistrationRequest: %v", err)
		return false
	}

	return req != nil
}
