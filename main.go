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
	"fyne.io/fyne/v2/canvas"
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
	qualitySlider *widget.Slider
	qualityLabel  *widget.Label
	renameCheck   *widget.Check
	prefixEntry   *widget.Entry
	mirrorCheck   *widget.Check
	progressBar   *widget.ProgressBar
	statusLabel   *widget.Label
	fileListLabel *widget.Label

	wmModeSel      *widget.RadioGroup
	wmMethodSel    *widget.RadioGroup
	wmHeightSlider *widget.Slider
	wmHeightLabel  *widget.Label
	wmLeftSlider   *widget.Slider
	wmLeftLabel    *widget.Label
	wmRightSlider  *widget.Slider
	wmRightLabel   *widget.Label
	wmBlurSlider   *widget.Slider
	wmBlurLabel    *widget.Label
	wmMethodBox    *fyne.Container
	wmPreviewImg   *canvas.Image
	wmPreviewInfo  *widget.Label

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
		doWatermarkPreview()
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

func getWmMode() string {
	switch wmModeSel.Selected {
	case "Удалить надпись (размытие/размазывание зоны)":
		return "remove"
	case "Обрезать часть фото с надписью":
		return "crop"
	default:
		return "none"
	}
}

func getWmMethod() string {
	if wmMethodSel.Selected == "Размытие" {
		return "blur"
	}
	return "mirror"
}

func doWatermarkPreview() {
	if len(selectedFiles) == 0 {
		return
	}
	if wmPreviewInfo == nil {
		return
	}

	targetW, targetH, err := getTargetSize()
	if err != nil {
		return
	}
	cropMode := getCropMode()

	wmPreviewInfo.SetText("Обновляю превью...")

	go func() {
		f, err := os.Open(selectedFiles[0])
		if err != nil {
			wmPreviewInfo.SetText("Не удалось открыть файл для превью.")
			return
		}
		defer f.Close()
		src, _, err := image.Decode(f)
		if err != nil {
			wmPreviewInfo.SetText("Не удалось декодировать файл для превью.")
			return
		}

		resized := preWatermarkImage(src, targetW, targetH, cropMode)

		mode := getWmMode()
		var overlay image.Image

		switch mode {
		case "remove":
			zone := fixedZoneRect(resized.Bounds(), wmHeightSlider.Value, wmLeftSlider.Value, wmRightSlider.Value)
			overlay = removeFixedZone(resized, zone, getWmMethod(), int(wmBlurSlider.Value))
			wmPreviewInfo.SetText("Так будет выглядеть результат очистки. Подбирай слайдеры, пока не устроит качество.")
		case "crop":
			overlay = cropZoneOut(resized, wmHeightSlider.Value, wmLeftSlider.Value, wmRightSlider.Value)
			wmPreviewInfo.SetText("Так будет выглядеть фото после обрезки нижней части. Итоговый размер подгонится под целевое разрешение отдельно.")
		default:
			zone := fixedZoneRect(resized.Bounds(), wmHeightSlider.Value, wmLeftSlider.Value, wmRightSlider.Value)
			overlay = drawZoneOutline(resized, zone)
			wmPreviewInfo.SetText("Красная рамка показывает зону с надписью (для справки). Выбери режим выше, чтобы что-то с ней сделать.")
		}

		wmPreviewImg.Image = overlay
		wmPreviewImg.Refresh()
	}()
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

			wmMode := getWmMode()
			workSrc := src
			if wmMode == "crop" {
				workSrc = cropZoneOut(src, wmHeightSlider.Value, wmLeftSlider.Value, wmRightSlider.Value)
			}

			pre := preWatermarkImage(workSrc, targetW, targetH, cropMode)

			if wmMode == "remove" {
				zone := fixedZoneRect(pre.Bounds(), wmHeightSlider.Value, wmLeftSlider.Value, wmRightSlider.Value)
				pre = removeFixedZone(pre, zone, getWmMethod(), int(wmBlurSlider.Value))
			}

			result := finalizeImage(pre, targetW, targetH, cropMode)

			if mirrorCheck.Checked {
				result = mirrorImage(result)
			}

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

	wmModeSel = widget.NewRadioGroup([]string{
		"Ничего не делать с надписью",
		"Удалить надпись (размытие/размазывание зоны)",
		"Обрезать часть фото с надписью",
	}, func(s string) {
		if wmMethodBox != nil {
			if s == "Удалить надпись (размытие/размазывание зоны)" {
				wmMethodBox.Show()
			} else {
				wmMethodBox.Hide()
			}
		}
		doWatermarkPreview()
	})
	wmModeSel.SetSelected("Ничего не делать с надписью")

	wmMethodSel = widget.NewRadioGroup([]string{
		"Размытие",
		"Размазывание соседних пикселей",
	}, func(s string) {
		doWatermarkPreview()
	})
	wmMethodSel.SetSelected("Размытие")

	wmBlurLabel = widget.NewLabel("Сила размытия: 14")
	wmBlurSlider = widget.NewSlider(4, 30)
	wmBlurSlider.SetValue(14)
	wmBlurSlider.OnChanged = func(v float64) {
		wmBlurLabel.SetText(fmt.Sprintf("Сила размытия: %.0f", v))
		doWatermarkPreview()
	}

	wmHeightLabel = widget.NewLabel("Высота зоны снизу: 10% от фото")
	wmHeightSlider = widget.NewSlider(2, 40)
	wmHeightSlider.SetValue(10)
	wmHeightSlider.OnChanged = func(v float64) {
		wmHeightLabel.SetText(fmt.Sprintf("Высота зоны снизу: %.0f%% от фото", v))
		doWatermarkPreview()
	}

	wmLeftLabel = widget.NewLabel("Отступ слева: 0%")
	wmLeftSlider = widget.NewSlider(0, 45)
	wmLeftSlider.SetValue(0)
	wmLeftSlider.OnChanged = func(v float64) {
		wmLeftLabel.SetText(fmt.Sprintf("Отступ слева: %.0f%%", v))
		doWatermarkPreview()
	}

	wmRightLabel = widget.NewLabel("Отступ справа: 0%")
	wmRightSlider = widget.NewSlider(0, 45)
	wmRightSlider.SetValue(0)
	wmRightSlider.OnChanged = func(v float64) {
		wmRightLabel.SetText(fmt.Sprintf("Отступ справа: %.0f%%", v))
		doWatermarkPreview()
	}

	wmPreviewImg = canvas.NewImageFromImage(nil)
	wmPreviewImg.FillMode = canvas.ImageFillContain
	wmPreviewImg.SetMinSize(fyne.NewSize(400, 225))
	wmPreviewInfo = widget.NewLabel("Нажми «Проверить на первом фото», чтобы увидеть, что найдёт программа.")
	wmPreviewInfo.Wrapping = fyne.TextWrapWord

	wmMethodBox = container.NewVBox(
		wmMethodSel,
		wmBlurLabel,
		wmBlurSlider,
	)
	wmMethodBox.Hide()

	wmPreviewBtn := widget.NewButton("Обновить превью", func() {
		doWatermarkPreview()
	})

	wmBox := container.NewVBox(
		wmModeSel,
		wmMethodBox,
		wmHeightLabel,
		wmHeightSlider,
		wmLeftLabel,
		wmLeftSlider,
		wmRightLabel,
		wmRightSlider,
		wmPreviewBtn,
		wmPreviewImg,
		wmPreviewInfo,
	)

	mirrorCheck = widget.NewCheck("Отзеркалить фото горизонтально", func(b bool) {})
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
		qualityLabel,
		qualitySlider,
		widget.NewSeparator(),
		renameCheck,
		prefixEntry,
		widget.NewSeparator(),
		mirrorCheck,
		widget.NewSeparator(),
		widget.NewLabel("Удаление надписей в углах:"),
		wmBox,
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
		doWatermarkPreview()
	})

	window.ShowAndRun()
}
