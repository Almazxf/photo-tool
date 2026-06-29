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
	// для крупных изображений можно прыгать через пиксель для скорости
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

type cornerInfo struct {
	name string
	rect image.Rectangle
	flag bool
}

// detectCorners возвращает 4 угла с пометкой, похоже ли там на надпись
func detectCorners(img image.Image, cornerFrac float64, sensitivity float64) []cornerInfo {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	cw := int(float64(w) * cornerFrac)
	ch := int(float64(h) * cornerFrac)
	if cw < 10 {
		cw = 10
	}
	if ch < 10 {
		ch = 10
	}

	corners := []cornerInfo{
		{"top-left", image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Min.X+cw, bounds.Min.Y+ch), false},
		{"top-right", image.Rect(bounds.Max.X-cw, bounds.Min.Y, bounds.Max.X, bounds.Min.Y+ch), false},
		{"bottom-left", image.Rect(bounds.Min.X, bounds.Max.Y-ch, bounds.Min.X+cw, bounds.Max.Y), false},
		{"bottom-right", image.Rect(bounds.Max.X-cw, bounds.Max.Y-ch, bounds.Max.X, bounds.Max.Y), false},
	}

	// средняя резкость по всему изображению (выборочно, для скорости)
	globalAvg := edgeDensity(img, bounds)

	threshold := globalAvg * sensitivity
	minFloor := 8.0 // не реагировать на совсем плоские/тёмные фото

	for i := range corners {
		d := edgeDensity(img, corners[i].rect)
		if d > threshold && d > minFloor {
			corners[i].flag = true
		}
	}
	return corners
}

// mirrorFillRect заполняет область rect, отражая соседние пиксели снаружи rect (по короткой стороне к центру изображения)
func mirrorFillRect(img *image.RGBA, rect image.Rectangle, fullBounds image.Rectangle) {
	cx := (fullBounds.Min.X + fullBounds.Max.X) / 2
	cy := (fullBounds.Min.Y + fullBounds.Max.Y) / 2

	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			// отражаем точку относительно границы прямоугольника в сторону центра изображения
			var sx, sy int
			if x < cx {
				sx = rect.Max.X + (rect.Max.X - x) // отражение вправо от правого края прямоугольника
			} else {
				sx = rect.Min.X - (x - rect.Min.X) // отражение влево от левого края
			}
			if y < cy {
				sy = rect.Max.Y + (rect.Max.Y - y)
			} else {
				sy = rect.Min.Y - (y - rect.Min.Y)
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

// blurRect делает простое блочное размытие (box blur) внутри rect
func blurRect(img *image.RGBA, rect image.Rectangle, radius int) {
	if radius < 1 {
		radius = 1
	}
	src := image.NewRGBA(rect)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			src.Set(x, y, img.At(x, y))
		}
	}

	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			var rs, gs, bs, as, n int
			for dy := -radius; dy <= radius; dy++ {
				for dx := -radius; dx <= radius; dx++ {
					sx, sy := x+dx, y+dy
					if sx < rect.Min.X || sx >= rect.Max.X || sy < rect.Min.Y || sy >= rect.Max.Y {
						continue
					}
					r, g, b, a := src.At(sx, sy).RGBA()
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

// removeWatermarks применяет автоопределение + удаление к копии изображения, возвращает результат и список найденных углов
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
			blurRect(rgba, c.rect, 6)
		} else {
			mirrorFillRect(rgba, c.rect, bounds)
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
