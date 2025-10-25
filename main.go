package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	bot           *tgbotapi.BotAPI
	reports       []Report
	authorizedIDs map[int64]bool
)

type Report struct {
	Date      string  `json:"date"`
	Text      string  `json:"text"`
	TotalSum  float64 `json:"total_sum"`
	Timestamp string  `json:"timestamp"`
}

func loadReports() {
	data, err := os.ReadFile("reports.json")
	if err != nil {
		return
	}
	json.Unmarshal(data, &reports)
}

func saveReports() {
	data, _ := json.MarshalIndent(reports, "", "  ")
	os.WriteFile("reports.json", data, 0644)
}

func isAuthorized(userID int64) bool {
	if len(authorizedIDs) == 0 {
		return true
	}
	return authorizedIDs[userID]
}

func handleUpdate(update tgbotapi.Update) {
	if update.Message == nil {
		return
	}

	userID := update.Message.From.ID
	if !isAuthorized(userID) {
		bot.Send(tgbotapi.NewMessage(userID, "❌ У вас нет доступа."))
		return
	}

	if update.Message.Text == "/start" {
		msg := tgbotapi.NewMessage(userID, "Выберите действие:")
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton("📊 Анализировать новый файл")),
			tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton("📁 Посмотреть историю отчётов")),
		)
		bot.Send(msg)
		return
	}

	if update.Message.Text == "📊 Анализировать новый файл" {
		bot.Send(tgbotapi.NewMessage(userID, "Отправьте Excel-файл (.xlsx)"))
		return
	}

	if update.Message.Text == "📁 Посмотреть историю отчётов" {
		if len(reports) == 0 {
			bot.Send(tgbotapi.NewMessage(userID, "Нет сохранённых отчётов."))
			return
		}

		var buttons [][]tgbotapi.KeyboardButton
		for i, report := range reports {
			btnText := fmt.Sprintf("%d. %s - %.2f лей", i+1, report.Date, report.TotalSum)
			buttons = append(buttons, tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton(btnText)))
		}

		msg := tgbotapi.NewMessage(userID, "Выберите отчёт:")
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(buttons...)
		bot.Send(msg)
		return
	}

	// Обработка выбора отчёта
	if strings.HasPrefix(update.Message.Text, "1.") || strings.HasPrefix(update.Message.Text, "2.") ||
		strings.HasPrefix(update.Message.Text, "3.") || strings.HasPrefix(update.Message.Text, "4.") ||
		strings.HasPrefix(update.Message.Text, "5.") || strings.HasPrefix(update.Message.Text, "6.") ||
		strings.HasPrefix(update.Message.Text, "7.") || strings.HasPrefix(update.Message.Text, "8.") ||
		strings.HasPrefix(update.Message.Text, "9.") {
		parts := strings.Split(update.Message.Text, ".")
		if len(parts) > 0 {
			if idx, err := strconv.Atoi(parts[0]); err == nil {
				if idx > 0 && idx <= len(reports) {
					bot.Send(tgbotapi.NewMessage(userID, reports[idx-1].Text))
					return
				}
			}
		}
	}

	// Обработка файла
	if update.Message.Document != nil {
		doc := update.Message.Document
		if !strings.HasSuffix(doc.FileName, ".xlsx") {
			bot.Send(tgbotapi.NewMessage(userID, "Пожалуйста, отправьте файл в формате .xlsx"))
			return
		}

		bot.Send(tgbotapi.NewMessage(userID, "📥 Получаю файл..."))

		fileURL, err := bot.GetFileDirectURL(doc.FileID)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(userID, "❌ Ошибка получения файла"))
			return
		}

		resp, err := http.Get(fileURL)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(userID, "❌ Ошибка скачивания файла"))
			return
		}
		defer resp.Body.Close()

		tempFile := filepath.Join(os.TempDir(), fmt.Sprintf("temp_%d.xlsx", time.Now().UnixNano()))
		out, err := os.Create(tempFile)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(userID, "❌ Ошибка создания временного файла"))
			return
		}
		io.Copy(out, resp.Body)
		out.Close()

		result, err := analyzeExcel(tempFile)
		os.Remove(tempFile)

		if err != nil {
			bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("❌ Ошибка анализа:\n%v", err)))
			return
		}

		// Сохранить отчёт
		reports = append(reports, Report{
			Date:      result.ReportDate,
			Text:      result.Text,
			TotalSum:  result.TotalSum,
			Timestamp: time.Now().Format(time.RFC3339),
		})
		saveReports()

		// Отправить результат
		if len(result.Text) < 4000 {
			bot.Send(tgbotapi.NewMessage(userID, result.Text))
		} else {
			bot.Send(tgbotapi.NewMessage(userID, "Отчёт слишком длинный. Отправляю файл."))
		}

		// TODO: Генерация Excel-файла с результатом (опционально)
	}
}

func main() {
	// Настройки
	botToken := os.Getenv("BOT_TOKEN")
	webhookURL := os.Getenv("WEBHOOK_URL") // Например: https://your-project.vercel.app/webhook

	// Авторизованные ID
	idsStr := os.Getenv("AUTHORIZED_IDS")
	authorizedIDs = make(map[int64]bool)
	if idsStr != "" {
		for _, idStr := range strings.Split(idsStr, ",") {
			if id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64); err == nil {
				authorizedIDs[id] = true
			}
		}
	}

	var err error
	bot, err = tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal(err)
	}

	// Загрузка отчётов
	loadReports()

	if webhookURL != "" {
		// Режим webhook
		_, err = bot.SetWebhook(tgbotapi.NewWebhook(webhookURL))
		if err != nil {
			log.Fatal(err)
		}

		updates := bot.ListenForWebhook("/webhook")
		go http.ListenAndServe(":8080", nil)

		for update := range updates {
			handleUpdate(update)
		}
	} else {
		// Режим polling (для локального запуска)
		log.Printf("Authorized on account %s", bot.Self.UserName)
		u := tgbotapi.NewUpdate(0)
		u.Timeout = 60

		updates := bot.GetUpdatesChan(u)
		for update := range updates {
			handleUpdate(update)
		}
	}
}
