//go:build !windows

// Dev preview: simulates the buddy and writes PNG frames (used to test the
// renderer on non-Windows machines). The real app is main_windows.go.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
)

func main() {
	const S = 2
	b := NewBuddy(S, []Area{{0, 0, 1920, 1040, true}}, -1, true)
	c := &Canvas{W: canvasW * S, H: canvasH * S}
	c.Pix = make([]byte, c.W*c.H*4)
	out := "."
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	if len(os.Args) > 2 && os.Args[2] == "sheet" {
		sheet(out)
		return
	}
	shot := func(name string) {
		b.Draw(c)
		img := image.NewRGBA(image.Rect(0, 0, c.W, c.H))
		for i := 0; i < c.W*c.H; i++ {
			bg := uint32(0x30)
			if (i%c.W/16+i/c.W/16)&1 == 0 {
				bg = 0x48
			}
			a := uint32(c.Pix[i*4+3])
			mix := func(v byte) byte { return byte(uint32(v) + bg*(255-a)/255) }
			img.Pix[i*4], img.Pix[i*4+1], img.Pix[i*4+2], img.Pix[i*4+3] = mix(c.Pix[i*4+2]), mix(c.Pix[i*4+1]), mix(c.Pix[i*4]), 255
		}
		_ = color.RGBA{}
		f, _ := os.Create(out + "/" + name + ".png")
		png.Encode(f, img)
		f.Close()
		x, y := b.WinPos()
		fmt.Println(name, "state", b.State, "win", x, y)
	}
	run := func(sec, cx, cy, idle float64) {
		for t := 0.0; t < sec; t += 1.0 / 60 {
			b.Update(1.0/60, cx, cy, idle)
		}
	}
	run(2.0, 900, 300, 0)
	shot("1_greeting")
	run(6, 900, 300, 0)
	b.Down(b.X, b.Y-80)
	b.Up()
	run(0.25, 1900, 100, 0)
	shot("2_pet")
	run(4, 900, 900, 0)
	b.Down(b.X, b.Y-80)
	for i := 0; i < 20; i++ {
		b.Update(1.0/60, b.X-b.offX-25, b.Y-b.offY-18, 0)
	}
	b.Up()
	run(0.5, 300, 300, 0)
	shot("3_thrown")
	run(1.5, 300, 300, 0)
	shot("4_landed_left")
	run(5, 300, 300, 0)
	shot("5_walking_home")
	run(20, 300, 300, 0)
	run(0.25, 300, 300, 200)
	shot("6a_tucking_in")
	run(6, 300, 300, 200)
	shot("6_sleeping")
	run(0.6, 300, 300, 0)
	shot("7_wake")
}

// sheet renders every activity at three moments into one contact sheet.
func sheet(out string) {
	const S, cols = 1, 6
	cw, ch := canvasW*S, canvasH*S
	rows := (len(acts) + cols - 1) / cols
	shotsPer := []float64{0.3, 0.5, 0.8}
	for si, frac := range shotsPer {
		img := image.NewRGBA(image.Rect(0, 0, cols*cw, rows*ch))
		for i, a := range acts {
			b := NewBuddy(S, []Area{{0, 0, 1920, 1040, true}}, -1, true)
			b.nextAct = 1e9
			for t := 0.0; t < 3; t += 1.0 / 60 {
				b.Update(1.0/60, 900, 500, 0)
			}
			b.StartActivity(i)
			for t := 0.0; t < a.Dur*frac; t += 1.0 / 60 {
				b.Update(1.0/60, 900, 500, 0)
			}
			c := &Canvas{W: cw, H: ch}
			c.Pix = make([]byte, cw*ch*4)
			b.Draw(c)
			c.text(2, 2, 1, strings.ToUpper(a.Name), 0xFFFFFFFF)
			ox, oy := (i%cols)*cw, (i/cols)*ch
			for y := 0; y < ch; y++ {
				for x := 0; x < cw; x++ {
					k := (y*cw + x) * 4
					al := uint32(c.Pix[k+3])
					bg := uint32(0x40)
					if ((x/8)+(y/8))&1 == 0 {
						bg = 0x50
					}
					m := func(v byte) uint8 { return uint8(uint32(v) + bg*(255-al)/255) }
					img.SetRGBA(ox+x, oy+y, color.RGBA{m(c.Pix[k+2]), m(c.Pix[k+1]), m(c.Pix[k]), 255})
				}
			}
		}
		f, _ := os.Create(fmt.Sprintf("%s/sheet_%d.png", out, si))
		png.Encode(f, img)
		f.Close()
	}
}
