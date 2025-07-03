package bot

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AlekSi/pointer"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/gratefultolord/ac_signup_bot/internal/db"
	"github.com/gratefultolord/ac_signup_bot/internal/files"
)

type BotService struct {
	botAPI                *tgbotapi.BotAPI
	registrationRepo      *db.RegistrationRequestRepository
	usersRepo             *db.UserRepository
	tokenRepo             *db.TokenRepository
	adminRepo             *db.AdminRepository
	fileService           *files.FileService
	userStates            map[int64]*UserState
	telegramProviderToken string
}

func New(
	botAPI *tgbotapi.BotAPI,
	registrationRepo *db.RegistrationRequestRepository,
	userRepo *db.UserRepository,
	tokenRepo *db.TokenRepository,
	adminRepo *db.AdminRepository,
	fileService *files.FileService,
	telegramProviderToken string,
) *BotService {
	return &BotService{
		botAPI:                botAPI,
		registrationRepo:      registrationRepo,
		usersRepo:             userRepo,
		tokenRepo:             tokenRepo,
		adminRepo:             adminRepo,
		fileService:           fileService,
		userStates:            make(map[int64]*UserState),
		telegramProviderToken: telegramProviderToken,
	}
}

func (b *BotService) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.botAPI.GetUpdatesChan(u)

	for update := range updates {
		if update.PreCheckoutQuery != nil {
			b.handlePreCheckoutQuery(update.PreCheckoutQuery)
			continue
		}

		if update.Message != nil && update.Message.SuccessfulPayment != nil {
			b.handleSuccessfulPayment(update.Message)
			continue
		}

		if update.Message != nil && update.Message.Text == "Подробнее о привилегиях" {
			b.handlePrivilegesInfo(update.Message.Chat.ID)
			continue
		}

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
			b.handlePayment(chatID, text, b.telegramProviderToken)
		case "waiting_payment_confirmation":
			msg := tgbotapi.NewMessage(chatID, "Платеж уже инициирован. Пожалуйста, завершите оплату в Telegram")
			b.botAPI.Send(msg)
		default:
			log.Printf("Unknown state %s for chatID %d", state.Step, chatID)
		}
	}
}

func (b *BotService) handleStartState(chatID int64) {
	log.Printf("handleStartState for chatID %d", chatID)

	keyboard := [][]tgbotapi.KeyboardButton{
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Начать регистрацию"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Подробнее о привилегиях"),
		),
	}

	poem := strings.Join([]string{
		"Мы родились под сенью великого МГИМО —",
		"Прекраснейшей из всех земных династий.",
		"Здесь столько поколений навеки сплетено,",
		"Дай Бог ему бессмертия и счастья.",
		"                                              (с) гимн МГИМО",
	}, "\n")

	welcomeText := poem + "\n\n" +
		"В переводе с английского <b>Ambassador</b> — это не только дипломатический сотрудник высшего ранга, но и представитель сообщества, адвокат его ценностей.\n\n" +
		"<b>Ambassador card</b> — премиальная карта, созданная специально для MGIMO-family, приверженцев философии, целей и принципов главной дипломатической альма-матер страны.\n\n" +
		"Пожалуйста, пройдите короткую регистрацию. Будьте готовы подтвердить свою принадлежность к MGIMO-family.\n\n" +
		"После успешного прохождения регистрации Вам будут доступны все привилегии сообщества Ambassador card:\n\n" +
		"♦️ Доступ в закрытый чат резидентов\n" +
		"♦️ Приложение с лучшими условиями от наших партнёров\n" +
		"♦️ Информация о мероприятиях сообщества\n\n" +
		"До скорой встречи!\n\n" +
		"С наилучшими пожеланиями,\n" +
		"Команда Ambassador Card"

	msg := tgbotapi.NewMessage(chatID, welcomeText)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(keyboard...)
	b.botAPI.Send(msg)
}

func (b *BotService) handleStartButton(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Укажите Ваше имя")
	msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	b.botAPI.Send(msg)

	b.userStates[chatID].Step = "first_name"
}

func (b *BotService) handlePreCheckoutQuery(query *tgbotapi.PreCheckoutQuery) {
	confirm := tgbotapi.PreCheckoutConfig{
		PreCheckoutQueryID: query.ID,
		OK:                 true,
	}

	if _, err := b.botAPI.Request(confirm); err != nil {
		log.Printf("failed to confirm PrecheckoutQuery: %v", err)
	}
}

func (b *BotService) handlePrivilegesInfo(chatId int64) {
	log.Printf("sending privileges (media group) пользователю %d", chatId)

	files, err := os.ReadDir("privileges")
	if err != nil {
		log.Printf("failed to read /privileges: %v", err)
		msg := tgbotapi.NewMessage(chatId, "Не удалось загрузить привилегии. Попробуйте позже")
		b.botAPI.Send(msg)
		return
	}

	var media []interface{}
	count := 0

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if !strings.HasSuffix(file.Name(), ".jpg") &&
			!strings.HasSuffix(file.Name(), ".png") &&
			!strings.HasSuffix(file.Name(), ".jpeg") {
			continue
		}

		if count >= 10 {
			break
		}

		photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(filepath.Join("privileges", file.Name())))
		if count == 0 {
			photo.Caption = "Вот привилегии Ambassador card:"
		}

		media = append(media, photo)
		count++
	}

	if len(media) == 0 {
		msg := tgbotapi.NewMessage(chatId, "Пока нет доступных изображений")
		b.botAPI.Send(msg)
		return
	}

	mediaGroup := tgbotapi.MediaGroupConfig{
		ChatID: chatId,
		Media:  media,
	}

	if _, err := b.botAPI.SendMediaGroup(mediaGroup); err != nil {
		log.Printf("failed to send media group: %v", err)
		msg := tgbotapi.NewMessage(chatId, "Ошибка при отправке изображений. Попробуйте позже")
		b.botAPI.Send(msg)
	}
}

func (b *BotService) handleFirstName(chatID int64, firstName string) {
	if firstName == "" {
		msg := tgbotapi.NewMessage(chatID, "Пожалуйста, укажите Ваше имя")
		b.botAPI.Send(msg)

		return
	}

	b.userStates[chatID].FirstName = firstName
	b.userStates[chatID].Step = "last_name"

	msg := tgbotapi.NewMessage(chatID, "Укажите Вашу фамилию")
	b.botAPI.Send(msg)
}

func (b *BotService) handleLastName(chatID int64, lastName string) {
	if lastName == "" {
		msg := tgbotapi.NewMessage(chatID, "Пожалуйста, укажите Вашу фамилию")
		b.botAPI.Send(msg)
		return
	}

	b.userStates[chatID].LastName = lastName
	b.userStates[chatID].Step = "birth_date"

	msg := tgbotapi.NewMessage(chatID, "Укажите дату рождения в формате ДД.ММ.ГГГГ (например, 01.01.2000)")
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

	msg := tgbotapi.NewMessage(chatID, "Выберите ваш статус в MGIMO-family")
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
		"Пожалуйста, загрузите фото или скан\n"+docType[status]+"\nили любого другого документа, удостоверяющего вашу принадлежность к альма-матер")
	b.botAPI.Send(msg)
}

func (b *BotService) handlePhoneNumber(chatID int64, text string) {
	normalized := NormalizePhoneNumber(text)

	if !IsValidPhoneNumber(normalized) {
		msg := tgbotapi.NewMessage(chatID, "Неверный формат номера телефона. Пример: +79991234567")
		b.botAPI.Send(msg)
		return
	}

	b.userStates[chatID].PhoneNumber = normalized
	b.userStates[chatID].Step = "agreement"
	b.userStates[chatID].WaitingForPrivacyConfirmation = false

	msg := tgbotapi.NewMessage(chatID, "Ознакомьтесь с согласием на обработку персональных данных")
	b.botAPI.Send(msg)

	// Заглушка — файл соглашения (можешь заменить на свой путь)
	agreementDoc := tgbotapi.NewDocument(chatID, tgbotapi.FilePath("agreements/agreement.docx"))
	b.botAPI.Send(agreementDoc)

	agreementMessage := "Я даю согласие на обработку моих персональных данных Ambassador card (ИНН 732610083401) в целях обработки заявки и дальнейшего пользования сервисом.\n"
	confirmMsg := tgbotapi.NewMessage(chatID, agreementMessage)
	confirmMsg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Да"),
			tgbotapi.NewKeyboardButton("Нет"),
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

	msg := tgbotapi.NewMessage(chatID, "Укажите Ваш номер телефона")
	b.botAPI.Send(msg)
}

func (b *BotService) handleAgreement(chatID int64, text string, telegramUserID int64) {
	state := b.userStates[chatID]

	if text == "Нет" {
		msg := tgbotapi.NewMessage(chatID, "Вы можете продолжить регистрацию позже, когда будете готовы дать согласие.")
		msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
		b.botAPI.Send(msg)
		b.userStates[chatID] = &UserState{Step: "start"}
		return
	}

	if text == "Да" && !state.WaitingForPrivacyConfirmation {
		state.WaitingForPrivacyConfirmation = true

		cancelAgreementMessage := "В любой момент Вы можете отозвать своё согласие, написав на почту сard.ambassador@gmail.com"
		msg := tgbotapi.NewMessage(chatID, cancelAgreementMessage)
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Понятно"),
			),
		)
		b.botAPI.Send(msg)
		return
	}

	if text == "Понятно" && state.WaitingForPrivacyConfirmation {
		prePolicyMessage := "Пожалуйста, ознакомьтесь с политикой конфиденциальности."
		preMsg := tgbotapi.NewMessage(chatID, prePolicyMessage)
		b.botAPI.Send(preMsg)

		policyDoc := tgbotapi.NewDocument(
			chatID, tgbotapi.FilePath("agreements/privacy_policy.docx"))
		b.botAPI.Send(policyDoc)

		msg := tgbotapi.NewMessage(chatID, "Я подтверждаю, что ознакомлен с политикой конфиденциальности.")
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Да"),
				tgbotapi.NewKeyboardButton("Нет"),
			),
		)
		b.botAPI.Send(msg)

		return
	}

	if text == "Да" && state.WaitingForPrivacyConfirmation {
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
			log.Printf("failed to create reg request: %v", err)
			msg := tgbotapi.NewMessage(chatID, "Произошла ошибка при сохранении заявки. Попробуйте позже")
			b.botAPI.Send(msg)
			return
		}

		delete(b.userStates, chatID)

		var keyboard [][]tgbotapi.KeyboardButton

		if b.hasRegistrationRequest(chatID) {
			keyboard = append(keyboard, tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Написать админу"),
			))
		}

		msg := tgbotapi.NewMessage(chatID, "Спасибо! Ваша заявка принята, а ее обработка займёт до 24 часов. После подтверждения заявки вам придет инструкция с дальнейшими действиями.")
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(keyboard...)
		b.botAPI.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatID, "Пожалуйста, выберите один из вариантов на клавиатуре")
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

func (b *BotService) handlePayment(chatID int64, text string, telegramProviderToken string) {
	if text == "Отмена" {
		b.userStates[chatID] = &UserState{Step: "start"}
		b.handleStartState(chatID)
		return
	}

	if text != "Оплатить" {
		msg := tgbotapi.NewMessage(chatID, "Пожалуйста, нажмите 'Оплатить' или 'Отмена'.")
		b.botAPI.Send(msg)
		return
	}

	title := "Регистрация AC"
	description := "Регистрация в программе Ambassador Card"
	payload := "ac_signup_payload_" + strconv.FormatInt(chatID, 10)
	currency := "RUB"
	price := tgbotapi.LabeledPrice{
		Label:  "Регистрация",
		Amount: 250000,
	}

	invoice := tgbotapi.NewInvoice(
		chatID,
		title,
		description,
		payload,
		telegramProviderToken,
		"",
		currency,
		[]tgbotapi.LabeledPrice{price},
	)
	invoice.NeedName = false
	invoice.NeedEmail = false
	invoice.NeedPhoneNumber = false
	invoice.NeedShippingAddress = false
	invoice.IsFlexible = false

	if _, err := b.botAPI.Send(invoice); err != nil {
		log.Printf("failed to send invoice: %v", err)
		msg := tgbotapi.NewMessage(chatID, "Не удалось отправить счет. Попробуйте позже")
		b.botAPI.Send(msg)
		return
	}

	b.userStates[chatID].Step = "waiting_payment_confirmation"
}

func (b *BotService) handleSuccessfulPayment(message *tgbotapi.Message) {
	chatId := message.Chat.ID
	payment := message.SuccessfulPayment
	providerChargeId := payment.ProviderPaymentChargeID

	log.Printf("Успешный платеж от %d, charge_id: %s", chatId, providerChargeId)

	authCode := GenerateAuthCode()

	req, err := b.registrationRepo.GetByTelegramID(chatId)
	if err != nil {
		log.Printf("failed to get registration request: %v", err)
		return
	}

	now := time.Now()

	err = b.usersRepo.Create(&db.UserShort{
		TelegramUserID: chatId,
		FirstName:      req.FirstName,
		LastName:       req.LastName,
		BirthDate:      req.BirthDate,
		Status:         req.UserStatus,
		PhoneNumber:    req.PhoneNumber,
		ExpiresAt:      now.AddDate(0, 1, 0),
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	if err != nil {
		log.Printf("failed to create user: %v", err)
		return
	}

	user, err := b.usersRepo.GetByTelegramUserID(chatId)
	if err != nil {
		log.Printf("failed to get user by telegram id: %v", err)
	}

	tokenReq := db.Token{
		UserID:      user.ID,
		Token:       nil,
		Code:        authCode,
		PhoneNumber: user.PhoneNumber,
	}

	err = b.tokenRepo.Create(pointer.To(tokenReq))
	if err != nil {
		log.Printf("Error creating token: %v", err)
		msg := tgbotapi.NewMessage(chatId, "Ошибка при создании кода. Попробуйте позже.")
		b.botAPI.Send(msg)
		return
	}

	poem := strings.Join([]string{
		"Куда бы нас ни бросило по миру — мы всегда",
		"В любой стране и на любых маршрутах",
		"Уверены — нам светит путеводная звезда",
		"Над сводами родного Института.",
		"                                              (с) гимн МГИМО",
	}, "\n")

	poemMessage := tgbotapi.NewMessage(chatId, poem)
	b.botAPI.Send(poemMessage)

	welcomeText := "<b>Добро пожаловать в закрытое сообщество Ambassador card!</b>\n\n" +
		"Ссылка на закрытый чат: \n" +
		"Ссылка на приложение: https://ambassador-card.ru\n\n"

	codeMessage := fmt.Sprintf("(Код доступа в приложение: %s)\n\n", authCode)

	forAddresation := "По всем вопросам вы всегда можете обратиться по почте сard.ambassador@gmail.com."

	msg := tgbotapi.NewMessage(chatId, welcomeText+codeMessage+forAddresation)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Написать админу"),
		),
	)
	b.botAPI.Send(msg)
}

func (b *BotService) hasRegistrationRequest(chatID int64) bool {
	req, err := b.registrationRepo.GetLatestByTelegramUserID(chatID)
	if err != nil {
		log.Printf("hasRegistrationRequest: %v", err)
		return false
	}

	return req != nil
}
