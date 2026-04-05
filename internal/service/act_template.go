package service

import (
	"bytes"
	"sort"
	"text/template"
	"time"

	"github.com/mioxin/kartg/internal/models"
)

const actTemplateText = `<!DOCTYPE html>
<html lang="ru">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Акт выдачи картриджей</title>
<style>
body { font-family: Arial, sans-serif; max-width: 800px; margin: 40px auto; padding: 20px; }
h1 { text-align: center; margin-bottom: 30px; font-size: 1.5em; }
table { width: 100%; border-collapse: collapse; margin: 20px 0; }
th, td { border: 1px solid #333; padding: 8px 12px; text-align: left; }
th { background-color: #f5f5f5; }
.total { font-weight: bold; margin: 20px 0; }
.type-breakdown { margin: 10px 0 20px 20px; }
.type-breakdown li { margin: 4px 0; }
.signatures { display: flex; justify-content: space-between; margin-top: 60px; }
.signature { width: 45%; text-align: center; }
.signature-line { border-bottom: 1px solid #333; margin-top: 40px; padding-top: 8px; }
.date { margin-top: 40px; }
@media print { body { margin: 0; } }
</style>
</head>
<body>

<h1>Акт выдачи картриджей на заправку</h1>

<p>Филиал АО "Kaspi Bank" в г. Петропавловск выдал на заправку</p>
<p>в ТОО "Петроком Центр" следующие картриджи:</p>

<table>
<thead>
<tr>
<th>№</th>
<th>ID картриджа</th>
<th>Тип картриджа</th>
</tr>
</thead>
<tbody>
{{range $i, $c := .Cartridges}}
<tr><td>{{inc $i}}</td><td>{{$c.ID}}</td><td>{{$c.Model}}</td></tr>
{{end}}
</tbody>
</table>

<p class="total">Итого выдано {{.TotalCount}} картриджей, в том числе:</p>

<ul class="type-breakdown">
{{range .TypeBreakdown}}
<li>картриджей типа {{.ModelType}} - {{.Count}} шт.;</li>
{{end}}
</ul>

<div class="signatures">
<div class="signature">
<b>ФИО/Подпись заказчика</b>
<div class="signature-line">&nbsp;</div>
</div>
<div class="signature">
<b>ФИО/Подпись подрядчика</b>
<div class="signature-line">&nbsp;</div>
</div>
</div>

<p class="date"><b>Дата:</b> {{.Date}}</p>

</body>
</html>`

type TypeBreakdown struct {
	ModelType string
	Count     int
}

type ActData struct {
	Cartridges   []models.Cartridge
	TotalCount   int
	TypeBreakdown []TypeBreakdown
	Date         string
}

var actTemplate *template.Template

func init() {
	funcMap := template.FuncMap{
		"inc": func(i int) int {
			return i + 1
		},
	}

	actTemplate = template.Must(template.New("act").Funcs(funcMap).Parse(actTemplateText))
}

func generateActHTML(cartridges []models.Cartridge) string {
	// Подсчёт по типам
	typeCount := make(map[string]int)
	for _, c := range cartridges {
		if c.Model != "" {
			typeCount[c.Model]++
		} else {
			typeCount["Не указан"]++
		}
	}

	// Сортируем типы для гарантированного порядка
	var sortedModelTypes []string
	for modelType := range typeCount {
		sortedModelTypes = append(sortedModelTypes, modelType)
	}
	sort.Strings(sortedModelTypes)

	var typeBreakdown []TypeBreakdown
	for _, modelType := range sortedModelTypes {
		typeBreakdown = append(typeBreakdown, TypeBreakdown{
			ModelType: modelType,
			Count:     typeCount[modelType],
		})
	}

	data := ActData{
		Cartridges:    cartridges,
		TotalCount:    len(cartridges),
		TypeBreakdown: typeBreakdown,
		Date:          time.Now().Format("02.01.2006"),
	}

	var buf bytes.Buffer
	if err := actTemplate.Execute(&buf, data); err != nil {
		return "<html><body><h1>Ошибка генерации акта</h1></body></html>"
	}

	return buf.String()
}
