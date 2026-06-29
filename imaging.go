package main

import (
	"image"
	"image/color"
)

// resizeBilinear делает простой билинейный ресайз изображения до заданных размеров.
func resizeBilinear(src image.Image, newW, newH int) *image.RGBA {
	bounds := src.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))

	xRatio := float64(srcW) / float64(newW)
	yRatio := float64(srcH) / float64(newH)

	for y := 0; y < newH; y++ {
		srcY := float64(y) * yRatio
		y0 := int(srcY)
		y1 := y0 + 1
		if y1 >= srcH {
			y1 = srcH - 1
		}
		yFrac := srcY - float64(y0)

		for x := 0; x < newW; x++ {
			srcX := float64(x) * xRatio
			x0 := int(srcX)
			x1 := x0 + 1
			if x1 >= srcW {
				x1 = srcW - 1
			}
			xFrac := srcX - float64(x0)

			c00 := src.At(bounds.Min.X+x0, bounds.Min.Y+y0)
			c10 := src.At(bounds.Min.X+x1, bounds.Min.Y+y0)
			c01 := src.At(bounds.Min.X+x0, bounds.Min.Y+y1)
			c11 := src.At(bounds.Min.X+x1, bounds.Min.Y+y1)

			r, g, b, a := bilerp(c00, c10, c01, c11, xFrac, yFrac)
			dst.Set(x, y, color.RGBA{r, g, b, a})
		}
	}
	return dst
}

func bilerp(c00, c10, c01, c11 color.Color, xFrac, yFrac float64) (uint8, uint8, uint8, uint8) {
	r00, g00, b00, a00 := toU8(c00)
	r10, g10, b10, a10 := toU8(c10)
	r01, g01, b01, a01 := toU8(c01)
	r11, g11, b11, a11 := toU8(c11)

	top := lerp4(r00, g00, b00, a00, r10, g10, b10, a10, xFrac)
	bot := lerp4(r01, g01, b01, a01, r11, g11, b11, a11, xFrac)

	r := lerp(top[0], bot[0], yFrac)
	g := lerp(top[1], bot[1], yFrac)
	b := lerp(top[2], bot[2], yFrac)
	a := lerp(top[3], bot[3], yFrac)
	return uint8(r), uint8(g), uint8(b), uint8(a)
}

func toU8(c color.Color) (float64, float64, float64, float64) {
	r, g, b, a := c.RGBA()
	return float64(r >> 8), float64(g >> 8), float64(b >> 8), float64(a >> 8)
}

func lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

func lerp4(r0, g0, b0, a0, r1, g1, b1, a1, t float64) [4]float64 {
	return [4]float64{lerp(r0, r1, t), lerp(g0, g1, t), lerp(b0, b1, t), lerp(a0, a1, t)}
}

// cropToAspect обрезает изображение по центру до нужного соотношения сторон.
func cropToAspect(src image.Image, targetW, targetH int) image.Image {
	bounds := src.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()
	targetRatio := float64(targetW) / float64(targetH)
	srcRatio := float64(srcW) / float64(srcH)

	var cropW, cropH int
	if srcRatio > targetRatio {
		// исходник шире нужного - обрезаем по бокам
		cropH = srcH
		cropW = int(float64(srcH) * targetRatio)
	} else {
		// исходник выше нужного - обрезаем сверху/снизу
		cropW = srcW
		cropH = int(float64(srcW) / targetRatio)
	}

	offX := bounds.Min.X + (srcW-cropW)/2
	offY := bounds.Min.Y + (srcH-cropH)/2

	cropped := image.NewRGBA(image.Rect(0, 0, cropW, cropH))
	for y := 0; y < cropH; y++ {
		for x := 0; x < cropW; x++ {
			cropped.Set(x, y, src.At(offX+x, offY+y))
		}
	}
	return cropped
}

// resizeFit уменьшает изображение с сохранением пропорций, чтобы вписать в targetW x targetH,
// но НЕ добавляет чёрные поля — возвращает картинку меньшего размера (без искажений и без полей).
func resizeFit(src image.Image, targetW, targetH int) *image.RGBA {
	bounds := src.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()
	srcRatio := float64(srcW) / float64(srcH)
	targetRatio := float64(targetW) / float64(targetH)

	var fitW, fitH int
	if srcRatio > targetRatio {
		fitW = targetW
		fitH = int(float64(targetW) / srcRatio)
	} else {
		fitH = targetH
		fitW = int(float64(targetH) * srcRatio)
	}

	return resizeBilinear(src, fitW, fitH)
}

// padToCanvas помещает уже готовое (например, очищенное от надписей) изображение
// на чёрный канвас нужного размера, по центру.
func padToCanvas(fit image.Image, targetW, targetH int) *image.RGBA {
	fitBounds := fit.Bounds()
	fitW, fitH := fitBounds.Dx(), fitBounds.Dy()

	canvas := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	for y := 0; y < targetH; y++ {
		for x := 0; x < targetW; x++ {
			canvas.Set(x, y, color.RGBA{0, 0, 0, 255})
		}
	}
	offX := (targetW - fitW) / 2
	offY := (targetH - fitH) / 2
	for y := 0; y < fitH; y++ {
		for x := 0; x < fitW; x++ {
			canvas.Set(offX+x, offY+y, fit.At(fitBounds.Min.X+x, fitBounds.Min.Y+y))
		}
	}
	return canvas
}

// letterboxFit вписывает изображение в целевой размер с сохранением пропорций, добавляя чёрные поля.
func letterboxFit(src image.Image, targetW, targetH int) *image.RGBA {
	fit := resizeFit(src, targetW, targetH)
	return padToCanvas(fit, targetW, targetH)
}

// preWatermarkImage возвращает изображение ПЕРЕД добавлением чёрных полей (для crop/stretch это уже финальный размер,
// для letterbox — уменьшенное фото без полей, чтобы поиск надписей видел настоящие углы фото).
func preWatermarkImage(src image.Image, targetW, targetH int, mode string) image.Image {
	switch mode {
	case "crop":
		cropped := cropToAspect(src, targetW, targetH)
		return resizeBilinear(cropped, targetW, targetH)
	case "letterbox":
		return resizeFit(src, targetW, targetH)
	case "stretch":
		return resizeBilinear(src, targetW, targetH)
	default:
		return resizeBilinear(src, targetW, targetH)
	}
}

// finalizeImage добавляет чёрные поля, если режим letterbox (для crop/stretch ничего не делает - картинка уже готова).
func finalizeImage(pre image.Image, targetW, targetH int, mode string) image.Image {
	if mode == "letterbox" {
		return padToCanvas(pre, targetW, targetH)
	}
	return pre
}

// processImage применяет полный пайплайн: обрезка/letterbox/растяжение -> ресайз (без учёта надписей)
func processImage(src image.Image, targetW, targetH int, mode string) image.Image {
	pre := preWatermarkImage(src, targetW, targetH, mode)
	return finalizeImage(pre, targetW, targetH, mode)
}

