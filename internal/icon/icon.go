// Package icon draws the app / tray icon (a headphone glyph on a rounded
// gradient tile) so no binary image assets are needed.
package icon

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

type Palette struct{ Top, Bottom color.RGBA }

var (
	Connected = Palette{color.RGBA{0x3B, 0x82, 0xF6, 255}, color.RGBA{0x14, 0xB8, 0xA6, 255}}
	Idle      = Palette{color.RGBA{0x64, 0x6B, 0x7A, 255}, color.RGBA{0x3F, 0x45, 0x52, 255}}
	Brand     = Palette{color.RGBA{0x63, 0x66, 0xF1, 255}, color.RGBA{0x06, 0xB6, 0xD4, 255}}
)

func roundedRect(x, y, cx, cy, hw, hh, r float64) float64 {
	dx := math.Abs(x-cx) - hw + r
	dy := math.Abs(y-cy) - hh + r
	ox, oy := math.Max(dx, 0), math.Max(dy, 0)
	return math.Hypot(ox, oy) + math.Min(math.Max(dx, dy), 0) - r
}

// glyph returns the signed distance to the headphone shape in unit space.
func glyph(x, y float64) float64 {
	const cx, cy, R, t = 0.5, 0.56, 0.255, 0.075
	band := math.Abs(math.Hypot(x-cx, y-cy)-R) - t/2
	if y > cy {
		band = math.Inf(1)
	}
	left := roundedRect(x, y, cx-R, 0.66, 0.075, 0.13, 0.06)
	right := roundedRect(x, y, cx+R, 0.66, 0.075, 0.13, 0.06)
	return math.Min(band, math.Min(left, right))
}

func mix(a, b uint8, t float64) uint8 { return uint8(float64(a) + (float64(b)-float64(a))*t + 0.5) }

// Render returns a PNG of the icon at the given pixel size.
func Render(size int, p Palette) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	const ss = 4
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var bgCov, fgCov float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					x := (float64(px) + (float64(sx)+0.5)/ss) / float64(size)
					y := (float64(py) + (float64(sy)+0.5)/ss) / float64(size)
					if roundedRect(x, y, 0.5, 0.5, 0.5, 0.5, 0.23) <= 0 {
						bgCov++
						if glyph(x, y) <= 0 {
							fgCov++
						}
					}
				}
			}
			if bgCov == 0 {
				continue
			}
			bgCov /= ss * ss
			fg := fgCov / (ss * ss) / bgCov
			t := float64(py) / float64(size-1)
			r := mix(mix(p.Top.R, p.Bottom.R, t), 255, fg)
			g := mix(mix(p.Top.G, p.Bottom.G, t), 255, fg)
			b := mix(mix(p.Top.B, p.Bottom.B, t), 255, fg)
			img.SetNRGBA(px, py, color.NRGBA{r, g, b, uint8(bgCov*255 + 0.5)})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
