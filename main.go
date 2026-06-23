package main

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	_ "image/gif"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

var (
	window fyne.Window

	resModeSel    *widget.RadioGroup
	customWEntry  *widget.Entry
	customHEntry  *widget.Entry
	cropModeSel   *widget.RadioGroup
	formatSel     *widget.RadioGroup
	mirrorSel     *widget.RadioGroup
	qualitySlider *widget.Slider
	qualityLabel  *widget.Label
	renameCheck   *widget.Check
	prefixEntry   *widget.Entry
	progressBar   *widget.ProgressBar
	statusLabel   *widget.Label
	fileListLabel *widget.Label

	selectedFiles []string
)

var imageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".bmp": true, ".gif": true,
}

func collectImages(path string, out *[]string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return
		}
		for _, e := range entries {
			full := filepath.Join(path, e.Name())
			if e.IsDir() {
				collectImages(full, out) // рекурсивно заходим в подпапки
			} else if imageExts[strings.ToLower(filepath.Ext(e.Name()))] {
				*out = append(*out, full)
			}
		}
	} else if imageExts[strings.ToLower(filepath.Ext(path))] {
		*out = append(*out, path)
	}
}

func updateFileListLabel() {
	if len(selectedFiles) == 0 {
		fileListLabel.SetText("Файлы не выбраны. Перетащи фото/папку в это окно или нажми кнопку ниже.")
		return
	}
	fileListLabel.SetText(fmt.Sprintf("Выбрано файлов: %d", len(selectedFiles)))
}

func showFilePicker() {
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		path := reader.URI().Path()
		reader.Close()
		selectedFiles = nil
		collectImages(path, &selectedFiles)
		updateFileListLabel()
	}, window)
	fd.Show()
}

func getTargetSize() (int, int, error) {
	switch resModeSel.Selected {
	case "1920x1080 (Full HD, стандарт RenPy)":
		return 1920, 1080, nil
	case "1280x720 (HD)":
		return 1280, 720, nil
	case "960x540":
		return 960, 540, nil
	case "854x480":
		return 854, 480, nil
	case "640x360":
		return 640, 360, nil
	case "320x180 (миниатюра)":
		return 320, 180, nil
	default:
		w, err1 := strconv.Atoi(strings.TrimSpace(customWEntry.Text))
		h, err2 := strconv.Atoi(strings.TrimSpace(customHEntry.Text))
		if err1 != nil || err2 != nil || w <= 0 || h <= 0 {
			return 0, 0, fmt.Errorf("укажи корректную ширину и высоту")
		}
		return w, h, nil
	}
}

func getCropMode() string {
	switch cropModeSel.Selected {
	case "Вписать с полями (letterbox)":
		return "letterbox"
	case "Растянуть":
		return "stretch"
	default:
		return "crop"
	}
}

func getMirrorMode() string {
	switch mirrorSel.Selected {
	case "Горизонтально":
		return "horizontal"
	case "Вертикально":
		return "vertical"
	default:
		return "none"
	}
}

func processFiles() {
	if len(selectedFiles) == 0 {
		dialog.ShowInformation("Нет файлов", "Сначала выбери фото или папку с фото.", window)
		return
	}

	targetW, targetH, err := getTargetSize()
	if err != nil {
		dialog.ShowError(err, window)
		return
	}

	cropMode := getCropMode()
	mirrorMode := getMirrorMode()
	useJPG := formatSel.Selected == "JPG"
	quality := int(qualitySlider.Value)

	outDir := "output"
	os.MkdirAll(outDir, 0755)

	total := len(selectedFiles)
	statusLabel.SetText("Обработка начата...")

	doRename := renameCheck.Checked
	prefix := strings.TrimSpace(prefixEntry.Text)
	if prefix == "" {
		prefix = "image"
	}

	var failed []string

	for i, path := range selectedFiles {
		name := filepath.Base(path)
		statusLabel.SetText(fmt.Sprintf("Обрабатываю: %s (%d/%d)", name, i+1, total))

		ok := func() bool {
			f, err := os.Open(path)
			if err != nil {
				return false
			}
			defer f.Close()

			src, _, err := image.Decode(f)
			if err != nil {
				return false
			}

			result := processImage(src, targetW, targetH, cropMode, mirrorMode)

			ext := ".jpg"
			if !useJPG {
				ext = ".png"
			}

			var outName string
			if doRename {
				outName = fmt.Sprintf("%s_%03d%s", prefix, i+1, ext)
			} else {
				outName = strings.TrimSuffix(name, filepath.Ext(name)) + ext
			}
			outPath := filepath.Join(outDir, outName)

			outFile, err := os.Create(outPath)
			if err != nil {
				return false
			}
			defer outFile.Close()

			if useJPG {
				jpeg.Encode(outFile, result, &jpeg.Options{Quality: quality})
			} else {
				png.Encode(outFile, result)
			}
			return true
		}()

		if !ok {
			failed = append(failed, name)
		}

		progressBar.SetValue(float64(i+1) / float64(total))
	}

	okCount := total - len(failed)
	statusLabel.SetText(fmt.Sprintf("Готово! Успешно: %d из %d.", okCount, total))

	msg := fmt.Sprintf("Обработано успешно: %d из %d.\nРезультат сохранён в папку \"output\" рядом с программой.", okCount, total)
	if len(failed) > 0 {
		msg += fmt.Sprintf("\n\nНе удалось обработать (%d):\n%s", len(failed), strings.Join(failed, "\n"))
	}
	dialog.ShowInformation("Готово", msg, window)
}

func main() {
	a := app.New()
	window = a.NewWindow("RenPy Photo Tool")
	window.Resize(fyne.NewSize(540, 640))

	fileListLabel = widget.NewLabel("")
	fileListLabel.Wrapping = fyne.TextWrapWord
	updateFileListLabel()

	browseBtn := widget.NewButton("Выбрать фото или папку...", func() {
		showFilePicker()
	})

	resModeSel = widget.NewRadioGroup([]string{
		"1920x1080 (Full HD, стандарт RenPy)",
		"1280x720 (HD)",
		"960x540",
		"854x480",
		"640x360",
		"320x180 (миниатюра)",
		"Своё разрешение",
	}, func(s string) {})
	resModeSel.SetSelected("1920x1080 (Full HD, стандарт RenPy)")

	customWEntry = widget.NewEntry()
	customWEntry.SetPlaceHolder("Ширина, px")
	customHEntry = widget.NewEntry()
	customHEntry.SetPlaceHolder("Высота, px")
	customRow := container.NewGridWithColumns(2, customWEntry, customHEntry)

	cropModeSel = widget.NewRadioGroup([]string{
		"Обрезать (crop)",
		"Вписать с полями (letterbox)",
		"Растянуть",
	}, func(s string) {})
	cropModeSel.SetSelected("Обрезать (crop)")

	formatSel = widget.NewRadioGroup([]string{"JPG", "PNG"}, func(s string) {})
	formatSel.SetSelected("JPG")

	mirrorSel = widget.NewRadioGroup([]string{
		"Без отзеркаливания",
		"Горизонтально",
		"Вертикально",
	}, func(s string) {})
	mirrorSel.SetSelected("Без отзеркаливания")

	qualityLabel = widget.NewLabel("Качество JPG: 85")
	qualitySlider = widget.NewSlider(10, 100)
	qualitySlider.SetValue(85)
	qualitySlider.OnChanged = func(v float64) {
		qualityLabel.SetText(fmt.Sprintf("Качество JPG: %.0f", v))
	}

	renameCheck = widget.NewCheck("Переименовать по порядку (bg_001, bg_002...)", func(b bool) {})
	prefixEntry = widget.NewEntry()
	prefixEntry.SetPlaceHolder("Префикс имени (например: bg)")
	prefixEntry.SetText("image")

	progressBar = widget.NewProgressBar()
	statusLabel = widget.NewLabel("Готов к работе")

	startBtn := widget.NewButton("Начать обработку", func() {
		go processFiles()
	})

	content := container.NewVBox(
		widget.NewLabelWithStyle("RenPy Photo Tool", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		fileListLabel,
		browseBtn,
		widget.NewSeparator(),
		widget.NewLabel("Целевое разрешение:"),
		resModeSel,
		customRow,
		widget.NewSeparator(),
		widget.NewLabel("Если пропорции не совпадают:"),
		cropModeSel,
		widget.NewSeparator(),
		widget.NewLabel("Формат вывода:"),
		formatSel,
		widget.NewSeparator(),
		widget.NewLabel("Отзеркаливание:"),
		mirrorSel,
		widget.NewSeparator(),
		qualityLabel,
		qualitySlider,
		widget.NewSeparator(),
		renameCheck,
		prefixEntry,
		widget.NewSeparator(),
		startBtn,
		progressBar,
		statusLabel,
	)

	window.SetContent(container.NewVScroll(content))

	window.SetOnDropped(func(pos fyne.Position, items []fyne.URI) {
		selectedFiles = nil
		for _, item := range items {
			collectImages(item.Path(), &selectedFiles)
		}
		updateFileListLabel()
	})

	window.ShowAndRun()
}
