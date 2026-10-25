// Command wallpaper draws the h2o wallpaper: a flat night landscape with a
// river, in the h2o palette. To make one for another theme, change the
// colours below and run it:
//
//	go run . -o wallpaper.png -w 3840 -h 2400
//
// It draws at twice the size and scales down, so edges are smooth.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

// The palette (../home/.config/theme/colors.sh) and shades between its
// colours.
var (
	text   = hex("e6f1fa")
	dim    = hex("8eabc4")
	accent = hex("3fa9f5")
	foam   = hex("8fe3f0")

	skyTop     = hex("08131f")
	skyHorizon = hex("1f4a6e")
	halo       = hex("2d5d86")
	haloInner  = hex("3d7099")
	farHills   = hex("1b3f5e")
	midHills   = hex("12304a")
	ground     = hex("0d2135")
	water      = hex("1c5f94")
	waterLight = hex("3fa9f5")
	nearHills  = hex("0a1724")
	trees      = hex("07111b")
)

// horizon is how far down the picture the land starts, as a fraction.
const horizon = 0.585

func main() {
	out := flag.String("o", "wallpaper.png", "output file")
	w := flag.Int("w", 3840, "width")
	h := flag.Int("h", 2400, "height")
	flag.Parse()

	c := newCanvas(*w*2, *h*2)
	m := moon{x: 0.70 * c.fw, y: 0.22 * c.fh, r: 0.052 * c.fh}
	drawSky(c)
	c.blendDisc(m.x, m.y, m.r*2.4, halo, 0.18)
	c.blendDisc(m.x, m.y, m.r*1.6, haloInner, 0.22)
	drawStars(c, m)
	c.disc(m.x, m.y, m.r, mix(foam, text, 0.55))
	drawHills(c)
	drawRiver(c, m)
	drawBanks(c)

	if err := write(*out, c.halve()); err != nil {
		fmt.Fprintln(os.Stderr, "wallpaper:", err)
		os.Exit(1)
	}
}

type moon struct{ x, y, r float64 }

// drawSky fills the sky with flat bands, from near black to a blue glow at
// the horizon.
func drawSky(c *canvas) {
	const bands = 8
	for y := 0; y < c.y(horizon); y++ {
		t := float64(y) / (c.fh * horizon)
		band := math.Floor(math.Pow(t, 1.6) * bands)
		col := mix(skyTop, skyHorizon, band/(bands-1))
		for x := 0; x < c.w; x++ {
			c.set(x, y, col)
		}
	}
}

// drawStars scatters stars over the sky, dimmer towards the horizon and none
// in the moon's halo.
func drawStars(c *canvas, m moon) {
	cell := 0.03 * c.fh
	for cy := 0; float64(cy)*cell < c.fh*horizon*0.9; cy++ {
		for cx := 0; float64(cx)*cell < c.fw; cx++ {
			if hash(cx, cy, 1) > 0.42 {
				continue
			}
			x := (float64(cx) + hash(cx, cy, 2)) * cell
			y := (float64(cy) + hash(cx, cy, 3)) * cell
			if math.Hypot(x-m.x, y-m.y) < m.r*2.6 {
				continue
			}
			r := c.fh * (0.0009 + 0.0013*hash(cx, cy, 4))
			col := dim
			if hash(cx, cy, 5) < 0.3 {
				col = text
			}
			col = mix(col, c.at(int(x), int(y)), y/(c.fh*horizon)*0.8)
			c.disc(x, y, r, col)
		}
	}
}

// drawHills draws two ridges along the horizon, then the ground below it.
func drawHills(c *canvas) {
	far := []float64{0.035, 5.3, 0.4, 0.022, 11.7, 1.9, 0.010, 27, 0.3}
	mid := []float64{0.018, 7.9, 2.2, 0.010, 19, 0.8, 0.004, 41, 1.1}
	c.fillDown(func(u float64) float64 { return horizon - 0.075 - ridge(u, far) }, horizon, farHills)
	c.fillDown(func(u float64) float64 { return horizon - 0.028 - ridge(u, mid) }, horizon, midHills)
	c.fillDown(func(float64) float64 { return horizon }, 1, ground)
}

// ridge sums sines given as amplitude, frequency, phase triples.
func ridge(u float64, f []float64) float64 {
	s := 0.0
	for i := 0; i+2 < len(f); i += 3 {
		s += f[i] * math.Sin(u*f[i+1]+f[i+2])
	}
	return s
}

// drawRiver draws the river winding down from the horizon and widening, with
// ripples and the moon's reflection.
func drawRiver(c *canvas, m moon) {
	moonU := m.x / c.fw
	for y := c.y(horizon); y < c.h; y++ {
		s := (float64(y)/c.fh - horizon) / (1 - horizon)
		cx := 0.5 + 0.17*math.Sin(s*5.2+0.9)*(0.25+0.75*s) + 0.06*s
		hw := 0.0025 + 0.23*math.Pow(s, 1.75)
		l, r := int((cx-hw)*c.fw), int((cx+hw)*c.fw)
		// Ripples: thin light lines, longer and farther apart up close.
		gap := 6 + s*90
		stripe := math.Mod(float64(y), gap) < 1.5+s*5
		for x := l; x <= r; x++ {
			col := water
			u := float64(x) / c.fw
			if stripe && hash(int(u*60/(0.2+s)), y/int(gap+1), 7) < 0.55 {
				col = waterLight
			}
			if math.Abs(u-moonU) < 0.012+0.035*s && stripe && hash(int(u*200), y, 9) < 0.8 {
				col = mix(foam, text, 0.3)
			}
			c.set(x, y, col)
		}
		// A bright edge on the far bank.
		for x := r - int(0.0015*c.fw); x <= r; x++ {
			c.set(x, y, mix(water, accent, 0.5))
		}
	}
}

// drawBanks draws the near hills in the bottom corners, over the river's
// edges, with pines on them.
func drawBanks(c *canvas) {
	left := func(u float64) float64 { return 0.70 + 0.55*math.Pow(u/0.42, 1.8) - 0.012*math.Sin(u*40) }
	right := func(u float64) float64 { return 0.76 + 0.6*math.Pow((1-u)/0.36, 2.0) - 0.01*math.Sin(u*33+1) }
	c.fillDown(left, 1, nearHills)
	c.fillDown(right, 1, nearHills)
	for i, u := range []float64{0.02, 0.06, 0.09, 0.14, 0.19, 0.25} {
		h := (0.16 - 0.018*float64(i)) * c.fh * (0.85 + 0.3*hash(i, 1, 11))
		drawPine(c, u*c.fw, left(u)*c.fh+0.01*c.fh, h)
	}
	for i, u := range []float64{0.985, 0.95, 0.91, 0.87, 0.82} {
		h := (0.13 - 0.016*float64(i)) * c.fh * (0.85 + 0.3*hash(i, 2, 13))
		drawPine(c, u*c.fw, right(u)*c.fh+0.01*c.fh, h)
	}
}

// drawPine draws a pine of height h standing at bx, by: four stacked
// triangles, each narrower than the one below, on a short trunk.
func drawPine(c *canvas, bx, by, h float64) {
	for i := range 4 {
		bot := by - h*0.14 - float64(i)*h*0.19
		apex := bot - h*0.36
		half := h * 0.27 * (1 - 0.2*float64(i))
		for y := int(apex); y <= int(bot); y++ {
			k := (float64(y) - apex) / (bot - apex)
			for x := int(bx - half*k); x <= int(bx+half*k); x++ {
				c.set(x, y, trees)
			}
		}
	}
	for y := int(by - h*0.16); y <= int(by); y++ {
		for x := int(bx - h*0.025); x <= int(bx+h*0.025); x++ {
			c.set(x, y, trees)
		}
	}
}

// canvas is the picture being drawn.
type canvas struct {
	img    *image.RGBA
	w, h   int
	fw, fh float64
}

func newCanvas(w, h int) *canvas {
	return &canvas{img: image.NewRGBA(image.Rect(0, 0, w, h)), w: w, h: h, fw: float64(w), fh: float64(h)}
}

func (c *canvas) set(x, y int, col color.RGBA) {
	if x >= 0 && y >= 0 && x < c.w && y < c.h {
		c.img.SetRGBA(x, y, col)
	}
}

func (c *canvas) at(x, y int) color.RGBA { return c.img.RGBAAt(x, y) }

// y turns a fraction of the height into a row.
func (c *canvas) y(v float64) int { return int(v * c.fh) }

func (c *canvas) disc(cx, cy, r float64, col color.RGBA) {
	c.eachInDisc(cx, cy, r, func(x, y int) { c.set(x, y, col) })
}

// blendDisc mixes col into a disc, t of the way.
func (c *canvas) blendDisc(cx, cy, r float64, col color.RGBA, t float64) {
	c.eachInDisc(cx, cy, r, func(x, y int) { c.set(x, y, mix(c.at(x, y), col, t)) })
}

func (c *canvas) eachInDisc(cx, cy, r float64, f func(x, y int)) {
	for y := int(cy - r); y <= int(cy+r); y++ {
		for x := int(cx - r); x <= int(cx+r); x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			if dx*dx+dy*dy <= r*r && y >= 0 && y < c.h && x >= 0 && x < c.w {
				f(x, y)
			}
		}
	}
}

// fillDown fills each column from top(u), a fraction of the height for the
// column's fraction u of the width, down to bottom.
func (c *canvas) fillDown(top func(u float64) float64, bottom float64, col color.RGBA) {
	for x := 0; x < c.w; x++ {
		for y := int(top(float64(x)/c.fw) * c.fh); y < c.y(bottom); y++ {
			c.set(x, y, col)
		}
	}
}

// halve scales the picture down to half its size, averaging each 2x2 block.
func (c *canvas) halve() *image.RGBA {
	small := image.NewRGBA(image.Rect(0, 0, c.w/2, c.h/2))
	for y := 0; y < c.h/2; y++ {
		for x := 0; x < c.w/2; x++ {
			var r, g, b int
			for _, p := range [][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
				px := c.at(x*2+p[0], y*2+p[1])
				r, g, b = r+int(px.R), g+int(px.G), b+int(px.B)
			}
			small.SetRGBA(x, y, color.RGBA{uint8(r / 4), uint8(g / 4), uint8(b / 4), 255})
		}
	}
	return small
}

func hex(s string) color.RGBA {
	var c color.RGBA
	c.A = 255
	for i, p := range []*uint8{&c.R, &c.G, &c.B} {
		var v uint8
		for _, ch := range s[i*2 : i*2+2] {
			v <<= 4
			switch {
			case ch >= '0' && ch <= '9':
				v |= uint8(ch - '0')
			case ch >= 'a' && ch <= 'f':
				v |= uint8(ch-'a') + 10
			}
		}
		*p = v
	}
	return c
}

func mix(a, b color.RGBA, t float64) color.RGBA {
	t = math.Max(0, math.Min(1, t))
	l := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t + 0.5) }
	return color.RGBA{l(a.R, b.R), l(a.G, b.G), l(a.B, b.B), 255}
}

// hash gives a repeatable pseudo-random number in [0, 1) for a cell.
func hash(a, b int, salt uint32) float64 {
	h := uint32(a)*374761393 + uint32(b)*668265263 + salt*2246822519
	h = (h ^ (h >> 13)) * 1274126177
	h ^= h >> 16
	return float64(h) / 4294967296
}

func write(path string, m image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, m); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
