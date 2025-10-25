package main

import (
	"fmt"
	"math"
	"strings"

	"github.com/360EntSecGroup-Skylar/excelize/v2"
)

var priorityDrinks = map[string]bool{
	"espresso":                           true,
	"double espresso decaffeinated":      true,
	"chocolate truffle":                  true,
	"sakura latte":                       true,
	"matcha latte":                       true,
	"berry raf":                          true,
	"kakao banana":                       true,
	"masala tea latte":                   true,
	"cheese & orange latte":              true,
	"double cappuccino vegan":            true,
	"flat white":                         true,
	"flat white decaffeinated":           true,
	"flat white vegan":                   true,
	"latte":                              true,
	"latte decaffeinated":                true,
	"latte vegan":                        true,
	"ice latte":                          true,
	"ice latte decaffeinated":            true,
	"espresso decaffeinated":             true,
	"ice latte vegan":                    true,
	"espresso tonic":                     true,
	"espresso tonic decaffeinated":       true,
	"bumblebee":                          true,
	"tea":                                true,
	"doppio(double espresso)":            true,
	"americano":                          true,
	"americano decaffeinated":            true,
	"cappuccino":                         true,
	"cappuccino decaffeinated":           true,
	"cacao":                              true,
	"hot chocolate":                      true,
	"cappuccino vegan":                   true,
	"double americano":                   true,
	"double cappuccino":                  true,
}

type Item struct {
	Name       string  `json:"name"`
	Quantity   float64 `json:"quantity"`
	Sum        float64 `json:"sum"`
	IsPriority bool    `json:"is_priority"`
}

type AnalysisResult struct {
	ReportDate string  `json:"report_date"`
	Text       string  `json:"text"`
	Items      []Item  `json:"items"`
	TotalSum   float64 `json:"total_sum"`
}

func isPriority(name string) bool {
	return priorityDrinks[strings.ToLower(strings.TrimSpace(name))]
}

func analyzeExcel(filePath string) (*AnalysisResult, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть файл: %v", err)
	}
	defer f.Close()

	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения строк: %v", err)
	}

	// Найти заголовки
	headerRow := -1
	var headers []string
	for i, row := range rows {
		for _, cell := range row {
			if strings.Contains(cell, "Denumire marfa") {
				headerRow = i
				headers = row
				break
			}
		}
		if headerRow != -1 {
			break
		}
	}

	if headerRow == -1 {
		return nil, fmt.Errorf("❌ не найдены заголовки")
	}

	// Найти индексы колонок
	nameIdx, qtyIdx, sumIdx, dateIdx := -1, -1, -1, -1
	for i, h := range headers {
		if h == "Denumire marfa" {
			nameIdx = i
		} else if h == "Cantitate" {
			qtyIdx = i
		} else if h == "Suma cu TVA fără reducere" {
			sumIdx = i
		} else if h == "Data" {
			dateIdx = i
		}
	}

	if nameIdx == -1 || qtyIdx == -1 || sumIdx == -1 {
		return nil, fmt.Errorf("❌ отсутствуют необходимые столбцы")
	}

	// Извлечь дату
	reportDate := "неизвестна"
	for i := headerRow + 1; i < len(rows); i++ {
		if dateIdx < len(rows[i]) && rows[i][dateIdx] != "" {
			reportDate = rows[i][dateIdx]
			break
		}
	}

	// Собрать данные
	itemsMap := make(map[string]*Item)
	for i := headerRow + 1; i < len(rows); i++ {
		row := rows[i]
		if len(row) <= nameIdx || row[nameIdx] == "" {
			continue
		}

		name := row[nameIdx]
		if strings.Contains(name, "Punga") {
			continue
		}

		quantity := 0.0
		if qtyIdx < len(row) && row[qtyIdx] != "" {
			fmt.Sscanf(row[qtyIdx], "%f", &quantity)
		}

		sum := 0.0
		if sumIdx < len(row) && row[sumIdx] != "" {
			fmt.Sscanf(row[sumIdx], "%f", &sum)
		}

		if _, exists := itemsMap[name]; !exists {
			itemsMap[name] = &Item{
				Name:       name,
				Quantity:   0,
				Sum:        0,
				IsPriority: isPriority(name),
			}
		}
		itemsMap[name].Quantity += quantity
		itemsMap[name].Sum += sum
	}

	// Преобразовать в срез
	var items []Item
	for _, item := range itemsMap {
		items = append(items, *item)
	}

	// Сортировка: сначала приоритетные, по убыванию суммы
	sortItems(items)

	// Формирование текста
	text := fmt.Sprintf("📅 Дата отчёта: %s\n\n📊 Отчёт по продажам:\n\n", reportDate)
	totalSum := 0.0
	for i, item := range items {
		if i >= 30 {
			break
		}
		totalSum += item.Sum
		text += fmt.Sprintf("%-40s %10.2f %10.2f\n", item.Name, item.Quantity, item.Sum)
	}

	if len(items) > 30 {
		text += fmt.Sprintf("\n... и ещё %d позиций. Полный отчёт — в файле.", len(items)-30)
	}

	return &AnalysisResult{
		ReportDate: reportDate,
		Text:       text,
		Items:      items,
		TotalSum:   totalSum,
	}, nil
}

func sortItems(items []Item) {
	// Простая сортировка пузырьком (для небольших данных)
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i].IsPriority == items[j].IsPriority {
				if items[i].Sum < items[j].Sum {
					items[i], items[j] = items[j], items[i]
				}
			} else if !items[i].IsPriority && items[j].IsPriority {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}
