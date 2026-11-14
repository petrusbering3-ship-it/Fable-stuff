// Fable Buddy - activities: little animated scenes with props.
//
// Coordinates for props are in "u" units (1u = 2 screen pixels per UI
// scale step), relative to the buddy's feet. +x points towards the middle of
// the screen (props are mirrored automatically), -y is up. The body spans
// x -16..16, y -28..0; the eyes sit around y -17 and the mouth at (0,-12).
package main

import (
	"math"
	"math/rand"
)

type Act struct {
	Name  string
	Dur   float64
	Say   []string
	Pose  func(t float64, p *pose)
	Back  func(d *dc, t float64) // drawn behind the body
	Front func(d *dc, t float64) // drawn in front of the body
	Emit  func(b *Buddy, t, dt float64)
}

const (
	eAuto = iota
	eClosed
	eHappy
	eLook
	eWide
	eDizzy
	eWink
)

const (
	mAuto = iota
	mO
	mBig
	mSmile
	mFlat
	mChew
	mHide
)

type pose struct {
	ox, oy, sq     float64 // body offset (u) and extra squash
	eyes, mouth    int
	lx, ly         float64 // where eLook/eWide eyes point (-1..1, +x inward)
	arms           [][2]float64
	noShadow, walk bool
}

// StartActivity begins activity i, or a random one (not the last) if i < 0.
func (b *Buddy) StartActivity(i int) {
	if b.State == Drag || b.State == Air {
		return
	}
	if i < 0 || i >= len(acts) {
		i = rand.Intn(len(acts))
		if i == b.lastAct {
			i = (i + 1) % len(acts)
		}
	}
	b.State, b.act, b.actT, b.lastAct, b.idleT = Busy, &acts[i], 0, i, 0
	if len(acts[i].Say) > 0 {
		b.Say(pick(acts[i].Say))
	}
}

// emit spawns a particle in prop coordinates (u, u/s, u/s²).
func (b *Buddy) emit(kind int, x, y, vx, vy, life float64, col uint32, g float64) {
	_, _, right := b.anchor()
	dir := 1.0
	if right {
		dir = -1
	}
	u := 2 * float64(b.S)
	b.parts = append(b.parts, particle{kind: kind, x: x * dir * u, y: y * u, vx: vx * dir * u, vy: vy * u,
		life: life, max: life, col: col, g: g * u})
}

func (b *Buddy) burst(kind int, x, y float64, n int, speed float64, cols ...uint32) {
	for i := 0; i < n; i++ {
		a := rand.Float64() * 2 * math.Pi
		v := speed * (0.4 + rand.Float64()*0.6)
		b.emit(kind, x, y, math.Cos(a)*v, math.Sin(a)*v-speed*0.5, 1+rand.Float64(), cols[rand.Intn(len(cols))], 60)
	}
}

// ---- tiny helpers ----

func at(t, dt, m float64) bool    { return t-dt < m && t >= m }
func every(t, dt, p float64) bool { return math.Floor(t/p) != math.Floor((t-dt)/p) }
func clamp01(x float64) float64   { return math.Max(0, math.Min(1, x)) }
func lerp(a, b, f float64) float64 {
	return a + (b-a)*f
}
func ease(f float64) float64 { f = clamp01(f); return f * f * (3 - 2*f) }
func seg(t, t0, t1 float64) float64 {
	return clamp01((t - t0) / (t1 - t0))
}
func rnd(lo, hi float64) float64 { return lo + rand.Float64()*(hi-lo) }

const (
	kW  = 0xFFFFFFFF
	kG  = 0xFFB8B8C8
	kGD = 0xFF6B6B7B
	kX  = 0xFF1E1E28
	kR  = 0xFFE84855
	kRD = 0xFFA82835
	kO  = 0xFFF59E3B
	kY  = 0xFFFFD166
	kYD = 0xFFE0A030
	kB  = 0xFF4F86E8
	kBD = 0xFF2F4FA8
	kC  = 0xFF7FD8F0
	kP  = 0xFFFF8FB8
	kPU = 0xFF9B5DE5
	kN  = 0xFFA0673C
	kND = 0xFF6B4226
	kE  = 0xFF5CC46A
	kED = 0xFF2E8B45
	kM  = 0xFFFFF1D6
)

var party = []uint32{kR, kY, kB, kE, kP, kPU, kO, kC}

var pal = map[rune]uint32{
	'k': cOutline, 'w': kW, 'g': kG, 'G': kGD, 'x': kX, 'r': kR, 'R': kRD, 'o': kO, 'y': kY,
	'Y': kYD, 'b': kB, 'B': kBD, 'c': kC, 'p': kP, 'P': kPU, 'n': kN, 'N': kND, 'e': kE,
	'E': kED, 'm': kM, 'a': cBase,
}

// dc draws props in u coordinates, mirrored so +x points inward.
type dc struct {
	c              *Canvas
	u, ax, ay, dir float64
}

func (d *dc) px(x float64) float64 { return d.ax + d.dir*x*d.u }

func (d *dc) rect(x, y, w, h float64, col uint32) {
	x0, x1 := d.px(x), d.px(x+w)
	if x1 < x0 {
		x0, x1 = x1, x0
	}
	y0, y1 := d.ay+y*d.u, d.ay+(y+h)*d.u
	if y1 < y0 {
		y0, y1 = y1, y0
	}
	ix, iy := int(math.Round(x0)), int(math.Round(y0))
	d.c.Rect(ix, iy, int(math.Round(x1))-ix, int(math.Round(y1))-iy, col)
}

// rectClip draws a rect limited to rows lo..hi.
func (d *dc) rectClip(x, y, w, h float64, col uint32, lo, hi float64) {
	y0, y1 := math.Max(y, lo), math.Min(y+h, hi)
	if y1 > y0 {
		d.rect(x, y0, w, y1-y0, col)
	}
}

func (d *dc) box(x, y, w, h float64, col uint32) {
	d.rect(x-1, y-1, w+2, h+2, cOutline)
	d.rect(x, y, w, h, col)
}

func (d *dc) line(x0, y0, x1, y1, w float64, col uint32) {
	n := int(math.Max(math.Abs(x1-x0), math.Abs(y1-y0))*2) + 1
	for i := 0; i <= n; i++ {
		f := float64(i) / float64(n)
		d.rect(lerp(x0, x1, f)-w/2, lerp(y0, y1, f)-w/2, w, w, col)
	}
}

func (d *dc) disc(cx, cy, r float64, col uint32) {
	for yy := -r; yy < r; yy += 0.5 {
		m := yy + 0.25
		hw := math.Sqrt(math.Max(0, r*r-m*m))
		d.rect(cx-hw, cy+yy, 2*hw, 0.5, col)
	}
}

func (d *dc) ball(cx, cy, r float64, col uint32) {
	d.disc(cx, cy, r+1, cOutline)
	d.disc(cx, cy, r, col)
}

// arc draws part of an ellipse outline from angle a0 to a1 (radians, +y down).
func (d *dc) arc(cx, cy, rx, ry, a0, a1, w float64, col uint32) {
	n := int(math.Max(rx, ry)*math.Abs(a1-a0)) + 2
	for i := 0; i <= n; i++ {
		a := lerp(a0, a1, float64(i)/float64(n))
		d.rect(cx+rx*math.Cos(a)-w/2, cy+ry*math.Sin(a)-w/2, w, w, col)
	}
}

func (d *dc) spr(x, y float64, rows []string) { d.sprCut(x, y, rows, 0) }

// sprCut draws a sprite, skipping its first `cut` columns.
func (d *dc) sprCut(x, y float64, rows []string, cut int) {
	for r, row := range rows {
		for k, ch := range row {
			if col, ok := pal[ch]; ok && k >= cut {
				d.rect(x+float64(k), y+float64(r), 1, 1, col)
			}
		}
	}
}

func (d *dc) arm(x, y float64) {
	d.rect(x-0.5, y-0.5, 3, 3, cOutline)
	d.rect(x, y, 2, 2, cBase)
}

func (d *dc) label(x, y float64, s string, col uint32) {
	w := float64(textW(s)) * 0.5
	l := math.Min(d.px(x), d.px(x+w))
	d.c.text(int(l), int(d.ay+y*d.u), int(d.u/2), s, col)
}

func star(d *dc, x, y float64, col uint32) {
	d.rect(x, y-1, 1, 3, col)
	d.rect(x-1, y, 3, 1, col)
}

// ---- roller coaster track ----

var coasterPts = [][2]float64{{-24, 0}, {4, 0}, {26, -44}, {50, 0}, {63, -16}, {76, 0}, {100, 0}}

func coasterH(x float64) float64 {
	p := coasterPts
	for i := 0; i < len(p)-1; i++ {
		if x <= p[i+1][0] {
			f := clamp01((x - p[i][0]) / (p[i+1][0] - p[i][0]))
			return lerp(p[i][1], p[i+1][1], (1-math.Cos(f*math.Pi))/2)
		}
	}
	return 0
}

func coasterX(t float64) float64 {
	switch {
	case t < 0.8:
		return 0
	case t < 3.8:
		return 26 * ease(seg(t, 0.8, 3.8))
	case t < 5.0:
		f := seg(t, 3.8, 5.0)
		return 26 + 48*f*f
	case t < 5.5:
		return 74
	case t < 8.2:
		return 74 * (1 - ease(seg(t, 5.5, 8.2)))
	}
	return 0
}

// ---- other shared motion ----

func ropeA(t float64) float64 { return t * 7 }
func ropeOY(t float64) float64 {
	return -7 * clamp01((-math.Cos(ropeA(t))-0.3)/0.7)
}

func skateX(t float64) float64 { return 24 * math.Sin(math.Pi*seg(t, 0.5, 8.5)) }
func skateHop(t float64) float64 {
	return -10 * math.Sin(math.Pi*seg(t, 4, 4.8))
}

func bikeX(t float64) float64 { return 26 * math.Sin(math.Pi*seg(t, 0.5, 7.5)) }

func hicOY(t float64) float64 {
	oy := 0.0
	for _, h := range []float64{1, 2.6, 3.4, 5.2, 6.4, 8} {
		if t >= h && t < h+0.35 {
			oy = -7 * math.Sin(math.Pi*(t-h)/0.35)
		}
	}
	return oy
}

func balloonOY(t float64) float64 { return -10 * math.Sin(math.Pi*seg(t, 4, 7)) }

func surfOY(t float64) float64 { return -4 + 1.5*math.Sin(t*3) }

func sipLift(t float64) float64 {
	m := math.Mod(t, 3)
	return 4 * math.Sin(math.Pi*seg(m, 1.5, 2.5))
}

func gumR(t float64) float64 {
	switch {
	case t < 2.5:
		return 5 * seg(t, 0.3, 2.5)
	case t < 3:
		return 0
	case t < 6:
		return 7 * seg(t, 3.2, 6)
	case t < 6.5:
		return 0
	case t < 9:
		return 11 * seg(t, 6.6, 9)
	}
	return 0
}

func noteCol() uint32 { return party[rand.Intn(len(party))] }

// ---- the activities ----

var acts = []Act{
	{
		Name: "Ride a roller coaster", Dur: 9, Say: []string{"ROLLER COASTER TIME!", "KEEP YOUR ARMS INSIDE THE RIDE."},
		Pose: func(t float64, p *pose) {
			x := coasterX(t)
			p.ox, p.oy, p.noShadow = x, coasterH(x)-5, true
			switch {
			case t < 0.8:
				p.eyes, p.mouth = eHappy, mSmile
			case t < 3.8:
				p.eyes, p.lx, p.ly, p.mouth = eLook, 1, -1, mO
			case t < 5.5:
				p.eyes, p.mouth, p.sq = eWide, mBig, 0.04*math.Sin(t*40)
				p.arms = [][2]float64{{-15, -36}, {13, -36}}
			default:
				p.eyes, p.mouth = eHappy, mSmile
				p.arms = [][2]float64{{-15, -36}, {13, -36}}
			}
		},
		Back: func(d *dc, t float64) {
			for x := -10.0; x <= 90; x += 10 {
				if h := coasterH(x); h < -2 {
					d.rect(x, h+1.5, 1, -h-1.5, kG)
				}
			}
			for x := -24.0; x <= 96; x += 0.5 {
				d.rect(x, coasterH(x), 0.5, 1.5, kND)
			}
		},
		Front: func(d *dc, t float64) {
			x := coasterX(t)
			h := coasterH(x)
			d.box(x-17, h-10, 34, 7, kR)
			d.rect(x-17, h-8, 34, 1, kY)
			d.ball(x-10, h-2, 1.5, kGD)
			d.ball(x+10, h-2, 1.5, kGD)
		},
		Emit: func(b *Buddy, t, dt float64) {
			if at(t, dt, 3.8) {
				b.Say("WHEEEEEEEEE!")
			}
			if at(t, dt, 5.5) {
				b.Say("AGAIN! BACKWARDS!")
			}
			if t > 3.8 && t < 5 && every(t, dt, 0.05) {
				x := coasterX(t)
				b.emit(2, x-14, coasterH(x)-4, -20, -10, 0.5, 0, 40)
			}
		},
	},
	{
		Name: "Scroll on the phone", Dur: 10, Say: []string{"JUST SCROLLING FOR A SEC...", "ONE MORE VIDEO. THEN WORK."},
		Pose: func(t float64, p *pose) {
			p.eyes, p.lx, p.ly = eLook, 0.9, 0.6
			p.arms = [][2]float64{{15, -11}, {22, -12 - 2*math.Abs(math.Sin(t*2))}}
			if t > 5 && t < 7 {
				p.eyes, p.mouth, p.sq = eHappy, mSmile, 0.04*math.Sin(t*30)
			}
		},
		Front: func(d *dc, t float64) {
			x0, y0 := 17.0, -24.0
			d.box(x0, y0, 9, 15, kX)
			sx, sy, sh := x0+1, y0+1.5, 11.0
			d.rect(sx, sy, 7, sh, kW)
			off := math.Mod(t*4, 5)
			for i := -1; i < 4; i++ {
				yy := sy + float64(i)*5 - off
				n := int(t*4/5) + i
				col := party[(n%len(party)+len(party))%len(party)]
				d.rectClip(sx+0.5, yy+0.5, 3, 3, col, sy, sy+sh)
				d.rectClip(sx+4, yy+1, 2.5, 0.7, kG, sy, sy+sh)
				d.rectClip(sx+4, yy+2.5, 2, 0.7, kG, sy, sy+sh)
			}
		},
		Emit: func(b *Buddy, t, dt float64) {
			if at(t, dt, 5) {
				b.Say(pick([]string{"HAHA, A CAT VIDEO.", "LOL. THIS MEME IS SO TRUE.", "WAIT, PEOPLE DO THAT?"}))
			}
			if at(t, dt, 8.5) {
				b.Say("OKAY OKAY. PHONE DOWN.")
			}
		},
	},
	{
		Name: "Coffee break", Dur: 8, Say: []string{"COFFEE BREAK.", "AH, BEAN JUICE."},
		Pose: func(t float64, p *pose) {
			l := sipLift(t)
			p.arms = [][2]float64{{13, -13 - l}}
			if l > 1 {
				p.eyes, p.mouth = eClosed, mO
			}
		},
		Front: func(d *dc, t float64) {
			l := sipLift(t)
			d.box(21.5, -15.5-l, 1.5, 3, kW)
			d.box(15, -17-l, 6, 6, kW)
			d.rect(15, -17-l, 6, 1, kND)
		},
		Emit: func(b *Buddy, t, dt float64) {
			if every(t, dt, 0.35) {
				b.emit(5, rnd(15, 20), -19-sipLift(t), rnd(-1, 1), -6, 1.5, 0xFFFFFFFF, 0)
			}
			if at(t, dt, 6.5) {
				b.Say("*SIP* PERFECT.")
			}
		},
	},
	{
		Name: "Read a book", Dur: 9, Say: []string{"READING A GOOD BOOK.", "JUST ONE MORE CHAPTER."},
		Pose: func(t float64, p *pose) {
			p.eyes, p.lx, p.ly = eLook, 0.6, 0.8
			if t > 5.5 && t < 7 {
				p.eyes, p.mouth = eWide, mO
			}
			p.arms = [][2]float64{{9, -11}, {28, -11}}
		},
		Front: func(d *dc, t float64) {
			d.rect(10, -8, 20, 1.5, kRD)
			d.box(12, -17, 16, 9, kM)
			d.rect(19.7, -17, 0.6, 9, kN)
			for _, y := range []float64{-15, -13, -11} {
				d.rect(13, y, 5.5, 0.6, kG)
				d.rect(21, y, 5.5, 0.6, kG)
			}
			if f := math.Mod(t, 3) / 0.5; f < 1 {
				w := 8 * math.Cos(math.Pi*f)
				d.rect(20, -17, w, 9, kW)
			}
		},
		Emit: func(b *Buddy, t, dt float64) {
			if at(t, dt, 5.5) {
				b.Say("PLOT TWIST?!")
			}
		},
	},
	{
		Name: "Listen to music", Dur: 9, Say: []string{"THIS SONG SLAPS.", "LO-FI BEATS TO CHILL TO."},
		Pose: func(t float64, p *pose) {
			p.oy, p.eyes, p.mouth, p.sq = -2*math.Abs(math.Sin(t*5)), eClosed, mSmile, 0.03*math.Sin(t*10)
		},
		Front: func(d *dc, t float64) {
			oy := -2 * math.Abs(math.Sin(t*5))
			d.arc(0, -25+oy, 18, 7, math.Pi, 2*math.Pi, 1.5, kX)
			d.box(-20, -23+oy, 4, 7, kR)
			d.box(16, -23+oy, 4, 7, kR)
		},
		Emit: func(b *Buddy, t, dt float64) {
			if every(t, dt, 0.5) {
				b.emit(3, rnd(16, 22), -30, rnd(4, 10), -12, 2, noteCol(), 0)
			}
		},
	},
	{
		Name: "Play guitar", Dur: 9, Say: []string{"*STRUMS GUITAR*", "I KNOW THREE CHORDS. THIS IS ONE."},
		Pose: func(t float64, p *pose) {
			p.eyes, p.mouth, p.sq = eClosed, mSmile, 0.02*math.Sin(t*6)
			p.arms = [][2]float64{{11, -10 + 1.5*math.Sin(t*14)}, {28, -22}}
		},
		Front: func(d *dc, t float64) {
			d.line(18, -12, 34, -25, 1.5, kND)
			d.box(33, -28, 3, 3, kN)
			d.ball(14, -8, 5, kO)
			d.disc(19, -10, 4, kO)
			d.disc(16, -9, 1.3, kND)
		},
		Emit: func(b *Buddy, t, dt float64) {
			if every(t, dt, 0.45) {
				b.emit(3, rnd(20, 26), -24, rnd(4, 10), -12, 2, noteCol(), 0)
			}
		},
	},
	{
		Name: "Eat pizza", Dur: 8.5, Say: []string{"PIZZA TIME!", "PIZZA IS JUST A CIRCLE OF HAPPINESS."},
		Pose: func(t float64, p *pose) {
			p.eyes, p.mouth = eHappy, mChew
			p.arms = [][2]float64{{12, -11}}
		},
		Front: func(d *dc, t float64) {
			bites := min(3, int(t/2))
			d.sprCut(5, -16, []string{
				"......nn",
				"....yyrn",
				"..yyyyyn",
				"yyyryyyn",
				"..yyyyyn",
				"....yryn",
				"......nn",
			}, bites*2)
		},
		Emit: func(b *Buddy, t, dt float64) {
			for _, m := range []float64{2, 4, 6} {
				if at(t, dt, m) {
					b.burst(13, 8, -13, 4, 10, kY, kN)
				}
			}
			if at(t, dt, 4) {
				b.Say("*NOM NOM*")
			}
			if at(t, dt, 7.4) {
				b.Say("DELICIOUS.")
			}
		},
	},
	{
		Name: "Blow bubble gum", Dur: 9.3, Say: []string{"*CHEWS GUM*", "WATCH THIS BUBBLE."},
		Pose: func(t float64, p *pose) {
			p.eyes, p.lx, p.ly, p.mouth = eLook, 0, 1, mO
			if gumR(t) == 0 {
				p.eyes, p.mouth = eWide, mChew
			}
			if t > 9 {
				p.sq = 0.2
			}
		},
		Front: func(d *dc, t float64) {
			if r := gumR(t); r > 0.5 {
				d.ball(r*0.3, -12, r, kP)
				d.rect(r*0.3-r*0.5, -12-r*0.6, 1, 1, kW)
			}
		},
		Emit: func(b *Buddy, t, dt float64) {
			if at(t, dt, 2.5) || at(t, dt, 6) {
				b.burst(6, 2, -12, 10, 20, kP)
				b.Say("POP!")
			}
			if at(t, dt, 9) {
				b.burst(6, 3, -12, 24, 30, kP)
				b.Say("BIG POP! IT'S IN MY ANTENNA.")
			}
		},
	},
	{
		Name: "Juggle", Dur: 8, Say: []string{"LOOK, I CAN JUGGLE!", "THREE BALLS. NO PROBLEM."},
		Pose: func(t float64, p *pose) {
			p.eyes, p.lx, p.ly = eLook, 0.4+0.6*math.Cos(t*2.4), -1
			bob := 2 * math.Sin(t*4.8)
			p.arms = [][2]float64{{5, -12 + bob}, {22, -12 - bob}}
			if t > 6.5 {
				p.eyes, p.ly, p.mouth = eLook, 1, mO
			}
		},
		Front: func(d *dc, t float64) {
			for i, col := range []uint32{kR, kB, kY} {
				ph := t*2.4 + float64(i)*2*math.Pi/3
				x, y := 14+9*math.Cos(ph), -26+12*math.Sin(ph)
				if i == 0 && t > 6.5 {
					ph0 := 6.5*2.4 + 0
					x = 14 + 9*math.Cos(ph0) + (t-6.5)*10
					y = math.Min(-2, -26+12*math.Sin(ph0)+40*(t-6.5)*(t-6.5))
				}
				d.ball(x, y, 1.8, col)
			}
		},
		Emit: func(b *Buddy, t, dt float64) {
			if at(t, dt, 6.6) {
				b.Say("OOPS.")
			}
		},
	},
	{
		Name: "Meditate", Dur: 9, Say: []string{"OMMMMM...", "INNER PEACE: LOADING...", "I AM ONE WITH THE DESKTOP."},
		Pose: func(t float64, p *pose) {
			p.oy = -6*ease(seg(t, 0, 1)) - 1.5*math.Sin(t*1.5)
			p.eyes, p.mouth = eClosed, mFlat
		},
		Emit: func(b *Buddy, t, dt float64) {
			if every(t, dt, 0.5) {
				b.emit(8, rnd(-22, 22), rnd(-36, -10), 0, -4, 1.6, kY, 0)
			}
		},
	},
	{
		Name: "Dance", Dur: 8, Say: []string{"DANCE BREAK!", "BUST A MOVE!"},
		Pose: func(t float64, p *pose) {
			a := math.Sin(t * 6)
			p.ox, p.sq, p.eyes, p.mouth = 2.5*a, 0.06*math.Sin(t*12), eHappy, mSmile
			p.arms = [][2]float64{{-19, -18 - 8*a}, {17, -18 + 8*a}}
		},
		Back: func(d *dc, t float64) {
			d.line(32, -90, 32, -74, 0.5, kG)
			d.ball(32, -70, 4, kG)
			for i := 0; i < 6; i++ {
				if (int(t*6)+i)%3 == 0 {
					d.rect(29+float64(i%3)*2, -73+float64(i/3)*3, 1, 1, kW)
				}
			}
		},
		Emit: func(b *Buddy, t, dt float64) {
			if every(t, dt, 0.4) {
				b.emit(3, rnd(-10, 20), -32, rnd(-6, 10), -12, 2, noteCol(), 0)
			}
			if every(t, dt, 0.12) {
				a := rnd(0, 2*math.Pi)
				b.emit(13, 32, -70, math.Cos(a)*30, math.Sin(a)*30, 0.8, noteCol(), 0)
			}
		},
	},
	{
		Name: "Go fishing", Dur: 10, Say: []string{"GONE FISHING.", "FISH ARE FRIENDS. AND FOOD."},
		Pose: func(t float64, p *pose) {
			p.eyes, p.lx, p.ly = eLook, 1, 0.5
			p.arms = [][2]float64{{14, -12}}
			if t > 7 {
				p.eyes = eWide
			}
			if t > 8.6 {
				p.eyes, p.mouth = eHappy, mSmile
			}
		},
		Back: func(d *dc, t float64) {
			d.rect(34, -1, 26, 2, kB)
			d.rect(37, -1.5, 20, 0.5, kC)
		},
		Front: func(d *dc, t float64) {
			d.line(16, -12, 40, -36, 1, kND)
			by := -1 + 0.5*math.Sin(t*3)
			if t > 7 && t < 7.4 {
				by = 1
			}
			fish := []string{"..ccc.c", ".cxcccc", "..ccc.c"}
			if t < 7.4 {
				d.line(40, -36, 47, by, 0.5, kG)
				d.rect(46, by-1, 2, 1, kR)
				d.rect(46, by, 2, 1, kW)
				return
			}
			f := seg(t, 7.4, 8.6)
			fx, fy := lerp(47, 22, f), lerp(0, -24, f)-14*math.Sin(math.Pi*f)
			if t > 8.6 {
				fy += math.Sin(t*20) * 0.5
			}
			d.line(40, -36, fx+3, fy, 0.5, kG)
			d.spr(fx, fy, fish)
		},
		Emit: func(b *Buddy, t, dt float64) {
			if at(t, dt, 7) {
				b.Say("A BITE!")
			}
			if at(t, dt, 8.6) {
				b.Say("I CAUGHT ONE! HI FISH.")
				b.burst(9, 47, -1, 8, 20, kC)
			}
		},
	},
	{
		Name: "Skateboard", Dur: 9, Say: []string{"SKATER MODE ON.", "RADICAL!"},
		Pose: func(t float64, p *pose) {
			p.ox, p.oy, p.noShadow = skateX(t), -3+skateHop(t), true
			if t > 4 && t < 5 {
				p.eyes, p.mouth = eHappy, mBig
			}
		},
		Front: func(d *dc, t float64) {
			x, h := skateX(t), skateHop(t)
			w := 30 * math.Abs(math.Cos(seg(t, 4, 4.8)*2*math.Pi))
			d.rect(x-w/2-0.5, -3+h, w+1, 2.5, cOutline)
			d.rect(x-w/2, -2.5+h, w, 1.5, kPU)
			if w > 20 {
				d.ball(x-10, -0.5+h, 1, kG)
				d.ball(x+10, -0.5+h, 1, kG)
			}
		},
		Emit: func(b *Buddy, t, dt float64) {
			if at(t, dt, 4) {
				b.Say("KICKFLIP!")
			}
			if at(t, dt, 4.8) {
				b.burst(2, skateX(t), 0, 8, 25, kY)
			}
		},
	},
	{
		Name: "Paint a picture", Dur: 9, Say: []string{"TIME TO PAINT.", "HAPPY LITTLE TREES."},
		Pose: func(t float64, p *pose) {
			p.eyes, p.lx, p.ly = eLook, 1, -0.3
			p.arms = [][2]float64{{20 + 3*math.Sin(t*5), -26 + 3*math.Cos(t*7)}}
		},
		Back: func(d *dc, t float64) {
			d.line(35, -36, 28, 0, 1, kND)
			d.line(35, -36, 42, 0, 1, kND)
			d.box(26, -38, 18, 14, kW)
			f := seg(t, 0.5, 7.5)
			d.rect(26, -38, 18, 8*clamp01((f-0.05)/0.2), kC)
			if f > 0.3 {
				d.rect(26, -29, 18, 5, kE)
				d.rect(30, -31, 9, 2, kE)
			}
			if f > 0.5 {
				d.disc(40, -35, 2, kY)
			}
			if f > 0.7 {
				d.rect(29, -33, 4, 3, kR)
				d.rect(30, -30, 2, 2, kN)
			}
			if f > 0.85 {
				d.rect(37, -27, 1, 1, kP)
				d.rect(40, -26, 1, 1, kPU)
			}
		},
		Front: func(d *dc, t float64) {
			x, y := 22+3*math.Sin(t*5), -24+3*math.Cos(t*7)
			d.line(x, y, x+4, y-5, 0.7, kN)
			d.rect(x+3.5, y-6, 1.2, 1.2, party[int(t)%len(party)])
		},
		Emit: func(b *Buddy, t, dt float64) {
			if at(t, dt, 7.4) {
				b.Say("A MASTERPIECE!")
			}
		},
	},
	{
		Name: "Water a plant", Dur: 9, Say: []string{"GROW, LITTLE PLANT!", "PLANTS NEED LOVE TOO."},
		Pose: func(t float64, p *pose) {
			p.eyes, p.lx, p.ly = eLook, 1, 0.5
			p.arms = [][2]float64{{13, -17}}
			if t > 7.5 {
				p.eyes, p.mouth = eHappy, mSmile
			}
		},
		Front: func(d *dc, t float64) {
			h := 3 + 9*seg(t, 1, 7)
			d.rect(33.5, -6-h, 1, h, kED)
			for y := -9.0; y > -6-h+1; y -= 3 {
				d.rect(34.5, y, 2, 1, kE)
				d.rect(31.5, y+1.5, 2, 1, kE)
			}
			if t > 7.5 {
				d.disc(34, -7.5-h, 2.2, kP)
				d.rect(33.5, -8-h, 1, 1, kY)
			}
			d.box(30, -6, 8, 6, kO)
			d.rect(29, -7, 10, 1.5, kN)
			d.box(14, -22, 7, 5, kB)
			d.line(21, -21, 26, -25, 1, kB)
		},
		Emit: func(b *Buddy, t, dt float64) {
			if t > 1 && t < 7 && every(t, dt, 0.12) {
				b.emit(9, 26, -25, rnd(3, 6), 0, 1.3, kC, 25)
			}
			if at(t, dt, 7.5) {
				b.Say("LOOK! A FLOWER!")
				b.burst(8, 34, -20, 6, 12, kY)
			}
		},
	},
	{
		Name: "Eat ice cream", Dur: 7, Say: []string{"ICE CREAM!", "STRAWBERRY AND VANILLA. CLASSIC."},
		Pose: func(t float64, p *pose) {
			p.eyes, p.mouth = eHappy, mO
			p.arms = [][2]float64{{15, -13}}
			if t > 4 && t < 5.5 {
				p.eyes, p.mouth, p.sq = eWide, mBig, 0.04*math.Sin(t*40)
			}
		},
		Front: func(d *dc, t float64) {
			rows := []string{
				"..ppp..",
				".ppppp.",
				"..mmm..",
				".mmmmm.",
				".yYyYy.",
				"..yYy..",
				"..yYy..",
				"...y...",
			}
			if t > 5.5 {
				rows[0] = "......."
			}
			d.spr(15, -24, rows)
		},
		Emit: func(b *Buddy, t, dt float64) {
			if at(t, dt, 4) {
				b.Say("BRAIN FREEZE!")
			}
		},
	},
	{
		Name: "Fly a kite", Dur: 9, Say: []string{"PERFECT KITE WEATHER.", "LOOK HOW HIGH IT GOES!"},
		Pose: func(t float64, p *pose) {
			p.eyes, p.lx, p.ly, p.mouth = eLook, 0.8, -1, mSmile
			p.arms = [][2]float64{{15, -14}}
		},
		Back: func(d *dc, t float64) {
			kx, ky := 58+4*math.Sin(t*1.3), -66+4*math.Cos(t*1.7)
			d.line(17, -14, kx, ky+3, 0.5, kG)
			for i := 1; i <= 4; i++ {
				d.rect(kx+math.Sin(t*3+float64(i))*1.5, ky+3+float64(i)*3, 1.5, 1, party[i])
			}
			d.spr(kx-3.5, ky-4, []string{
				"...r...",
				"..rry..",
				".rrryy.",
				"rrrryyy",
				".bbbee.",
				"..bbe..",
				"...b...",
			})
		},
	},
	{
		Name: "Chill in the rain", Dur: 9, Say: []string{"I DON'T MIND THE RAIN.", "RAINY DAYS ARE COZY."},
		Pose: func(t float64, p *pose) {
			p.eyes, p.mouth = eHappy, mSmile
			p.arms = [][2]float64{{17, -16}}
		},
		Front: func(d *dc, t float64) {
			d.line(8, -36, 18, -16, 1, kND)
			for yy := -10.0; yy <= 0; yy += 0.5 {
				hw := 22 * math.Sqrt(math.Max(0, 1-(yy/10)*(yy/10)))
				d.rect(8-hw-1, -36+yy-1, 2*hw+2, 1, cOutline)
			}
			for yy := -10.0; yy <= 0; yy += 0.5 {
				hw := 22 * math.Sqrt(math.Max(0, 1-(yy/10)*(yy/10)))
				for x := 8 - hw; x < 8+hw; x += 1 {
					col := uint32(kR)
					if int(math.Floor((x+30)/7))%2 == 0 {
						col = kW
					}
					d.rect(x, -36+yy, 1, 0.5, col)
				}
			}
		},
		Emit: func(b *Buddy, t, dt float64) {
			if every(t, dt, 0.04) {
				x := rnd(-24, 96)
				if x > -15 && x < 31 {
					x += 46
				}
				b.emit(7, x, -90, 0, 90, 1, 0, 0)
			}
		},
	},
	{
		Name: "Put on sunglasses", Dur: 6, Say: []string{"*PUTS ON SHADES*"},
		Pose: func(t float64, p *pose) {
			p.mouth = mFlat
			if t < 1.5 {
				p.arms = [][2]float64{{12, -22 - 20*(1-ease(seg(t, 0.3, 1.3)))}}
			}
		},
		Front: func(d *dc, t float64) {
			y := -21 - 20*(1-ease(seg(t, 0.3, 1.3)))
			d.rect(-12, y, 9, 7, kX)
			d.rect(3, y, 9, 7, kX)
			d.rect(-3, y+1, 6, 1, kX)
			if g := math.Mod(t*1.5, 3); g < 1 {
				d.rect(-11+g*8, y+1, 1, 1, kW)
				d.rect(4+g*8, y+1, 1, 1, kW)
			}
		},
		Emit: func(b *Buddy, t, dt float64) {
			if at(t, dt, 1.4) {
				b.Say("DEAL WITH IT.")
				b.burst(8, 8, -20, 5, 15, kY)
			}
		},
	},
	{
		Name: "Movie night", Dur: 9, Say: []string{"MOVIE NIGHT!", "I BROUGHT POPCORN."},
		Pose: func(t float64, p *pose) {
			p.eyes, p.lx, p.ly, p.mouth = eLook, -0.6, 0, mChew
			p.arms = [][2]float64{{13, -10}}
			if t > 5 && t < 6.5 {
				p.eyes, p.mouth = eWide, mO
			}
		},
		Front: func(d *dc, t float64) {
			for _, x := range []float64{16, 19, 22} {
				d.ball(x, -15+math.Mod(x, 2)*0.5, 1.6, kM)
			}
			d.box(15, -14, 9, 10, kW)
			d.rect(17, -14, 1.5, 10, kR)
			d.rect(21, -14, 1.5, 10, kR)
		},
		Emit: func(b *Buddy, t, dt float64) {
			if every(t, dt, 1.3) {
				b.emit(13, 19, -16, -16, -22, 0.75, kM, 45)
			}
			if at(t, dt, 5) {
				b.Say("SHH, IT'S THE GOOD PART.")
			}
		},
	},
	{
		Name: "Type on a laptop", Dur: 8, Say: []string{"TYPING VERY IMPORTANT THINGS.", "*CLACK CLACK CLACK*"},
		Pose: func(t float64, p *pose) {
			p.eyes, p.lx, p.ly, p.mouth = eLook, 0.8, 0.3, mFlat
			k := float64(int(t*10) % 2)
			p.arms = [][2]float64{{15 + k*3, -5}, {21 - k*3, -5}}
			if t > 6 {
				p.eyes, p.mouth = eHappy, mSmile
			}
		},
		Front: func(d *dc, t float64) {
			d.rect(-16, -26, 32, 20, 0x284F86E8) // screen glow
			d.box(16, -15, 14, 12, kG)
			d.disc(23, -9, 1.5, kW)
			d.box(13, -2.5, 19, 2, kGD)
		},
		Emit: func(b *Buddy, t, dt float64) {
			if every(t, dt, 0.5) {
				b.emit(13, rnd(18, 28), -16, rnd(-2, 2), -10, 1, kE, 0)
			}
			if at(t, dt, 6) {
				b.Say("IT COMPILES! SHIP IT.")
			}
		},
	},
	{
		Name: "Play video games", Dur: 8, Say: []string{"JUST ONE QUICK GAME.", "PLAYER 1 READY."},
		Pose: func(t float64, p *pose) {
			p.eyes, p.lx, p.ly, p.mouth = eWide, -0.8, 0, mFlat
			j := 0.5 * math.Sin(t*50)
			p.arms = [][2]float64{{11, -10 + j}, {24, -10 - j}}
			if t > 2 && t < 6.5 {
				p.sq = 0.03 * math.Sin(t*40)
			}
			if t > 6.5 {
				p.eyes, p.mouth = eHappy, mSmile
			}
		},
		Front: func(d *dc, t float64) {
			d.box(13, -12, 12, 5, kGD)
			d.box(12, -9, 3, 3, kGD)
			d.box(23, -9, 3, 3, kGD)
			d.rect(14.5, -11, 2, 1, kX)
			d.rect(15, -11.5, 1, 2, kX)
			d.rect(21, -11, 1, 1, kR)
			d.rect(22.5, -10, 1, 1, kB)
		},
		Emit: func(b *Buddy, t, dt float64) {
			if at(t, dt, 6.5) {
				b.Say(pick([]string{"GG EZ.", "NEW HIGH SCORE!", "I WON! DID YOU SEE THAT?"}))
				b.burst(8, 18, -20, 8, 20, kY)
			}
		},
	},
	{
		Name: "Drink boba tea", Dur: 7, Say: []string{"BOBA BREAK.", "THE PEARLS ARE THE BEST PART."},
		Pose: func(t float64, p *pose) {
			p.eyes, p.mouth = eHappy, mO
			p.arms = [][2]float64{{14, -13}}
		},
		Front: func(d *dc, t float64) {
			d.line(19, -21, 3, -13, 1.2, kP)
			d.box(16, -20, 7, 11, kM)
			d.rect(16, -17, 7, 8, 0xFFD9B38C)
			n := 6 - int(t/1.2)
			for i := 0; i < n; i++ {
				d.rect(16.5+float64(i%3)*2.2, -11-float64(i/3)*1.5, 1.2, 1.2, kX)
			}
			d.rect(15.5, -21, 8, 1.5, kW)
		},
		Emit: func(b *Buddy, t, dt float64) {
			if at(t, dt, 3) {
				b.Say("*SLUUURP*")
			}
		},
	},
	{
		Name: "Lift weights", Dur: 8, Say: []string{"GYM TIME.", "DO YOU EVEN LIFT?"},
		Pose: func(t float64, p *pose) {
			lift := math.Abs(math.Sin(t * 2))
			p.eyes, p.mouth, p.sq = eClosed, mFlat, 0.04*lift
			p.arms = [][2]float64{{18, -12 - 10*lift}}
		},
		Front: func(d *dc, t float64) {
			y := -12 - 10*math.Abs(math.Sin(t*2))
			d.line(20, y-3, 20, y+4, 1, kGD)
			d.box(18, y-4, 4, 1.5, kX)
			d.box(18, y+4, 4, 1.5, kX)
		},
		Emit: func(b *Buddy, t, dt float64) {
			for i, l := range []string{"ONE.", "TWO.", "THREE.", "FOUR...", "FIVE! GAINS."} {
				if at(t, dt, 0.8+float64(i)*math.Pi/2) {
					b.Say(l)
				}
			}
			if every(t, dt, 0.7) {
				b.emit(9, rnd(-14, 14), -26, rnd(-10, 10), -10, 0.8, kC, 60)
			}
		},
	},
	{
		Name: "Jump rope", Dur: 8, Say: []string{"JUMP ROPE! HOW MANY CAN I DO?"},
		Pose: func(t float64, p *pose) {
			p.oy, p.eyes, p.mouth = ropeOY(t), eHappy, mSmile
			p.arms = [][2]float64{{-20, -12}, {18, -12}}
		},
		Back: func(d *dc, t float64) {
			if math.Sin(ropeA(t)) > 0 {
				drawRope(d, t)
			}
		},
		Front: func(d *dc, t float64) {
			if math.Sin(ropeA(t)) <= 0 {
				drawRope(d, t)
			}
		},
		Emit: func(b *Buddy, t, dt float64) {
			if at(t, dt, 7) {
				b.Say("47! NEW RECORD.")
			}
		},
	},
	{
		Name: "Hold a balloon", Dur: 8, Say: []string{"I GOT A BALLOON!"},
		Pose: func(t float64, p *pose) {
			p.oy, p.eyes, p.mouth = balloonOY(t), eHappy, mSmile
			p.arms = [][2]float64{{16, -15}}
			if t > 4 && t < 7 {
				p.eyes, p.mouth = eWide, mO
			}
		},
		Back: func(d *dc, t float64) {
			oy := balloonOY(t)
			bx, by := 26+2*math.Sin(t*1.5), -60+2*math.Cos(t*2)+oy
			d.line(18, -14+oy, bx, by+7, 0.5, kG)
			d.ball(bx, by, 6, kR)
			d.disc(bx, by+1.5, 5.5, kR)
			d.rect(bx-0.5, by+6.5, 1.5, 1, kRD)
			d.rect(bx-3, by-3, 1.5, 2, kW)
		},
		Emit: func(b *Buddy, t, dt float64) {
			if at(t, dt, 4) {
				b.Say("UP, UP AND...")
			}
			if at(t, dt, 6.4) {
				b.Say("...NOPE. TOO HEAVY.")
			}
		},
	},
	{
		Name: "Birthday cake", Dur: 9, Say: []string{"MAKE A WISH!", "IT'S SOMEBODY'S BIRTHDAY SOMEWHERE!"},
		Pose: func(t float64, p *pose) {
			p.eyes, p.lx, p.ly = eLook, 1, 0.6
			switch {
			case t > 4 && t < 5:
				p.eyes, p.mouth, p.sq = eClosed, mO, -0.06
			case t >= 5:
				p.eyes, p.mouth = eHappy, mSmile
			}
		},
		Front: func(d *dc, t float64) {
			for i, x := range []float64{22.5, 26.5, 30.5} {
				d.rect(x, -14, 1, 4, party[i+2])
				if t < 5 {
					fl := 0.4 * math.Sin(t*20+x)
					d.rect(x-0.2, -16.5+fl, 1.4, 2.5-fl, kO)
					d.rect(x+0.2, -16+fl, 0.6, 1.2, kY)
				}
			}
			d.box(20, -10, 14, 10, kM)
			d.rect(20, -10, 14, 2.5, kP)
			d.rect(22, -7.5, 1, 1.5, kP)
			d.rect(28, -7.5, 1, 2, kP)
		},
		Emit: func(b *Buddy, t, dt float64) {
			if at(t, dt, 4) {
				b.Say("*HUFF*")
			}
			if at(t, dt, 5) {
				b.burst(6, 26, -18, 30, 40, party...)
				b.Say("YAY! I WISHED FOR MORE PETS.")
			}
			if t > 5 && t < 6.5 && every(t, dt, 0.15) {
				b.emit(5, rnd(22, 31), -16, 0, -6, 1.2, kG, 0)
			}
		},
	},
	{
		Name: "Throw a paper plane", Dur: 6.5, Say: []string{"WATCH THIS.", "PAPER PLANE TEST FLIGHT."},
		Pose: func(t float64, p *pose) {
			if t < 2 {
				p.arms = [][2]float64{{16, -17}}
			} else if t < 2.4 {
				p.arms = [][2]float64{{20, -27}}
			}
			if t > 2 {
				p.eyes, p.lx, p.ly = eLook, 1, -0.3
			}
		},
		Front: func(d *dc, t float64) {
			x, y := 17.0, -24.0
			if t > 2 {
				f := t - 2
				x, y = 17+f*24, -24-8*math.Sin(f*2.2)-f*3
			}
			d.spr(x, y, []string{"ww....", ".wwww.", "..wwwk", ".ww..."})
		},
		Emit: func(b *Buddy, t, dt float64) {
			if at(t, dt, 2.6) {
				b.Say("IT FLIES!")
			}
			if at(t, dt, 5) {
				b.Say("BYE, LITTLE PLANE.")
			}
		},
	},
	{
		Name: "Hula hoop", Dur: 7, Say: []string{"HULA HOOP CHAMPION!", "WATCH MY HIPS. I HAVE NO HIPS."},
		Pose: func(t float64, p *pose) {
			p.ox, p.eyes, p.mouth = -1.5*math.Cos(t*9), eHappy, mSmile
			p.arms = [][2]float64{{-17, -31}, {15, -31}}
		},
		Back: func(d *dc, t float64) {
			hc := 3 * math.Cos(t*9)
			d.arc(hc, -9, 21, 4, math.Pi, 2*math.Pi, 1.5, kPU)
		},
		Front: func(d *dc, t float64) {
			hc := 3 * math.Cos(t*9)
			d.arc(hc, -9, 21, 4, 0, math.Pi, 1.5, kPU)
			for a := 0.3; a < math.Pi; a += 0.7 {
				d.rect(hc+21*math.Cos(a)-0.5, -9+4*math.Sin(a)-0.5, 1, 1, kY)
			}
		},
	},
	{
		Name: "Stargazing", Dur: 9, Say: []string{"STARGAZING.", "SPACE IS SO BIG. I AM SO SMALL."},
		Pose: func(t float64, p *pose) {
			p.eyes, p.lx, p.ly = eWink, 0.6, -0.6
			if t > 5 && t < 6.5 {
				p.mouth = mO
			}
		},
		Front: func(d *dc, t float64) {
			d.line(30, -20, 24, 0, 1, kGD)
			d.line(30, -20, 36, 0, 1, kGD)
			d.line(13, -19, 38, -35, 3, kB)
			d.box(37, -38, 3, 4, kBD)
		},
		Emit: func(b *Buddy, t, dt float64) {
			if every(t, dt, 0.25) {
				b.emit(8, rnd(30, 92), rnd(-88, -48), 0, 0, 1.2, kW, 0)
			}
			if at(t, dt, 5) {
				b.emit(11, 92, -86, -55, 25, 1.4, 0, 0)
				b.Say("A SHOOTING STAR! QUICK, MAKE A WISH.")
			}
		},
	},
	{
		Name: "Knit a scarf", Dur: 9, Say: []string{"KNITTING YOU A SCARF.", "CLICK CLACK GO THE NEEDLES."},
		Pose: func(t float64, p *pose) {
			p.eyes, p.lx, p.ly, p.mouth = eLook, 0.6, 0.7, mFlat
			k := 0.5 * math.Sin(t*10)
			p.arms = [][2]float64{{12 + k, -15}, {23 - k, -15}}
		},
		Front: func(d *dc, t float64) {
			n := int(2 + t*1.3)
			for i := 0; i < n; i++ {
				col := uint32(kR)
				if i%2 == 1 {
					col = kW
				}
				d.rectClip(16, -12+float64(i), 6, 1, col, -14, -1)
			}
			k := 0.5 * math.Sin(t*10)
			d.line(13+k, -18, 25+k, -10, 0.7, kG)
			d.line(25-k, -18, 13-k, -10, 0.7, kG)
			d.line(24, -12, 30, -4, 0.5, kR)
			d.ball(30, -3, 3, kR)
			d.line(28, -5, 32, -1, 0.5, kRD)
		},
		Emit: func(b *Buddy, t, dt float64) {
			if at(t, dt, 7.8) {
				b.Say("IT'S... A VERY SHORT SCARF.")
			}
		},
	},
	{
		Name: "Blow bubbles", Dur: 8, Say: []string{"BUBBLES!", "SO MANY BUBBLES."},
		Pose: func(t float64, p *pose) {
			p.eyes, p.mouth = eHappy, mSmile
			if m := math.Mod(t, 2); m > 1.4 && m < 1.9 {
				p.mouth = mO
			}
			p.arms = [][2]float64{{15, -12}}
		},
		Front: func(d *dc, t float64) {
			d.line(17, -11, 20, -19, 0.7, kP)
			d.arc(21, -22, 2.5, 2.5, 0, 2*math.Pi, 0.8, kP)
		},
		Emit: func(b *Buddy, t, dt float64) {
			for _, m := range []float64{1.5, 3.5, 5.5, 7.5} {
				if at(t, dt, m) {
					for i := 0; i < 5; i++ {
						b.emit(4, 22, -22, rnd(8, 25), rnd(-15, -4), rnd(2.5, 4), 0, 0)
					}
				}
			}
		},
	},
	{
		Name: "Take a selfie", Dur: 6, Say: []string{"SAY CHEESE!", "SELFIE TIME."},
		Pose: func(t float64, p *pose) {
			p.eyes, p.mouth = eHappy, mSmile
			if t > 3.3 {
				p.eyes, p.lx, p.ly = eWink, -0.5, -0.5
			}
			p.arms = [][2]float64{{13, -31}}
		},
		Front: func(d *dc, t float64) {
			d.box(14, -40, 7, 10, kX)
			d.disc(16, -38, 0.8, kGD)
			if t > 3 && t < 3.15 {
				d.rect(8, -46, 20, 20, 0xC0FFFFFF)
			}
		},
		Emit: func(b *Buddy, t, dt float64) {
			if at(t, dt, 3) {
				b.burst(8, 17, -36, 6, 20, kW)
			}
			if at(t, dt, 3.4) {
				b.Say("NAILED IT. POSTING THIS.")
			}
		},
	},
	{
		Name: "Ride a bike", Dur: 8, Say: []string{"BIKE RIDE!", "LOOK, NO TRAINING WHEELS!"},
		Pose: func(t float64, p *pose) {
			p.ox, p.oy, p.noShadow, p.eyes, p.mouth = bikeX(t), -13, true, eHappy, mSmile
			p.arms = [][2]float64{{9, -19}}
		},
		Front: func(d *dc, t float64) {
			x := bikeX(t)
			rot := x / 6
			for _, wx := range []float64{x - 12, x + 12} {
				d.arc(wx, -6, 6, 6, 0, 2*math.Pi, 1.2, kX)
				for k := 0.0; k < 2; k++ {
					a := rot + k*math.Pi/2
					d.line(wx-5*math.Cos(a), -6-5*math.Sin(a), wx+5*math.Cos(a), -6+5*math.Sin(a), 0.4, kG)
				}
			}
			d.line(x-12, -6, x-2, -14, 1, kR)
			d.line(x-2, -14, x+8, -14, 1, kR)
			d.line(x+8, -14, x+12, -6, 1, kR)
			d.line(x-2, -14, x, -6, 1, kR)
			d.line(x, -6, x-12, -6, 1, kR)
			d.line(x+8, -14, x+10, -19, 1, kGD)
			d.rect(x+9, -20, 3, 1, kX)
		},
		Emit: func(b *Buddy, t, dt float64) {
			if at(t, dt, 4) {
				b.Say("RING RING!")
			}
		},
	},
	{
		Name: "Do a magic trick", Dur: 8, Say: []string{"FOR MY NEXT TRICK...", "NOTHING UP MY SLEEVES. I HAVE NO SLEEVES."},
		Pose: func(t float64, p *pose) {
			p.eyes, p.lx, p.ly = eLook, 1, 0.5
			if t > 3 {
				p.eyes, p.mouth = eHappy, mSmile
			}
			p.arms = [][2]float64{{15, -22 + math.Sin(t*6)}}
		},
		Front: func(d *dc, t float64) {
			if t > 3 {
				ry := -10 - 7*ease(seg(t, 3, 3.6))
				d.box(26.5, ry-6, 1.2, 4, kW)
				d.box(29.3, ry-6, 1.2, 4, kW)
				d.rect(26.7, ry-5, 0.8, 2, kP)
				d.rect(29.5, ry-5, 0.8, 2, kP)
				d.ball(28.5, ry, 3, kW)
				d.rect(27, ry-1, 0.8, 0.8, kX)
				d.rect(29.5, ry-1, 0.8, 0.8, kX)
			}
			d.box(23, -10, 11, 8, kX)
			d.rect(23, -4, 11, 1.5, kR)
			d.box(20, -2, 17, 1.5, kX)
			y := -21 + math.Sin(t*6)
			d.line(17, y, 24, y-6, 0.8, kX)
			d.rect(23.5, y-7, 1.2, 1.2, kW)
		},
		Emit: func(b *Buddy, t, dt float64) {
			if at(t, dt, 2.4) {
				b.Say("ABRACADABRA...")
			}
			if at(t, dt, 3.1) {
				b.burst(8, 28, -14, 10, 25, kY, kW, kP)
				b.Say("TA-DA! A BUNNY!")
			}
		},
	},
	{
		Name: "Build a snowman", Dur: 9, Say: []string{"SNOW DAY! BUILDING A FRIEND."},
		Pose: func(t float64, p *pose) {
			p.eyes, p.mouth = eHappy, mSmile
			p.arms = [][2]float64{{17, -12 + math.Sin(t*5)}}
		},
		Front: func(d *dc, t float64) {
			if t > 1 {
				d.ball(34, -6, 6*ease(seg(t, 1, 2)), kW)
			}
			if t > 3 {
				d.ball(34, -16, 4.5*ease(seg(t, 3, 4)), kW)
			}
			if t > 5 {
				d.ball(34, -23.5, 3.5*ease(seg(t, 5, 6)), kW)
			}
			if t > 6 {
				d.rect(32.5, -25, 1, 1, kX)
				d.rect(35, -25, 1, 1, kX)
				d.rect(31, -23.5, 3, 1, kO)
				d.line(30, -17, 25, -20, 0.6, kND)
				d.line(38, -17, 43, -20, 0.6, kND)
			}
		},
		Emit: func(b *Buddy, t, dt float64) {
			if every(t, dt, 0.07) {
				b.emit(10, rnd(-24, 96), -90, 0, rnd(15, 25), 4, 0, 0)
			}
			if at(t, dt, 7) {
				b.Say("HIS NAME IS GERALD.")
			}
		},
	},
	{
		Name: "Toast marshmallows", Dur: 10, Say: []string{"CAMPFIRE TIME.", "MARSHMALLOW O'CLOCK."},
		Pose: func(t float64, p *pose) {
			p.eyes, p.lx, p.ly = eLook, 1, 0.3
			p.arms = [][2]float64{{15, -15}}
			if t > 8.8 {
				p.eyes, p.mouth = eWide, mO
			}
		},
		Front: func(d *dc, t float64) {
			d.line(24, -1, 36, -3, 2, kND)
			d.line(24, -3, 36, -1, 2, kN)
			for k, h := range []float64{7 + 2*math.Sin(t*13), 5 + 1.5*math.Sin(t*17+1), 4 + math.Sin(t*11+2)} {
				x := 30 + float64(k-1)*2.5
				for i := 0.0; i < h; i++ {
					w := (h - i) / h * 4
					col := uint32(kR)
					if i > h*0.4 {
						col = kO
					}
					if i > h*0.75 {
						col = kY
					}
					d.rect(x-w/2, -3-i, w, 1, col)
				}
			}
			d.line(16, -14, 27, -20, 0.6, kN)
			col := uint32(kW)
			switch {
			case t > 8.5:
				col = kX
			case t > 6.8:
				col = 0xFF8A5A2A
			case t > 4:
				col = 0xFFE8C080
			}
			d.box(26.5, -21.5, 2.5, 2, col)
		},
		Emit: func(b *Buddy, t, dt float64) {
			if every(t, dt, 0.2) {
				b.emit(12, rnd(27, 33), -8, rnd(-3, 3), -18, 1, kO, 0)
			}
			if at(t, dt, 7) {
				b.Say("PERFECTLY GOLDEN.")
			}
			if at(t, dt, 8.8) {
				b.Say("OOPS. CRISPY.")
			}
			if t > 8.5 && every(t, dt, 0.15) {
				b.emit(5, 27.5, -23, 0, -8, 1.2, kGD, 0)
			}
		},
	},
	{
		Name: "Go surfing", Dur: 8, Say: []string{"SURF'S UP!", "COWABUNGA!"},
		Pose: func(t float64, p *pose) {
			p.ox, p.oy, p.noShadow, p.eyes, p.mouth = 10*math.Sin(t*1.2), surfOY(t), true, eHappy, mSmile
			p.arms = [][2]float64{{-20, -16}, {18, -16}}
		},
		Back: func(d *dc, t float64) {
			for x := 50.0; x < 96; x++ {
				h := 30 * math.Sin(math.Pi*(x-50)/46)
				d.rect(x, -h, 1, h, kB)
				d.rect(x, -h, 1, 1.5, kW)
			}
		},
		Front: func(d *dc, t float64) {
			x, oy := 10*math.Sin(t*1.2), surfOY(t)
			d.box(x-14, oy-0.5, 28, 1.5, kY)
			for xx := -24.0; xx < 96; xx++ {
				wy := -2 + 1.5*math.Sin(xx*0.3-t*4)
				d.rect(xx, wy, 1, 4-wy, kB)
				d.rect(xx, wy, 1, 0.6, kC)
			}
		},
		Emit: func(b *Buddy, t, dt float64) {
			if every(t, dt, 0.1) {
				b.emit(9, 10*math.Sin(t*1.2)+rnd(-14, 14), -2, rnd(-15, 15), rnd(-25, -10), 0.8, kC, 60)
			}
		},
	},
	{
		Name: "Bubble bath", Dur: 9, Say: []string{"BUBBLE BATH. DO NOT DISTURB.", "SPA DAY."},
		Pose: func(t float64, p *pose) {
			p.eyes, p.mouth, p.sq = eClosed, mSmile, 0.02*math.Sin(t*2)
		},
		Front: func(d *dc, t float64) {
			d.box(-22, -14, 44, 13, kW)
			d.rect(-18, -1, 3, 1.5, kYD)
			d.rect(15, -1, 3, 1.5, kYD)
			for x := -20.0; x <= 20; x += 5 {
				d.disc(x, -15+0.6*math.Sin(x+t*2), 2.5, kW)
			}
			d.spr(10, -19+0.7*math.Sin(t*2), []string{"..yy..", "..yxyo", "yyyyy.", ".yyy.."})
		},
		Emit: func(b *Buddy, t, dt float64) {
			if every(t, dt, 0.6) {
				b.emit(4, rnd(-18, 18), -16, rnd(-3, 3), -8, 2.5, 0, 0)
			}
			if at(t, dt, 5) {
				b.Say("*SQUEAK SQUEAK*")
			}
		},
	},
	{
		Name: "Spin around", Dur: 6.5, Say: []string{"SPIN TO WIN!"},
		Pose: func(t float64, p *pose) {
			if t < 3 {
				p.sq = 0.08 * math.Sin(t*25)
			} else {
				p.eyes, p.mouth, p.ox = eDizzy, mO, math.Sin(t*3)
			}
		},
		Front: func(d *dc, t float64) {
			if t > 3 {
				for i := 0.0; i < 3; i++ {
					a := t*4 + i*2.09
					star(d, math.Sin(t*3)+14*math.Cos(a), -32+3*math.Sin(a), kY)
				}
			}
		},
		Emit: func(b *Buddy, t, dt float64) {
			if t < 3 {
				b.face = math.Sin(t * 25)
			}
			if at(t, dt, 3) {
				b.Say("WHOA... I'M DIZZY.")
			}
		},
	},
	{
		Name: "Party horn", Dur: 6, Say: []string{"PARTY TIME!", "TOOT TOOT!"},
		Pose: func(t float64, p *pose) {
			p.mouth, p.eyes = mHide, eHappy
			if math.Sin(t*3) > 0.5 {
				p.eyes = eClosed
			}
		},
		Front: func(d *dc, t float64) {
			l := 16 * clamp01(math.Sin(t*3))
			d.rect(-1, -13.5, 3, 3, kY)
			if l > 2 {
				d.rect(2, -13, l, 2, kP)
				for x := 3.0; x < 2+l; x += 3 {
					d.rect(x, -13, 1, 2, kY)
				}
			} else {
				d.ball(4, -12, 2, kP)
			}
		},
		Emit: func(b *Buddy, t, dt float64) {
			for _, m := range []float64{math.Pi / 6, math.Pi/6 + 2*math.Pi/3, math.Pi/6 + 4*math.Pi/3} {
				if at(t, dt, m) {
					b.burst(6, 18, -12, 10, 25, party...)
				}
			}
		},
	},
	{
		Name: "Drum solo", Dur: 7, Say: []string{"DRUM SOLO!", "*BA DUM BA DUM*"},
		Pose: func(t float64, p *pose) {
			p.eyes, p.mouth, p.sq = eClosed, mSmile, 0.03*math.Sin(t*36)
			ph := float64(int(t*6) % 2)
			p.arms = [][2]float64{{15, -15 + ph*3}, {24, -15 + (1-ph)*3}}
		},
		Front: func(d *dc, t float64) {
			ph := float64(int(t*6) % 2)
			d.box(14, -10, 15, 6, kW)
			d.rect(14, -10, 15, 1, kR)
			d.rect(14, -5, 15, 1, kR)
			d.line(34, -1, 34, -22, 0.8, kGD)
			cy := -23.0
			if t > 6 {
				cy += math.Sin(t*40) * 0.6
			}
			d.box(29, cy, 10, 1.2, kYD)
			d.line(17, -13+ph*3, 21, -10, 0.6, kN)
			d.line(26, -13+(1-ph)*3, 22, -10, 0.6, kN)
		},
		Emit: func(b *Buddy, t, dt float64) {
			if every(t, dt, 1.0/3) {
				b.emit(8, rnd(16, 26), -12, rnd(-10, 10), -15, 0.5, kY, 0)
			}
			if at(t, dt, 6) {
				b.Say("BA DUM TSS.")
			}
		},
	},
	{
		Name: "Solve a puzzle cube", Dur: 8, Say: []string{"I CAN SOLVE THIS. PROBABLY."},
		Pose: func(t float64, p *pose) {
			p.eyes, p.lx, p.ly, p.mouth = eLook, 0.7, 0.5, mFlat
			p.arms = [][2]float64{{12, -14}, {24, -14}}
			if t > 6 {
				p.eyes, p.mouth = eHappy, mSmile
			}
		},
		Front: func(d *dc, t float64) {
			d.box(15, -21, 9, 9, kX)
			cols := []uint32{kR, kB, kE, kY, kO, kW}
			for i := 0; i < 3; i++ {
				for j := 0; j < 3; j++ {
					col := uint32(kR)
					if t < 6 {
						col = cols[(i*7+j*13+int(t*2)*5)%len(cols)]
					}
					d.rect(15.3+float64(i)*3, -20.7+float64(j)*3, 2.4, 2.4, col)
				}
			}
		},
		Emit: func(b *Buddy, t, dt float64) {
			if at(t, dt, 6) {
				b.Say("SOLVED! (I PEELED THE STICKERS.)")
			}
		},
	},
	{
		Name: "Sneeze", Dur: 6, Say: []string{"AH..."},
		Pose: func(t float64, p *pose) {
			switch {
			case t < 2.4:
				p.eyes, p.mouth, p.sq = eClosed, mO, -0.08*seg(t, 0, 2.4)
			case t < 3.5:
				p.eyes, p.mouth, p.sq, p.ox = eClosed, mBig, 0.3*(1-seg(t, 2.4, 3.2)), -3*(1-seg(t, 2.4, 3.5))
			default:
				p.eyes, p.mouth = eHappy, mSmile
			}
		},
		Emit: func(b *Buddy, t, dt float64) {
			if at(t, dt, 1.2) {
				b.Say("AHH... AHHH...")
			}
			if at(t, dt, 2.4) {
				b.Say("ACHOO!")
				for i := 0; i < 14; i++ {
					b.emit(2, 4, -12, rnd(20, 70), rnd(-30, 10), 0.6, 0, 40)
				}
			}
			if at(t, dt, 4) {
				b.Say("EXCUSE ME.")
			}
		},
	},
	{
		Name: "Go bowling", Dur: 7, Say: []string{"BOWLING TIME.", "GUTTER BALLS DON'T COUNT."},
		Pose: func(t float64, p *pose) {
			p.eyes, p.lx, p.ly = eLook, 1, 0.3
			if t < 1.4 {
				p.arms = [][2]float64{{16, -7 - 4*math.Sin(t*4)}}
			}
			if t > 3.4 {
				p.eyes, p.mouth = eHappy, mSmile
			}
		},
		Front: func(d *dc, t float64) {
			for i := 0; i < 5; i++ {
				x, y := 80+float64(i)*2.6, -5.0-float64(i%2)
				if t > 3.4 {
					f := t - 3.4
					x += (float64(i) - 1.5) * 8 * f
					y = math.Min(-5, y-30*f+60*f*f)
				}
				d.box(x, y, 1.6, 5, kW)
				d.rect(x, y+1, 1.6, 0.8, kR)
			}
			bx := 18.0
			if t > 1.4 {
				bx = math.Min(100, 18+(t-1.4)*32)
			}
			d.ball(bx, -2.5, 2.5, kB)
			d.rect(bx-0.5+math.Sin(t*10), -3.5, 0.8, 0.8, kX)
		},
		Emit: func(b *Buddy, t, dt float64) {
			if at(t, dt, 3.4) {
				b.Say("STRIKE!!")
				b.burst(6, 84, -10, 20, 35, party...)
			}
		},
	},
	{
		Name: "Shoot hoops", Dur: 6, Say: []string{"FROM DOWNTOWN...", "WATCH THIS SHOT."},
		Pose: func(t float64, p *pose) {
			p.eyes, p.lx, p.ly = eLook, 1, -0.6
			if t < 1.5 {
				p.arms = [][2]float64{{15, -21}}
			}
			if t > 2.6 {
				p.eyes, p.mouth = eHappy, mSmile
			}
		},
		Back: func(d *dc, t float64) {
			d.line(74, 0, 74, -56, 1.2, kGD)
			d.box(71, -66, 2, 14, kW)
			net := 0.0
			if t > 2.6 && t < 3.2 {
				net = math.Sin(t*30) * 0.5
			}
			for i := 0.0; i < 4; i++ {
				d.line(63+i*2.2, -55, 64+i*2+net, -51, 0.4, kW)
			}
			d.rect(62, -56, 9, 1, kO)
		},
		Front: func(d *dc, t float64) {
			x, y := 17.0, -24.0
			switch {
			case t > 3:
				f := seg(t, 3, 5)
				x, y = lerp(66, 32, f), -2.5-6*math.Abs(math.Sin(f*3*math.Pi))*(1-f)
			case t > 2.6:
				x, y = 66, lerp(-58, -2.5, seg(t, 2.6, 3))
			case t > 1.5:
				f := seg(t, 1.5, 2.6)
				x, y = lerp(17, 66, f), lerp(-24, -58, f)-30*math.Sin(math.Pi*f)
			}
			d.ball(x, y, 2.5, kO)
			d.rect(x-2.5, y, 5, 0.4, kND)
		},
		Emit: func(b *Buddy, t, dt float64) {
			if at(t, dt, 2.7) {
				b.Say("NOTHING BUT NET!")
				b.burst(8, 66, -56, 8, 20, kY)
			}
		},
	},
	{
		Name: "Play detective", Dur: 9, Say: []string{"HMM. SUSPICIOUS.", "SOMETHING IS AFOOT."},
		Pose: func(t float64, p *pose) {
			gx, gy := 7+2*math.Sin(t*1.3), -17+math.Cos(t*1.7)
			p.ox, p.mouth = 6*math.Sin(t*0.8), mFlat
			p.arms = [][2]float64{{gx + 9, gy + 11}}
		},
		Front: func(d *dc, t float64) {
			ox := 6 * math.Sin(t*0.8)
			d.rect(ox-12, -29, 24, 2, kND)
			d.rect(ox-8, -33, 16, 4, kN)
			d.rect(ox-8, -31, 16, 1, kND)
			gx, gy := ox+7+2*math.Sin(t*1.3), -17+math.Cos(t*1.7)
			d.line(gx+4, gy+4, gx+10, gy+11, 1.5, kND)
			d.ball(gx, gy, 6, kYD)
			d.disc(gx, gy, 5, 0xFFDDF4FF)
			d.disc(gx, gy, 3.5, kW)
			d.disc(gx+1, gy+0.5, 2, kX)
			d.rect(gx+0.3, gy-0.5, 0.8, 0.8, kW)
		},
		Emit: func(b *Buddy, t, dt float64) {
			if at(t, dt, 3.5) {
				b.Say("THE CASE OF THE MISSING SEMICOLON.")
			}
			if at(t, dt, 7.3) {
				b.Say("CASE CLOSED.")
			}
		},
	},
	{
		Name: "Pet a kitty", Dur: 9, Say: []string{"A KITTY! HI KITTY.", "OH! A VISITOR!"},
		Pose: func(t float64, p *pose) {
			p.eyes, p.lx, p.ly = eLook, 1, 0.6
			if t > 2.5 && t < 6.5 {
				p.eyes, p.mouth = eHappy, mSmile
				p.arms = [][2]float64{{20, -10 + math.Sin(t*6)}}
			}
		},
		Front: func(d *dc, t float64) {
			x, walking := 30.0, false
			switch {
			case t < 2.5:
				x, walking = lerp(96, 30, seg(t, 0, 2.5)), true
			case t > 6.5:
				x, walking = lerp(30, 100, seg(t, 6.5, 8.5)), true
			}
			legs := ".g.g.g.g."
			if walking && int(t*8)%2 == 0 {
				legs = "g.g...g.g"
			}
			rows := []string{"g.g.....g", "ggg....g.", "gxgggggg.", "ggggggg..", legs}
			if t > 6.5 {
				for i, r := range rows { // walk away: face the other way
					rr := []rune(r)
					for a, z := 0, len(rr)-1; a < z; a, z = a+1, z-1 {
						rr[a], rr[z] = rr[z], rr[a]
					}
					rows[i] = string(rr)
				}
			}
			d.spr(x, -5, rows)
		},
		Emit: func(b *Buddy, t, dt float64) {
			if t > 2.5 && t < 6.5 && every(t, dt, 0.7) {
				b.emit(0, 32, -8, 0, -25, 1.2, 0, 0)
			}
			if at(t, dt, 4) {
				b.Say("IT'S PURRING!")
			}
			if at(t, dt, 7) {
				b.Say("BYE KITTY!")
			}
		},
	},
	{
		Name: "Get the hiccups", Dur: 8.8, Say: []string{"I THINK I HAVE THE HIC-"},
		Pose: func(t float64, p *pose) {
			p.oy, p.mouth = hicOY(t), mFlat
			if hicOY(t) < -1 {
				p.eyes, p.mouth = eWide, mO
			}
		},
		Emit: func(b *Buddy, t, dt float64) {
			for _, h := range []float64{1, 2.6, 3.4, 5.2, 6.4} {
				if at(t, dt, h) {
					b.Say("HIC!")
				}
			}
			if at(t, dt, 7) {
				b.Say("I THINK THEY'RE GONE.")
			}
			if at(t, dt, 8) {
				b.Say("HIC!")
			}
		},
	},
	{
		Name: "Launch a rocket", Dur: 7, Say: []string{"ROCKET LAUNCH IN 3..."},
		Pose: func(t float64, p *pose) {
			p.eyes, p.lx, p.ly = eLook, 1, 0.4
			if t > 3.6 {
				p.eyes, p.ly, p.mouth = eWide, -1, mO
			}
		},
		Front: func(d *dc, t float64) {
			y, sh := -14.0, 0.0
			if t > 3.6 {
				f := t - 3.6
				y -= 40 * f * f
			} else if t > 3 {
				sh = math.Sin(t*60) * 0.4
			}
			if t > 3.6 {
				d.rect(31+sh, y+12, 3, 2+math.Sin(t*30), kO)
				d.rect(31.5+sh, y+12, 2, 1.2, kY)
			}
			d.spr(30+sh, y, []string{
				"..r..",
				".rrr.",
				".www.",
				".wcw.",
				".www.",
				".www.",
				".www.",
				"rwwwr",
				"rwwwr",
				"r.r.r",
			})
		},
		Emit: func(b *Buddy, t, dt float64) {
			if at(t, dt, 1.2) {
				b.Say("2...")
			}
			if at(t, dt, 2.4) {
				b.Say("1...")
			}
			if at(t, dt, 3.6) {
				b.Say("LIFTOFF!")
			}
			if t > 3.4 && every(t, dt, 0.06) {
				b.emit(5, rnd(29, 36), rnd(-4, -1), rnd(-12, 12), rnd(-4, 0), 1.5, kG, 0)
			}
		},
	},
	{
		Name: "Play with a yo-yo", Dur: 7, Say: []string{"YO-YO TIME.", "WALK THE DOG!"},
		Pose: func(t float64, p *pose) {
			p.eyes, p.lx, p.ly, p.mouth = eLook, 0.8, 0.8, mSmile
			p.arms = [][2]float64{{16, -15}}
		},
		Front: func(d *dc, t float64) {
			x, y := 19.0, -14+12*math.Abs(math.Sin(t*2.5))
			if t > 4 && t < 6 {
				x, y = 19+20*math.Sin(math.Pi*seg(t, 4, 6)), -2
			}
			d.line(18, -14, x, y, 0.4, kG)
			d.ball(x, y, 1.8, kR)
			d.rect(x-0.4, y-0.4, 0.8, 0.8, kW)
		},
	},
	{
		Name: "Have an idea", Dur: 7, Say: []string{"HMMMM...", "THINKING..."},
		Pose: func(t float64, p *pose) {
			p.eyes, p.lx, p.ly, p.mouth = eLook, 0.7, -1, mFlat
			if t > 3.5 {
				p.eyes, p.mouth = eWide, mSmile
			}
			if t > 5.5 {
				p.eyes, p.mouth = eLook, mO
			}
		},
		Front: func(d *dc, t float64) {
			d.ball(15, -32, 1, kW)
			d.ball(20, -38, 1.5, kW)
			for _, q := range [][3]float64{{30, -48, 5}, {36, -50, 6}, {42, -48, 5}, {36, -45, 5}} {
				d.disc(q[0], q[1], q[2]+1, cOutline)
			}
			for _, q := range [][3]float64{{30, -48, 5}, {36, -50, 6}, {42, -48, 5}, {36, -45, 5}} {
				d.disc(q[0], q[1], q[2], kW)
			}
			switch {
			case t < 3.5 || t > 5.5:
				d.label(35, -51, "?", cOutline)
			default:
				d.ball(36, -51, 3, kY)
				d.box(35, -48, 2, 2, kG)
			}
		},
		Emit: func(b *Buddy, t, dt float64) {
			if at(t, dt, 3.5) {
				b.Say("I HAVE AN IDEA!")
				b.burst(8, 36, -51, 8, 18, kY)
			}
			if at(t, dt, 5.5) {
				b.Say("...I FORGOT IT.")
			}
		},
	},
	{
		Name: "Wave hello", Dur: 4, Say: []string{"HI! JUST WAVING AT YOU.", "HELLO THERE!"},
		Pose: func(t float64, p *pose) {
			p.eyes, p.mouth = eHappy, mSmile
			p.arms = [][2]float64{{18 + 2*math.Sin(t*12), -26}}
		},
		Emit: func(b *Buddy, t, dt float64) {
			if every(t, dt, 0.5) {
				b.emit(2, 22, -30, rnd(-5, 15), -15, 0.6, 0, 0)
			}
		},
	},
}

func drawRope(d *dc, t float64) {
	oy := ropeOY(t)
	mid := -19 - 21*math.Cos(ropeA(t))
	c := 2*mid + 12
	x0, y0, x1, y1 := -19.0, -11+oy, 19.0, -11+oy
	prevX, prevY := x0, y0
	for i := 1; i <= 24; i++ {
		f := float64(i) / 24
		x := lerp(x0, x1, f)
		y := (1-f)*(1-f)*y0 + 2*(1-f)*f*(c+oy) + f*f*y1
		d.line(prevX, prevY, x, y, 0.8, kR)
		prevX, prevY = x, y
	}
}
