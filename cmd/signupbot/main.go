package main

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type UserState struct {
	Step         string
	FirstName    string
	LastName     string
	BirthDate    string
	UserStatus   string
	DocumentPath string
	PhoneNumber  string
	RequestID    int    // для доработки заявки
	MessageDraft string // черновик сообщения для админа
}

type Bot struct {
	botAPI       *tgbotapi.BotAPI
	db           *sqlx.DB
	userStates   map[int64]*UserState
	docDir       string
	adminChatIDs map[int64]bool
}

func main() {
	// Загружаем .env
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	// Инициализация бота
	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatal("BOT_TOKEN not set")
	}

	botAPI, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal("Cannot initialize bot:", err)
	}

	// Инициализация базы данных
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	connStr := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable", dbUser, dbPassword, dbName)
	db, err := sqlx.Connect("postgres", connStr)
	if err != nil {
		log.Fatal("Cannot connect to database:", err)
	}
	defer db.Close()

	if err := executeSQLScripts(db, "db_scripts/init.sql", "db_scripts/admin.sql"); err != nil {
		log.Fatal("Failed to execute SQL scripts:", err)
	}

	adminChatIDs, err := loadAdmins(db)
	if err != nil {
		log.Fatal("Cannot load admins:", err)
	}

	// Создаём папку doc_files
	docDir := "doc_files"
	if err := os.MkdirAll(docDir, 0755); err != nil {
		log.Fatal("Cannot create doc_files directory:", err)
	}

	bot := &Bot{
		botAPI:       botAPI,
		db:           db,
		userStates:   make(map[int64]*UserState),
		docDir:       docDir,
		adminChatIDs: adminChatIDs,
	}

	// Настройка обновлений
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := botAPI.GetUpdatesChan(u)

	// Обработка сообщений
	for update := range updates {
		if update.Message == nil {
			continue
		}

		chatID := update.Message.Chat.ID
		text := update.Message.Text

		// Инициализация состояния пользователя
		if _, exists := bot.userStates[chatID]; !exists {
			bot.userStates[chatID] = &UserState{Step: "start"}
		}
		state := bot.userStates[chatID]

		// Обработка кнопки "Начать регистрацию"
		if state.Step == "start" {
			if text == "Начать регистрацию" {
				bot.handleStartButton(chatID)
				continue
			} else if text == "Написать админу" {
				bot.handleWriteAdmin(chatID)
				continue
			}
		}

		switch state.Step {
		case "start":
			bot.handleStartState(chatID)
		case "first_name":
			bot.handleFirstName(chatID, text)
		case "last_name":
			bot.handleLastName(chatID, text)
		case "birth_date":
			bot.handleBirthDate(chatID, text)
		case "user_status":
			bot.handleUserStatus(chatID, text)
		case "document":
			bot.handleDocument(chatID, update.Message)
		case "phone_number":
			bot.handlePhoneNumber(chatID, text)
		case "agreement":
			bot.handleAgreement(chatID, text, update.Message.From.ID)
		case "needs_revision":
			bot.handleNeedsRevision(chatID, update.Message, state.RequestID)
		case "write_admin":
			bot.handleWriteAdminMessage(chatID, update.Message)
		case "awaiting_payment":
			bot.handlePayment(chatID, text)
		}
	}
}

func (b *Bot) handleStartState(chatID int64) {
	log.Printf("Handling start for chatID: %d", chatID)

	if _, exists := b.userStates[chatID]; !exists {
		b.userStates[chatID] = &UserState{}
	}
	b.userStates[chatID].Step = "start"

	var request struct {
		ID              int    `db:"id"`
		Status          string `db:"status"`
		RejectionReason string `db:"rejection_reason"`
	}
	err := b.db.Get(&request, `
		SELECT id, status, rejection_reason
		FROM registration_requests
		WHERE telegram_user_id = $1
		ORDER BY created_at DESC
		LIMIT 1`, chatID)

	if err == nil {
		if request.Status == "needs_revision" {
			b.userStates[chatID] = &UserState{Step: "needs_revision", RequestID: request.ID}
			log.Printf("Set state to needs_revision for chatID %d, requestID %d", chatID, request.ID)
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(
				"Ваша заявка требует доработки: %s\nНажмите кнопку, чтобы загрузить новый документ",
				request.RejectionReason))
			msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
				tgbotapi.NewKeyboardButtonRow(
					tgbotapi.NewKeyboardButton("Загрузить новый документ"),
					tgbotapi.NewKeyboardButton("Написать админу"),
					tgbotapi.NewKeyboardButton("Отмена"),
				),
			)
			b.botAPI.Send(msg)
			return
		} else if request.Status == "approved" {
			b.userStates[chatID] = &UserState{Step: "awaiting_payment", RequestID: request.ID}
			log.Printf("Set state to awaiting_payment for chatID %d, requestID %d", chatID, request.ID)
			msg := tgbotapi.NewMessage(chatID, "Ваша заявка одобрена! Пожалуйста, оплатите доступ.")
			msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
				tgbotapi.NewKeyboardButtonRow(
					tgbotapi.NewKeyboardButton("Оплатить"),
					tgbotapi.NewKeyboardButton("Написать админу"),
					tgbotapi.NewKeyboardButton("Отмена"),
				),
			)
			b.botAPI.Send(msg)
			return
		}
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

	msg := tgbotapi.NewMessage(chatID, "Добро пожаловать! Выберите действие:")
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(keyboard...)
	b.botAPI.Send(msg)
}

func (b *Bot) handleStartButton(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Введите ваше имя:")
	msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	b.botAPI.Send(msg)
	b.userStates[chatID].Step = "first_name"
}

func (b *Bot) handleFirstName(chatID int64, text string) {
	if text == "" {
		b.botAPI.Send(tgbotapi.NewMessage(chatID, "Пожалуйста, введите ваше имя:"))
		return
	}
	b.userStates[chatID].FirstName = text
	b.userStates[chatID].Step = "last_name"
	b.botAPI.Send(tgbotapi.NewMessage(chatID, "Введите вашу фамилию:"))
}

func (b *Bot) handleLastName(chatID int64, text string) {
	if text == "" {
		b.botAPI.Send(tgbotapi.NewMessage(chatID, "Пожалуйста, введите вашу фамилию:"))
		return
	}
	b.userStates[chatID].LastName = text
	b.userStates[chatID].Step = "birth_date"
	b.botAPI.Send(tgbotapi.NewMessage(chatID, "Введите дату рождения в формате ДД.ММ.ГГГГ (например, 01.01.2000):"))
}

func (b *Bot) handleBirthDate(chatID int64, text string) {
	if !isValidDate(text) {
		b.botAPI.Send(tgbotapi.NewMessage(chatID, "Неверный формат. Введите дату рождения в формате ДД.ММ.ГГГГ:"))
		return
	}
	b.userStates[chatID].BirthDate = text
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

func (b *Bot) handleUserStatus(chatID int64, text string) {
	text = strings.ToLower(text)
	// Преобразуем русские значения в английские
	statusMap := map[string]string{
		"студент":   "student",
		"сотрудник": "employee",
		"выпускник": "graduate",
	}

	status, ok := statusMap[text]
	if !ok {
		log.Printf("Invalid user_status received: %s", text)
		b.botAPI.Send(tgbotapi.NewMessage(chatID, "Пожалуйста, выберите один из вариантов: Студент, Сотрудник, Выпускник"))
		return
	}

	b.userStates[chatID].UserStatus = status
	b.userStates[chatID].Step = "document"

	docType := map[string]string{
		"student":  "▪️студенческого билета\n▪️пропуска",
		"employee": "▪️пропуска",
		"graduate": "▪️студенческого билета\n▪️карты выпускника",
	}
	b.botAPI.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("Пожалуйста, загрузите фото или скан\n%s\nили любого другого документа, удостоверяющего вашу принадлежность к МГИМО", docType[status])))
}

func (b *Bot) handleDocument(chatID int64, msg *tgbotapi.Message) {
	if msg.Document == nil && msg.Photo == nil {
		b.botAPI.Send(tgbotapi.NewMessage(chatID, "Пожалуйста, загрузите документ или фото."))
		return
	}

	// Получаем FileID
	fileID := ""
	if msg.Document != nil {
		fileID = msg.Document.FileID
	} else if len(msg.Photo) > 0 {
		fileID = msg.Photo[len(msg.Photo)-1].FileID
	}

	// Скачиваем и сохраняем файл
	filePath, err := b.saveFile(fileID)
	if err != nil {
		log.Printf("Error saving file: %v", err)
		b.botAPI.Send(tgbotapi.NewMessage(chatID, "Ошибка при сохранении документа. Попробуйте снова."))
		return
	}

	b.userStates[chatID].DocumentPath = filePath
	b.userStates[chatID].Step = "phone_number"
	b.botAPI.Send(tgbotapi.NewMessage(chatID, "Введите номер телефона (например, +79991234567):"))
}

func (b *Bot) handlePhoneNumber(chatID int64, text string) {
	if !isValidPhoneNumber(text) {
		b.botAPI.Send(tgbotapi.NewMessage(chatID, "Неверный формат номера телефона. Пример: +79991234567"))
		return
	}
	b.userStates[chatID].PhoneNumber = text
	b.userStates[chatID].Step = "agreement"

	// Отправляем пользовательское соглашение (заглушка)
	agreementMsg := tgbotapi.NewMessage(chatID, "Ознакомьтесь с согласием на обработку персональных данных")
	b.botAPI.Send(agreementMsg)

	docPath := "agreements/personal_agreement.docx"
	doc := tgbotapi.NewDocument(chatID, tgbotapi.FilePath(docPath))
	b.botAPI.Send(doc)

	msg := tgbotapi.NewMessage(chatID, "Подтвердите согласие:")
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Я согласен на обработку персональных данных"),
			tgbotapi.NewKeyboardButton("Не согласен на обработку персональных данных"),
		),
	)
	b.botAPI.Send(msg)
}

func (b *Bot) handleAgreement(chatID int64, text string, telegramUserID int64) {
	if text != "Я согласен на обработку персональных данных" {
		b.botAPI.Send(tgbotapi.NewMessage(chatID, "Пожалуйста, подтвердите согласие, нажав кнопку."))
		return
	}

	// Проверяем корректность user_status перед сохранением
	state := b.userStates[chatID]
	validStatuses := map[string]bool{"student": true, "employee": true, "graduate": true}
	if !validStatuses[state.UserStatus] {
		log.Printf("Invalid user_status before saving: %s", state.UserStatus)
		b.botAPI.Send(tgbotapi.NewMessage(chatID, "Ошибка: неверный статус пользователя. Начните регистрацию заново."))
		delete(b.userStates, chatID)
		return
	}

	// Сохраняем заявку в БД
	birthDate, err := time.Parse("02.01.2006", state.BirthDate)
	if err != nil {
		log.Printf("Error parsing birth date: %v", err)
		b.botAPI.Send(tgbotapi.NewMessage(chatID, "Ошибка при обработке даты рождения."))
		return
	}

	_, err = b.db.Exec(`
        INSERT INTO registration_requests (telegram_user_id, first_name, last_name, birth_date, user_status, document_path, phone_number, status)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		telegramUserID, state.FirstName, state.LastName, birthDate, state.UserStatus, state.DocumentPath, state.PhoneNumber, "pending",
	)
	if err != nil {
		log.Printf("Error saving request: %v", err)
		b.botAPI.Send(tgbotapi.NewMessage(chatID, "Произошла ошибка при сохранении заявки. Попробуйте снова."))
		return
	}

	// Очищаем состояние
	delete(b.userStates, chatID)

	// Отправляем подтверждение
	msg := tgbotapi.NewMessage(chatID, "Заявка принята. Обработка займёт 24 часа.")
	msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	b.botAPI.Send(msg)
}

// Скачивание и сохранение файла
func (b *Bot) saveFile(fileID string) (string, error) {
	// Получаем информацию о файле
	file, err := b.botAPI.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		return "", fmt.Errorf("cannot get file: %w", err)
	}

	// Генерируем уникальное имя файла
	fileExt := filepath.Ext(file.FilePath)
	if fileExt == "" {
		fileExt = ".jpg"
	}
	uuid := uuid.New().String()
	fileName := fmt.Sprintf("%s%s", uuid, fileExt)
	filePath := filepath.Join(b.docDir, fileName)

	// Скачиваем файл
	resp, err := http.Get(file.Link(b.botAPI.Token))
	if err != nil {
		return "", fmt.Errorf("cannot download file: %w", err)
	}
	defer resp.Body.Close()

	// Создаём файл на диске
	out, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("cannot create file: %w", err)
	}
	defer out.Close()

	// Копируем содержимое
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", fmt.Errorf("cannot save file: %w", err)
	}

	return filePath, nil
}

// Валидация даты (ДД.ММ.ГГГГ)
func isValidDate(date string) bool {
	matched, _ := regexp.MatchString(`^\d{2}\.\d{2}\.\d{4}$`, date)
	if !matched {
		return false
	}
	_, err := time.Parse("02.01.2006", date)
	return err == nil
}

// Валидация номера телефона
func isValidPhoneNumber(phone string) bool {
	matched, _ := regexp.MatchString(`^\+\d{10,15}$`, phone)
	return matched
}

func (b *Bot) handleNeedsRevision(chatID int64, message *tgbotapi.Message, requestID int) {
	log.Printf("Handling needs_revision for chatID %d, requestID %d", chatID, requestID)
	if message.Text == "Отмена" {
		b.cancelAction(chatID)
		return
	}
	if message.Text == "Написать админу" {
		b.handleWriteAdmin(chatID)
		return
	}
	if message.Text == "Загрузить новый документ" {
		msg := tgbotapi.NewMessage(chatID, "Пожалуйста, загрузите новый документ или фото:")
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Отмена"),
			),
		)
		b.botAPI.Send(msg)
		return
	}

	// Check for document or photo
	var fileID string
	isPhoto := false
	if message.Document != nil {
		fileID = message.Document.FileID
		log.Printf("Received document for chatID %d, MIME type: %s", chatID, message.Document.MimeType)
	} else if len(message.Photo) > 0 {
		// Use highest quality photo
		photo := message.Photo[len(message.Photo)-1]
		fileID = photo.FileID
		isPhoto = true
		log.Printf("Received photo for chatID %d, file size: %d", chatID, photo.FileSize)
	} else {
		msg := tgbotapi.NewMessage(chatID, "Пожалуйста, загрузите документ или фото:")
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Загрузить новый документ"),
				tgbotapi.NewKeyboardButton("Написать админу"),
				tgbotapi.NewKeyboardButton("Отмена"),
			),
		)
		b.botAPI.Send(msg)
		return
	}

	// Save the file using provided saveFile method
	filePath, err := b.saveFile(fileID)
	if err != nil {
		log.Printf("Error saving file for chatID %d: %v", chatID, err)
		b.botAPI.Send(tgbotapi.NewMessage(chatID, "Ошибка при сохранении файла. Попробуйте снова."))
		return
	}

	// Remove old file if exists
	var oldPath string
	err = b.db.Get(&oldPath, `SELECT document_path FROM registration_requests WHERE id = $1`, requestID)
	if err == nil && oldPath != "" {
		if err := os.Remove(oldPath); err != nil {
			log.Printf("Error removing old file %s: %v", oldPath, err)
		}
	}

	// Update the request
	_, err = b.db.Exec(`
        UPDATE registration_requests
        SET document_path = $1, status = 'pending', rejection_reason = NULL, updated_at = CURRENT_TIMESTAMP
        WHERE id = $2 AND telegram_user_id = $3`,
		filePath, requestID, chatID)
	if err != nil {
		log.Printf("Error updating request %d for chatID %d: %v", requestID, chatID, err)
		b.botAPI.Send(tgbotapi.NewMessage(chatID, "Ошибка при обновлении заявки. Попробуйте снова."))
		return
	}

	// Notify admins
	var request struct {
		FirstName   string `db:"first_name"`
		LastName    string `db:"last_name"`
		UserStatus  string `db:"user_status"`
		PhoneNumber string `db:"phone_number"`
	}
	err = b.db.Get(&request, `
        SELECT first_name, last_name, user_status, phone_number
        FROM registration_requests
        WHERE id = $1`, requestID)
	if err != nil {
		log.Printf("Error fetching request details for notification: %v", err)
	} else {
		fileType := "документ"
		if isPhoto {
			fileType = "фото"
		}
		message := fmt.Sprintf(
			"Пользователь загрузил новый %s для заявки #%d\nИмя: %s %s\nСтатус: %s\nТелефон: %s\nФайл: %s",
			fileType, requestID, request.FirstName, request.LastName, request.UserStatus, request.PhoneNumber, filePath,
		)
		b.notifyAdmins(message)
	}

	// Confirm to user
	msg := tgbotapi.NewMessage(chatID, "Новый файл загружен. Заявка отправлена на повторную проверку.")
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Начать регистрацию"),
			tgbotapi.NewKeyboardButton("Написать админу"),
		),
	)
	b.botAPI.Send(msg)
	b.userStates[chatID].Step = "start"
	log.Printf("File uploaded for request %d, state reset to start for chatID %d", requestID, chatID)
}

func (b *Bot) notifyAdmins(message string) {
	adminBot, err := tgbotapi.NewBotAPI(os.Getenv("ADMIN_BOT_TOKEN"))
	if err != nil {
		log.Printf("Error initializing admin bot: %v", err)
		return
	}

	for adminChatID := range b.adminChatIDs {
		msg := tgbotapi.NewMessage(adminChatID, message)
		adminBot.Send(msg)
	}
}

func loadAdmins(db *sqlx.DB) (map[int64]bool, error) {
	adminChatIDs := make(map[int64]bool)
	var admins []struct {
		ChatID int64 `db:"chat_id"`
	}
	err := db.Select(&admins, `SELECT chat_id FROM admins`)
	if err != nil {
		return nil, err
	}
	for _, admin := range admins {
		adminChatIDs[admin.ChatID] = true
	}
	return adminChatIDs, nil
}

func (b *Bot) handleWriteAdmin(chatID int64) {
	log.Printf("Handling write_admin for chatID %d", chatID)
	b.userStates[chatID].Step = "write_admin"
	msg := tgbotapi.NewMessage(chatID, "Введите сообщение для администратора:")
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Отмена"),
		),
	)
	if _, err := b.botAPI.Send(msg); err != nil {
		log.Printf("Error sending write_admin prompt to chatID %d: %v", chatID, err)
	}
}

func (b *Bot) handleWriteAdminMessage(chatID int64, message *tgbotapi.Message) {
	log.Printf("Handling write_admin_message for chatID %d", chatID)
	if message.Text == "Отмена" {
		b.cancelAction(chatID)
		return
	}
	if message.Text == "" {
		b.botAPI.Send(tgbotapi.NewMessage(chatID, "Сообщение не может быть пустым. Введите текст:"))
		return
	}

	// Fetch user details
	var request struct {
		FirstName string `db:"first_name"`
		LastName  string `db:"last_name"`
	}
	err := b.db.Get(&request, `
        SELECT first_name, last_name
        FROM registration_requests
        WHERE telegram_user_id = $1
        ORDER BY created_at DESC
        LIMIT 1`, chatID)
	if err != nil {
		log.Printf("Error fetching user details for chatID %d: %v", chatID, err)
		b.botAPI.Send(tgbotapi.NewMessage(chatID, "Ошибка: у вас нет заявок. Сначала подайте заявку."))
		b.cancelAction(chatID)
		return
	}

	// Save message to admin_messages
	_, err = b.db.Exec(`
        INSERT INTO admin_messages (telegram_user_id, first_name, last_name, message, created_at)
        VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)`,
		chatID, request.FirstName, request.LastName, message.Text)
	if err != nil {
		log.Printf("Error saving admin message for chatID %d: %v", chatID, err)
		b.botAPI.Send(tgbotapi.NewMessage(chatID, "Ошибка при сохранении сообщения. Попробуйте позже."))
		return
	}

	// Confirm to user
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
	if _, err := b.botAPI.Send(msg); err != nil {
		log.Printf("Error sending confirmation to chatID %d: %v", chatID, err)
	}
	b.userStates[chatID].Step = "start"
	log.Printf("Admin message saved for chatID %d", chatID)
}

func (b *Bot) handleConfirmAdminMessage(chatID int64, text string) {
	if text == "Отмена" {
		b.cancelAction(chatID)
		return
	}

	if text == "Изменить" {
		b.userStates[chatID].Step = "write_admin"
		msg := tgbotapi.NewMessage(chatID, "Введите новое сообщение для администратора:")
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Отмена"),
			),
		)
		b.botAPI.Send(msg)
		return
	} else if text != "Отправить" {
		b.botAPI.Send(tgbotapi.NewMessage(chatID, "Пожалуйста, выберите 'Отправить', 'Изменить' или 'Отмена'."))
		return
	}

	// Получаем информацию о заявке
	var request struct {
		ID          int    `db:"id"`
		FirstName   string `db:"first_name"`
		LastName    string `db:"last_name"`
		UserStatus  string `db:"user_status"`
		PhoneNumber string `db:"phone_number"`
	}
	err := b.db.Get(&request, `
        SELECT id, first_name, last_name, user_status, phone_number
        FROM registration_requests
        WHERE telegram_user_id = $1
        ORDER BY created_at DESC
        LIMIT 1`, chatID)
	if err != nil {
		log.Printf("Error fetching request for admin message: %v", err)
		b.botAPI.Send(tgbotapi.NewMessage(chatID, "Ошибка: заявка не найдена."))
		delete(b.userStates, chatID)
		return
	}

	// Отправляем сообщение админам
	message := fmt.Sprintf(
		"Сообщение от пользователя (Заявка #%d)\nИмя: %s %s\nСтатус: %s\nТелефон: %s\nСообщение: %s",
		request.ID, request.FirstName, request.LastName, request.UserStatus, request.PhoneNumber, b.userStates[chatID].MessageDraft,
	)
	b.notifyAdmins(message)

	// Очищаем состояние
	delete(b.userStates, chatID)

	// Подтверждаем отправку
	msg := tgbotapi.NewMessage(chatID, "Сообщение отправлено администратору.")
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Начать регистрацию"),
			tgbotapi.NewKeyboardButton("Написать админу"),
		),
	)
	b.botAPI.Send(msg)
}

func (b *Bot) cancelAction(chatID int64) {
	// Очищаем состояние
	delete(b.userStates, chatID)

	// Отправляем главное меню
	msg := tgbotapi.NewMessage(chatID, "Действие отменено. Выберите действие:")
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Начать регистрацию"),
			tgbotapi.NewKeyboardButton("Написать админу"),
		),
	)
	b.botAPI.Send(msg)

	// Устанавливаем начальное состояние
	b.userStates[chatID] = &UserState{Step: "start"}
}

// executeSQLScripts выполняет указанные SQL-скрипты в базе данных
func executeSQLScripts(db *sqlx.DB, scriptPaths ...string) error {
	for _, scriptPath := range scriptPaths {
		log.Printf("Executing SQL script: %s", scriptPath)

		// Читаем файл
		file, err := os.Open(scriptPath)
		if err != nil {
			return fmt.Errorf("cannot open %s: %w", scriptPath, err)
		}
		defer file.Close()

		// Читаем содержимое
		content, err := io.ReadAll(file)
		if err != nil {
			return fmt.Errorf("cannot read %s: %w", scriptPath, err)
		}

		// Разделяем скрипт на отдельные команды по точке с запятой
		statements := strings.Split(string(content), ";")
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}

			// Выполняем команду
			_, err := db.Exec(stmt)
			if err != nil {
				// Игнорируем ошибки "уже существует" для таблиц или записей
				if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "duplicate key") {
					log.Printf("Skipping error in %s: %v", scriptPath, err)
					continue
				}
				return fmt.Errorf("error executing statement in %s: %w", scriptPath, err)
			}
		}
		log.Printf("Successfully executed %s", scriptPath)
	}
	return nil
}

func (b *Bot) hasRegistrationRequest(chatID int64) bool {
	var count int
	err := b.db.Get(&count, `
        SELECT COUNT(*)
        FROM registration_requests
        WHERE telegram_user_id = $1`, chatID)
	if err != nil {
		log.Printf("Error checking registration requests for chatID %d: %v", chatID, err)
		return false
	}
	log.Printf("Found %d registration requests for chatID %d", count, chatID)
	return count > 0
}

func (b *Bot) handlePayment(chatID int64, text string) {
	log.Printf("Handling payment for chatID %d", chatID)
	if text == "Отмена" {
		b.cancelAction(chatID)
		return
	}
	if text != "Оплатить" {
		b.botAPI.Send(tgbotapi.NewMessage(chatID, "Пожалуйста, нажмите 'Оплатить' или 'Отмена'."))
		return
	}

	// Generate 6-digit auth code
	rand.Seed(time.Now().UnixNano())
	authCode := fmt.Sprintf("%06d", rand.Intn(1000000))

	// Send auth code and link
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(
		"Ваш код аутентификации: %s\nПерейдите на сайт для завершения оплаты: [ссылка на сайт]",
		authCode))

	var keyboard [][]tgbotapi.KeyboardButton
	keyboard = append(keyboard, tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("Начать регистрацию"),
	))
	if b.hasRegistrationRequest(chatID) {
		keyboard = append(keyboard, tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Написать админу"),
		))
	}

	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(keyboard...)
	b.botAPI.Send(msg)
	b.userStates[chatID].Step = "start"
	log.Printf("Sent auth code %s to chatID %d", authCode, chatID)
}
