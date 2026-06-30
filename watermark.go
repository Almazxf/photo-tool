package main

import (
	"image"
	"image/color"
	"math"
)

// toGray быстро переводит пиксель в яркость (без создания целого grayscale-изображения)
func luminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	return 0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(b>>8)
}

// edgeDensity считает среднюю величину градиента (резкость/контраст) в области rect
func edgeDensity(img image.Image, rect image.Rectangle) float64 {
	bounds := img.Bounds()
	r := rect.Intersect(bounds)
	if r.Dx() < 3 || r.Dy() < 3 {
		return 0
	}
	var sum float64
	var count int
	step := 1
	if r.Dx()*r.Dy() > 200000 {
		step = 2
	}
	for y := r.Min.Y; y < r.Max.Y-1; y += step {
		for x := r.Min.X; x < r.Max.X-1; x += step {
			l0 := luminance(img.At(x, y))
			l1 := luminance(img.At(x+1, y))
			l2 := luminance(img.At(x, y+1))
			gx := l1 - l0
			gy := l2 - l0
			sum += math.Sqrt(gx*gx + gy*gy)
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

// maxBlockDensity разбивает rect на блоки blockSize x blockSize и возвращает максимальную
// среднюю резкость среди блоков. Это гораздо чувствительнее к маленькому локальному тексту
// на спокойном фоне, чем средняя резкость по всей area (которая "размывает" текст в среднем).
func maxBlockDensity(img image.Image, rect image.Rectangle, blockSize int) float64 {
	bounds := img.Bounds()
	r := rect.Intersect(bounds)
	if r.Dx() < blockSize || r.Dy() < blockSize {
		// область меньше блока - просто считаем целиком
		return edgeDensity(img, r)
	}

	maxD := 0.0
	for by := r.Min.Y; by < r.Max.Y; by += blockSize {
		for bx := r.Min.X; bx < r.Max.X; bx += blockSize {
			blockRect := image.Rect(bx, by, minInt(bx+blockSize, r.Max.X), minInt(by+blockSize, r.Max.Y))
			d := edgeDensity(img, blockRect)
			if d > maxD {
				maxD = d
			}
		}
	}
	return maxD
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type cornerInfo struct {
	name string
	rect image.Rectangle
	flag bool
}

// detectCorners проверяет 4 полосы (верх/низ/лево/право). Внутри каждой полосы ищется
// самый "контрастный" блок (maxBlockDensity) - так маленький текст на спокойном фоне не
// "размывается" в среднем. Порог сравнивается с фоном ВНЕ этой полосы (центр фото), что
// надёжнее, чем сравнение со средним по всему кадру.
func detectCorners(img image.Image, sizeFrac float64, sensitivity float64) []cornerInfo {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	stripH := int(float64(h) * sizeFrac)
	stripW := int(float64(w) * sizeFrac)
	if stripH < 10 {
		stripH = 10
	}
	if stripW < 10 {
		stripW = 10
	}

	corners := []cornerInfo{
		{"верх", image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Max.X, bounds.Min.Y+stripH), false},
		{"низ", image.Rect(bounds.Min.X, bounds.Max.Y-stripH, bounds.Max.X, bounds.Max.Y), false},
		{"слева", image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Min.X+stripW, bounds.Max.Y), false},
		{"справа", image.Rect(bounds.Max.X-stripW, bounds.Min.Y, bounds.Max.X, bounds.Max.Y), false},
	}

	// фон для сравнения - центральная область фото (без полос), чтобы не зависеть от
	// общей "шумности" всего кадра, а сравнивать именно с тем, что считается "чистым" фоном
	centerRect := image.Rect(
		bounds.Min.X+stripW, bounds.Min.Y+stripH,
		bounds.Max.X-stripW, bounds.Max.Y-stripH,
	)
	if centerRect.Dx() < 10 || centerRect.Dy() < 10 {
		centerRect = bounds
	}
	blockSize := 24
	backgroundLevel := edgeDensity(img, centerRect)

	threshold := backgroundLevel*sensitivity + 2.0 // +2.0 страхует от деления на почти ноль на гладком фоне

	for i := range corners {
		d := maxBlockDensity(img, corners[i].rect, blockSize)
		if d > threshold {
			corners[i].flag = true
		}
	}
	return corners
}

// mirrorFillRect заполняет полосу (rect), отражая зеркально соседний "чистый" фон,
// примыкающий к внутренней границе этой полосы. direction указывает, с какой стороны
// фото расположена полоса ("верх", "низ", "слева", "справа") - это определяет, какая
// граница считается "внутренней" (откуда берём отражение).
func mirrorFillRect(img *image.RGBA, rect image.Rectangle, fullBounds image.Rectangle, direction string) {
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			sx, sy := x, y
			switch direction {
			case "низ":
				// внутренняя граница - верхний край полосы (rect.Min.Y)
				sy = 2*rect.Min.Y - y - 1
			case "верх":
				// внутренняя граница - нижний край полосы (rect.Max.Y)
				sy = 2*rect.Max.Y - y - 1
			case "слева":
				sx = 2*rect.Max.X - x - 1
			case "справа":
				sx = 2*rect.Min.X - x - 1
			}
			sx = clampInt(sx, fullBounds.Min.X, fullBounds.Max.X-1)
			sy = clampInt(sy, fullBounds.Min.Y, fullBounds.Max.Y-1)
			img.Set(x, y, img.At(sx, sy))
		}
	}
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// blurRect делает блочное размытие (box blur) внутри rect, но сэмплирует пиксели из СНИМКА,
// расширенного на radius за пределы rect (внутри границ всего изображения). Благодаря этому
// на верхней/боковой границе зоны размытие "захватывает" немного чистого фона снаружи —
// переход получается плавным, без резкого шва на границе зоны.
func blurRect(img *image.RGBA, rect image.Rectangle, fullBounds image.Rectangle, radius int) {
	if radius < 1 {
		radius = 1
	}

	snapRect := image.Rect(
		rect.Min.X-radius, rect.Min.Y-radius,
		rect.Max.X+radius, rect.Max.Y+radius,
	).Intersect(fullBounds)

	snap := image.NewRGBA(snapRect)
	for y := snapRect.Min.Y; y < snapRect.Max.Y; y++ {
		for x := snapRect.Min.X; x < snapRect.Max.X; x++ {
			snap.Set(x, y, img.At(x, y))
		}
	}

	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			var rs, gs, bs, as, n int
			for dy := -radius; dy <= radius; dy++ {
				for dx := -radius; dx <= radius; dx++ {
					sx, sy := x+dx, y+dy
					if sx < snapRect.Min.X || sx >= snapRect.Max.X || sy < snapRect.Min.Y || sy >= snapRect.Max.Y {
						continue
					}
					r, g, b, a := snap.At(sx, sy).RGBA()
					rs += int(r >> 8)
					gs += int(g >> 8)
					bs += int(b >> 8)
					as += int(a >> 8)
					n++
				}
			}
			if n > 0 {
				img.Set(x, y, color.RGBA{
					uint8(rs / n), uint8(gs / n), uint8(bs / n), uint8(as / n),
				})
			}
		}
	}
}

// fixedZoneRect строит прямоугольник зоны вручную, по отступам в процентах от размеров фото.
// bottomPct/heightPct задают полосу снизу: heightPct - высота зоны, leftPct/rightPct - отступы
// по бокам (чтобы можно было не трогать всю ширину, если надпись не на всю ширину).
func fixedZoneRect(bounds image.Rectangle, heightPct, leftPct, rightPct float64) image.Rectangle {
	w, h := bounds.Dx(), bounds.Dy()
	zoneH := int(float64(h) * heightPct / 100.0)
	leftCut := int(float64(w) * leftPct / 100.0)
	rightCut := int(float64(w) * rightPct / 100.0)

	return image.Rect(
		bounds.Min.X+leftCut, bounds.Max.Y-zoneH,
		bounds.Max.X-rightCut, bounds.Max.Y,
	)
}

// cropZoneOut физически вырезает зону (например, полосу снизу с надписью) из фото, возвращая
// уменьшенное изображение БЕЗ этой части (а не закрашивает её, как removeFixedZone).
func cropZoneOut(src image.Image, heightPct, leftPct, rightPct float64) image.Image {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	cutH := int(float64(h) * heightPct / 100.0)
	leftCut := int(float64(w) * leftPct / 100.0)
	rightCut := int(float64(w) * rightPct / 100.0)

	newRect := image.Rect(
		bounds.Min.X+leftCut, bounds.Min.Y,
		bounds.Max.X-rightCut, bounds.Max.Y-cutH,
	)
	if newRect.Dx() < 1 || newRect.Dy() < 1 {
		return src // на всякий случай - если настройки совсем абсурдные, ничего не обрезаем
	}

	out := image.NewRGBA(image.Rect(0, 0, newRect.Dx(), newRect.Dy()))
	for y := newRect.Min.Y; y < newRect.Max.Y; y++ {
		for x := newRect.Min.X; x < newRect.Max.X; x++ {
			out.Set(x-newRect.Min.X, y-newRect.Min.Y, src.At(x, y))
		}
	}
	return out
}

// removeFixedZone применяет выбранный метод удаления к ОДНОЙ заданной вручную зоне (без автоопределения).
func removeFixedZone(src image.Image, zone image.Rectangle, method string, blurStrength int) *image.RGBA {
	bounds := src.Bounds()
	rgba := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rgba.Set(x, y, src.At(x, y))
		}
	}
	zone = zone.Intersect(bounds)
	if zone.Empty() {
		return rgba
	}
	if method == "blur" {
		blurRect(rgba, zone, bounds, blurStrength)
	} else {
		mirrorFillRect(rgba, zone, bounds, "низ")
	}
	return rgba
}

// drawZoneOutline рисует красную рамку вокруг заданной зоны для предпросмотра.
func drawZoneOutline(src image.Image, zone image.Rectangle) *image.RGBA {
	bounds := src.Bounds()
	rgba := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rgba.Set(x, y, src.At(x, y))
		}
	}
	zone = zone.Intersect(bounds)
	if !zone.Empty() {
		drawRectOutline(rgba, zone, color.RGBA{255, 40, 40, 255}, 4)
	}
	return rgba
}
func removeWatermarks(src image.Image, cornerFrac, sensitivity float64, method string) (*image.RGBA, []cornerInfo) {
	bounds := src.Bounds()
	rgba := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rgba.Set(x, y, src.At(x, y))
		}
	}

	corners := detectCorners(src, cornerFrac, sensitivity)
	for _, c := range corners {
		if !c.flag {
			continue
		}
		if method == "blur" {
			blurRect(rgba, c.rect, bounds, 6)
		} else {
			mirrorFillRect(rgba, c.rect, bounds, c.name)
		}
	}
	return rgba, corners
}

// drawCornerOutlines рисует красные рамки вокруг углов для предпросмотра (флаг = найдено)
func drawCornerOutlines(src image.Image, corners []cornerInfo) *image.RGBA {
	bounds := src.Bounds()
	rgba := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rgba.Set(x, y, src.At(x, y))
		}
	}
	red := color.RGBA{255, 40, 40, 255}
	for _, c := range corners {
		if !c.flag {
			continue
		}
		drawRectOutline(rgba, c.rect, red, 4)
	}
	return rgba
}

func drawRectOutline(img *image.RGBA, rect image.Rectangle, col color.Color, thickness int) {
	for t := 0; t < thickness; t++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			img.Set(x, rect.Min.Y+t, col)
			img.Set(x, rect.Max.Y-1-t, col)
		}
		for y := rect.Min.Y; y < rect.Max.Y; y++ {
			img.Set(rect.Min.X+t, y, col)
			img.Set(rect.Max.X-1-t, y, col)
		}
	}
}
