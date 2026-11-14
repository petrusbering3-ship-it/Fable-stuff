// Fable Buddy - simulation and software renderer (platform independent).
package main

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"
)

// ---------- canvas (premultiplied BGRA, top-down) ----------

type Canvas struct {
	W, H int
	Pix  []byte
}

func (c *Canvas) Clear() { clear(c.Pix) }

// Rect fills a rectangle with a 0xAARRGGBB colour using source-over blending.
func (c *Canvas) Rect(x, y, w, h int, col uint32) {
	a := col >> 24
	if a == 0 {
		return
	}
	if x < 0 {
		w += x
		x = 0
	}
	if y < 0 {
		h += y
		y = 0
	}
	if x+w > c.W {
		w = c.W - x
	}
	if y+h > c.H {
		h = c.H - y
	}
	if w <= 0 || h <= 0 {
		return
	}
	r, g, b := (col>>16)&255*a/255, (col>>8)&255*a/255, col&255*a/255
	ia := 255 - a
	for yy := y; yy < y+h; yy++ {
		i := (yy*c.W + x) * 4
		for xx := 0; xx < w; xx++ {
			p := c.Pix[i : i+4 : i+4]
			p[0] = byte(b + uint32(p[0])*ia/255)
			p[1] = byte(g + uint32(p[1])*ia/255)
			p[2] = byte(r + uint32(p[2])*ia/255)
			p[3] = byte(a + uint32(p[3])*ia/255)
			i += 4
		}
	}
}

func fade(col uint32, f float64) uint32 {
	if f >= 1 {
		return col
	}
	if f < 0 {
		f = 0
	}
	return uint32(float64(col>>24)*f)<<24 | col&0xFFFFFF
}

// ---------- tiny 3x5 pixel font ----------

var font = map[rune][5]uint8{
	'A': {2, 5, 7, 5, 5}, 'B': {6, 5, 6, 5, 6}, 'C': {3, 4, 4, 4, 3}, 'D': {6, 5, 5, 5, 6},
	'E': {7, 4, 6, 4, 7}, 'F': {7, 4, 6, 4, 4}, 'G': {3, 4, 5, 5, 3}, 'H': {5, 5, 7, 5, 5},
	'I': {7, 2, 2, 2, 7}, 'J': {1, 1, 1, 5, 2}, 'K': {5, 5, 6, 5, 5}, 'L': {4, 4, 4, 4, 7},
	'M': {17, 27, 21, 17, 17}, 'N': {7, 5, 5, 5, 5}, 'O': {2, 5, 5, 5, 2}, 'P': {6, 5, 6, 4, 4},
	'Q': {2, 5, 5, 7, 3}, 'R': {6, 5, 6, 5, 5}, 'S': {3, 4, 2, 1, 6}, 'T': {7, 2, 2, 2, 2},
	'U': {5, 5, 5, 5, 7}, 'V': {5, 5, 5, 5, 2}, 'W': {17, 17, 21, 27, 17}, 'X': {5, 5, 2, 5, 5},
	'Y': {5, 5, 2, 2, 2}, 'Z': {7, 1, 2, 4, 7},
	'0': {7, 5, 5, 5, 7}, '1': {2, 6, 2, 2, 7}, '2': {6, 1, 2, 4, 7}, '3': {6, 1, 2, 1, 6},
	'4': {5, 5, 7, 1, 1}, '5': {7, 4, 6, 1, 6}, '6': {3, 4, 7, 5, 7}, '7': {7, 1, 2, 2, 2},
	'8': {7, 5, 7, 5, 7}, '9': {7, 5, 7, 1, 6},
	'.': {0, 0, 0, 0, 2}, ',': {0, 0, 0, 2, 4}, '!': {2, 2, 2, 0, 2}, '?': {6, 1, 2, 0, 2},
	'\'': {2, 2, 0, 0, 0}, ':': {0, 2, 0, 2, 0}, '-': {0, 0, 7, 0, 0}, '*': {0, 5, 2, 5, 0},
	'%': {5, 1, 2, 4, 5}, '+': {0, 2, 7, 2, 0},
}

func (c *Canvas) text(x, y, px int, s string, col uint32) {
	for _, ch := range s {
		if g, ok := font[ch]; ok {
			gw := glyphW(ch)
			for r := 0; r < 5; r++ {
				for k := 0; k < gw; k++ {
					if g[r]>>(gw-1-k)&1 == 1 {
						c.Rect(x+k*px, y+r*px, px, px, col)
					}
				}
			}
		}
		x += (glyphW(ch) + 1) * px
	}
}

func glyphW(ch rune) int {
	if ch == 'M' || ch == 'W' {
		return 5
	}
	return 3
}

func textW(s string) int {
	n := 0
	for _, ch := range s {
		n += glyphW(ch) + 1
	}
	return n
}

func wrap(s string, max int) []string {
	var lines []string
	cur := ""
	for _, w := range strings.Fields(s) {
		if cur != "" && len(cur)+1+len(w) > max {
			lines = append(lines, cur)
			cur = w
		} else if cur == "" {
			cur = w
		} else {
			cur += " " + w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

// ---------- the buddy ----------

const (
	Idle = iota
	Walk
	Drag
	Air
	Sleep
)

const (
	canvasW = 240 // in scale units
	canvasH = 190
)

type particle struct {
	x, y, vx, vy, life, max float64
	kind                    int // 0 heart, 1 Z, 2 sparkle
}

type Buddy struct {
	S          int     // integer UI scale
	L, T, R, B float64 // work area in screen pixels
	X, Y       float64 // feet centre in screen pixels
	VX, VY     float64
	State      int
	HomeRight  bool

	t, idleT, awayWait, nextAct float64
	blinkIn, blinkT, happy      float64
	squash, lookX, lookY        float64
	walkTo, face                float64
	offX, offY, moved, zT       float64
	text                        string
	textAge, textDur            float64
	parts                       []particle
	pets, lastHour              int
	started                     time.Time
}

func NewBuddy(scale int, l, t, r, b float64, homeRight bool) *Buddy {
	bd := &Buddy{S: scale, HomeRight: homeRight, blinkIn: 2, nextAct: 12, started: time.Now()}
	bd.SetArea(l, t, r, b)
	bd.X = bd.homeX()
	bd.Y = t + (b-t)*0.55 // drop in from above
	bd.State = Air
	bd.lastHour = time.Now().Hour()
	h := time.Now().Hour()
	switch {
	case h < 5:
		bd.Say("STILL UP? I'LL KEEP YOU COMPANY.")
	case h < 12:
		bd.Say("GOOD MORNING! LET'S BUILD SOMETHING.")
	case h < 18:
		bd.Say("GOOD AFTERNOON! I'M RIGHT HERE.")
	default:
		bd.Say("GOOD EVENING! NICE TO SEE YOU.")
	}
	return bd
}

func (b *Buddy) SetArea(l, t, r, bt float64) { b.L, b.T, b.R, b.B = l, t, r, bt }

func (b *Buddy) homeX() float64 {
	m := 60 * float64(b.S)
	if b.HomeRight {
		return b.R - m
	}
	return b.L + m
}

func (b *Buddy) Say(s string) {
	b.text = strings.ToUpper(s)
	b.textAge = 0
	b.textDur = 2.5 + float64(len(s))*0.07
}

var quips = []string{
	"REMEMBER TO DRINK SOME WATER.",
	"YOU'RE DOING GREAT. SERIOUSLY.",
	"SHIP IT!",
	"HAVE YOU SAVED YOUR WORK?",
	"STRETCH YOUR SHOULDERS. I'LL WAIT.",
	"I LIVE ON YOUR TASKBAR NOW.",
	"TRY THROWING ME. IT'S FUN.",
	"ONE SMALL STEP AT A TIME.",
	"LOOK AWAY FROM THE SCREEN FOR 20 SECONDS.",
	"I BELIEVE IN YOU.",
	"BUGS ARE JUST SURPRISE FEATURES.",
	"WHAT ARE WE MAKING TODAY?",
	"DEEP BREATH IN... AND OUT.",
	"RIGHT-CLICK ME FOR OPTIONS.",
}

func (b *Buddy) Quip() {
	switch rand.Intn(6) {
	case 0:
		b.Say(time.Now().Format("IT'S 15:04. TIME FLIES!"))
	case 1:
		m := int(time.Since(b.started).Minutes())
		b.Say(fmt.Sprintf("WE'VE HUNG OUT FOR %d MIN. YOU PETTED ME %d TIMES.", m, b.pets))
	default:
		b.Say(quips[rand.Intn(len(quips))])
	}
}

func (b *Buddy) Hop(power float64) {
	if b.State == Drag {
		return
	}
	b.VY = -power * float64(b.S)
	b.State = Air
}

func (b *Buddy) SetHome(right bool) {
	b.HomeRight = right
	b.idleT, b.awayWait = 0, 0.3
	if b.State == Sleep {
		b.State = Idle
	}
	b.Say("MOVING! BE RIGHT THERE.")
}

func (b *Buddy) spawn(kind, n int) {
	S := float64(b.S)
	for i := 0; i < n; i++ {
		p := particle{kind: kind, x: (rand.Float64()*40 - 20) * S, y: -50 * S}
		switch kind {
		case 0:
			p.vx, p.vy, p.max = (rand.Float64()*40-20)*S, -(40+rand.Float64()*30)*S, 1.3
		case 1:
			p.x, p.y = 14*S, -58*S
			p.vx, p.vy, p.max = 5*S, -20*S, 2.4
		case 2:
			p.y = -2 * S
			p.vx, p.vy, p.max = (rand.Float64()*240-120)*S, -(40+rand.Float64()*120)*S, 0.6
		}
		p.life = p.max
		b.parts = append(b.parts, p)
	}
}

// Mouse input. Positions are screen pixels.
func (b *Buddy) Down(cx, cy float64) {
	b.State = Drag
	b.offX, b.offY = b.X-cx, b.Y-cy
	b.moved, b.VX, b.VY = 0, 0, 0
}

func (b *Buddy) Up() {
	if b.State != Drag {
		return
	}
	S := float64(b.S)
	b.State = Air
	b.idleT, b.awayWait = 0, 2.5
	if b.moved < 6*S { // a click: pet!
		b.VX, b.VY = 0, 0
		b.pets++
		b.happy = 1.6
		b.spawn(0, 4)
		if b.Y >= b.B-1 {
			b.VY = -330 * S
		}
		switch {
		case b.pets == 1:
			b.Say("HEHE! THAT TICKLES.")
		case b.pets%25 == 0:
			b.Say(fmt.Sprintf("%d PETS! I'M THE LUCKIEST BUDDY.", b.pets))
		case b.pets%7 == 0:
			b.Say("MORE PETS, PLEASE!")
		}
		return
	}
	lim := 3200 * S
	b.VX = math.Max(-lim, math.Min(lim, b.VX))
	b.VY = math.Max(-lim, math.Min(lim, b.VY))
	if math.Hypot(b.VX, b.VY) > 900*S {
		b.Say([]string{"WHEEEEE!", "I CAN FLY!", "AGAIN! AGAIN!", "WOOOAH!"}[rand.Intn(4)])
	}
}

func (b *Buddy) Update(dt, cx, cy, userIdle float64) {
	dt = math.Max(0.001, math.Min(0.05, dt))
	S := float64(b.S)
	b.t += dt
	floor, half := b.B, 36*S

	if b.blinkIn -= dt; b.blinkIn < 0 {
		b.blinkT, b.blinkIn = 0.13, 1.5+rand.Float64()*4
	}
	b.blinkT -= dt
	b.happy -= dt
	b.squash *= math.Exp(-dt * 9)
	if b.text != "" {
		if b.textAge += dt; b.textAge > b.textDur {
			b.text = ""
		}
	}
	ex, ey := cx-b.X, cy-(b.Y-36*S)
	d := math.Hypot(ex, ey) + 120*S
	k := math.Min(1, dt*12)
	b.lookX += (ex/d - b.lookX) * k
	b.lookY += (ey/d - b.lookY) * k
	b.face *= math.Exp(-dt * 6)

	switch b.State {
	case Drag:
		nx, ny := cx+b.offX, math.Min(cy+b.offY, floor)
		b.VX += ((nx-b.X)/dt - b.VX) * 0.3
		b.VY += ((ny-b.Y)/dt - b.VY) * 0.3
		b.moved += math.Abs(nx-b.X) + math.Abs(ny-b.Y)
		b.X, b.Y = nx, ny
	case Air:
		b.VY += 2000 * S * dt
		b.X += b.VX * dt
		b.Y += b.VY * dt
		if b.X < b.L+half {
			b.X, b.VX = b.L+half, math.Abs(b.VX)*0.6
		}
		if b.X > b.R-half {
			b.X, b.VX = b.R-half, -math.Abs(b.VX)*0.6
		}
		if b.Y < b.T+70*S {
			b.Y, b.VY = b.T+70*S, math.Abs(b.VY)*0.4
		}
		if b.Y >= floor {
			b.Y = floor
			if b.VY > 320*S {
				b.squash = math.Min(0.4, b.VY/(3500*S))
				b.spawn(2, 6)
				b.VY, b.VX = -b.VY*0.42, b.VX*0.7
			} else {
				b.VX, b.VY, b.State, b.idleT, b.squash = 0, 0, Idle, 0, 0.18
			}
		}
	case Walk:
		dir := 1.0
		if b.walkTo < b.X {
			dir = -1
		}
		b.face = dir
		b.X += dir * 95 * S * dt
		if (b.walkTo-b.X)*dir <= 0 {
			b.X, b.State, b.idleT = b.walkTo, Idle, 0
		}
	case Idle:
		b.idleT += dt
		home := b.homeX()
		away := math.Abs(b.X-home) > 2
		switch {
		case away && b.idleT > b.awayWait:
			b.walkTo, b.State = home, Walk
		case !away && userIdle > 120:
			b.State, b.zT, b.text = Sleep, 0.5, ""
		case !away:
			if b.nextAct -= dt; b.nextAct < 0 {
				b.nextAct = 25 + rand.Float64()*45
				switch rand.Intn(10) {
				case 0, 1, 2, 3:
					b.Hop(300)
				case 4, 5, 6:
					dir := 1.0
					if b.HomeRight {
						dir = -1
					}
					b.walkTo, b.State, b.awayWait = home+dir*(60+rand.Float64()*200)*S, Walk, 4
				default:
					b.Quip()
				}
			}
		}
	case Sleep:
		if b.zT -= dt; b.zT < 0 {
			b.zT = 1.3
			b.spawn(1, 1)
		}
		if userIdle < 1 {
			b.State, b.idleT = Idle, 0
			b.Say("*YAWN* OH! WELCOME BACK.")
			b.Hop(260)
		}
	}
	// keep inside the work area if it changed
	if b.State != Drag && b.State != Air {
		b.X = math.Max(b.L+half, math.Min(b.R-half, b.X))
		if b.Y > floor {
			b.Y = floor
		} else if b.Y < floor-1 {
			b.State = Air
		}
	}
	if now := time.Now(); now.Hour() != b.lastHour {
		b.lastHour = now.Hour()
		if b.State != Sleep {
			b.Say(fmt.Sprintf("IT'S %02d:00! QUICK STRETCH?", now.Hour()))
		}
	}
	n := 0
	for _, p := range b.parts {
		p.life -= dt
		if p.kind == 2 {
			p.vy += 500 * S * dt
		}
		if p.kind == 0 {
			p.vx = math.Sin(b.t*5+p.max*p.y) * 14 * S
		}
		p.x += p.vx * dt
		p.y += p.vy * dt
		if p.life > 0 {
			b.parts[n] = p
			n++
		}
	}
	b.parts = b.parts[:n]
}

func (b *Buddy) anchor() (ax, ay int, right bool) {
	right = b.X > (b.L+b.R)/2
	ax = 48 * b.S
	if right {
		ax = (canvasW - 48) * b.S
	}
	return ax, (canvasH - 10) * b.S, right
}

// WinPos returns the top-left screen position of the canvas window.
func (b *Buddy) WinPos() (int, int) {
	ax, ay, _ := b.anchor()
	return int(math.Round(b.X)) - ax, int(math.Round(b.Y)) - ay
}

var body = [13]string{
	"...oo......oo...",
	"..oooooooooooo..",
	".oooooooooooooo.",
	".oooooooooooooo.",
	"oooooooooooooooo",
	"oooooooooooooooo",
	"oooooooooooooooo",
	"oooooooooooooooo",
	".oooooooooooooo.",
	".oooooooooooooo.",
	"..oooooooooooo..",
	"..oo.oo..oo.oo..",
	"..oo.oo..oo.oo..",
}

const (
	cOutline = 0xFF3A2018
	cDark    = 0xFF2B1812
	cLight   = 0xFFEB9A76
	cBase    = 0xFFD97757
	cShade   = 0xFFC2623F
	cLeg     = 0xFFA85235
	cPaper   = 0xFFFBF6EC
)

func (b *Buddy) Draw(c *Canvas) {
	c.Clear()
	S := b.S
	Sf := float64(S)
	ax, ay, right := b.anchor()
	P := 4.0 * Sf

	// squash & stretch
	sq := b.squash
	if b.State == Air {
		sq -= math.Min(0.18, math.Abs(b.VY)/(9000*Sf))
	} else if b.State == Sleep {
		sq += 0.05 + 0.04*math.Sin(b.t*1.6)
	} else {
		sq += 0.025 * math.Sin(b.t*2.6)
	}
	sx, sy := 1+sq*0.8, 1-sq
	cell := func(fx, fy, fw, fh float64, col uint32) {
		x0 := int(math.Round(float64(ax) + (fx-8)*P*sx))
		x1 := int(math.Round(float64(ax) + (fx+fw-8)*P*sx))
		y0 := int(math.Round(float64(ay) - (14-fy)*P*sy))
		y1 := int(math.Round(float64(ay) - (14-fy-fh)*P*sy))
		c.Rect(x0, y0, x1-x0, y1-y0, col)
	}

	// ground shadow
	if h := b.B - b.Y; h < 60*Sf {
		f := 1 - h/(60*Sf)
		w := int(26 * Sf * (0.5 + 0.5*f) * sx)
		a := uint32(70 * f)
		c.Rect(ax-w, ay-2*S, 2*w, 4*S, a<<24)
		c.Rect(ax-w+3*S, ay-3*S, 2*w-6*S, 6*S, a<<24)
	}

	// legs animation
	step := -1
	if b.State == Walk {
		step = int(b.t*9) & 1
	}
	filled := func(x, y int) bool {
		if x < 0 || y < 0 || x > 15 || y > 12 || body[y][x] != 'o' {
			return false
		}
		if y == 12 && step >= 0 {
			leg := 0
			if x > 4 {
				leg = 1
			}
			if x > 8 {
				leg = 2
			}
			if x > 11 {
				leg = 3
			}
			return leg&1 != step
		}
		return true
	}
	for y := -1; y <= 13; y++ {
		for x := -1; x <= 16; x++ {
			fx, fy := float64(x), float64(y)
			if !filled(x, y) {
				if filled(x-1, y) || filled(x+1, y) || filled(x, y-1) || filled(x, y+1) {
					cell(fx, fy, 1, 1, cOutline)
				}
				continue
			}
			col := uint32(cBase)
			switch {
			case y <= 2:
				col = cLight
			case y >= 11:
				col = cLeg
			case y >= 9:
				col = cShade
			}
			cell(fx, fy, 1, 1, col)
		}
	}
	cell(2, 2, 2, 1, 0xFFF6BFA3) // highlight
	// cheeks
	cell(1.5, 7.4, 1.6, 1, 0xB0F49C9C)
	cell(12.9, 7.4, 1.6, 1, 0xB0F49C9C)

	// antenna spark
	bob := math.Sin(b.t*3) * 0.3
	cell(7.75, -1.6, 0.5, 1.6, cOutline)
	glow := 0.75 + 0.25*math.Sin(b.t*5)
	gold := fade(0xFFFFD166, glow)
	cell(7.5, -3.6+bob, 1, 1, 0xFFFFF3C4)
	cell(7.5, -4.6+bob, 1, 1, gold)
	cell(7.5, -2.6+bob, 1, 1, gold)
	cell(6.5, -3.6+bob, 1, 1, gold)
	cell(8.5, -3.6+bob, 1, 1, gold)

	// eyes
	fo := b.face * 0.8
	for _, exx := range []float64{3 + fo, 10 + fo} {
		switch {
		case b.State == Sleep || b.blinkT > 0:
			cell(exx, 5.4, 3, 0.8, cDark)
		case b.happy > 0:
			cell(exx, 5.6, 1, 1, cDark)
			cell(exx+1, 4.6, 1, 1, cDark)
			cell(exx+2, 5.6, 1, 1, cDark)
		default:
			cell(exx, 4, 3, 3, 0xFFFFFFFF)
			px := exx + 0.5 + b.lookX*0.9
			py := 4.5 + b.lookY*0.9
			cell(px, py, 2, 2, cDark)
			cell(px+0.2, py+0.2, 0.7, 0.7, 0xFFFFFFFF)
		}
	}
	// mouth
	switch {
	case b.happy > 0:
		cell(6.5+fo, 7.6, 3, 1.3, cDark)
		cell(7+fo, 8.2, 2, 0.7, 0xFFE85D75)
	case b.text != "" && b.textAge < float64(len(b.text))/38 && int(b.t*9)&1 == 0:
		cell(7+fo, 7.8, 2, 1.3, cDark)
	case b.State == Sleep:
		cell(7.5+fo, 8, 1, 1, cDark)
	default:
		cell(7+fo, 8, 2, 0.6, cDark)
	}

	// particles
	for _, p := range b.parts {
		f := math.Min(1, p.life/p.max*2)
		x, y := ax+int(p.x), ay+int(p.y)
		switch p.kind {
		case 0:
			for r, row := range []string{".##.##.", "#######", "#######", ".#####.", "..###..", "...#..."} {
				for k, ch := range row {
					if ch == '#' {
						c.Rect(x+k*2*S, y+r*2*S, 2*S, 2*S, fade(0xFFFF5C7A, f))
					}
				}
			}
		case 1:
			sz := S * (1 + int((p.max-p.life)*0.8))
			for r, row := range []string{"#####", "...#.", "..#..", ".#...", "#####"} {
				for k, ch := range row {
					if ch == '#' {
						c.Rect(x+k*sz, y+r*sz, sz, sz, fade(0xFFBFD7FF, f))
					}
				}
			}
		case 2:
			c.Rect(x, y, 2*S, 2*S, fade(0xFFFFE08A, f))
		}
	}

	// speech bubble
	if b.text != "" {
		f := math.Min(1, (b.textDur-b.textAge)/0.3)
		pop := math.Min(1, b.textAge/0.12)
		lines := wrap(b.text, 24)
		maxLen := 0
		for _, l := range lines {
			maxLen = max(maxLen, textW(l))
		}
		px := 2 * S
		w := maxLen*px - px + 14*S
		h := len(lines)*12*S + 10*S
		bot := ay - 88*S + int((1-pop)*6*Sf)
		x := 4 * S
		if right {
			x = canvasW*S - 4*S - w
		}
		y := bot - h
		ink, paper := fade(cOutline, f), fade(cPaper, f)
		c.Rect(x+2*S, y, w-4*S, h, ink)
		c.Rect(x, y+2*S, w, h-4*S, ink)
		tx := ax - 4*S
		tx = max(x+8*S, min(x+w-16*S, tx))
		for i := 0; i < 3; i++ { // tail
			c.Rect(tx+i*2*S-2*S, bot+i*2*S, (10-i*4+4)*S, 2*S, ink)
		}
		if f >= 1 {
			c.Rect(x+2*S, y+2*S, w-4*S, h-4*S, paper)
			for i := 0; i < 2; i++ {
				c.Rect(tx+i*2*S, bot-2*S+i*2*S, (10-i*4)*S, 2*S, paper)
			}
		}
		shown := int(b.textAge * 38)
		for i, l := range lines {
			if shown <= 0 {
				break
			}
			if shown < len(l) {
				l = l[:shown]
			}
			shown -= len(l) + 1
			col := uint32(cOutline)
			if f < 1 {
				col = fade(cPaper, f)
			}
			c.text(x+7*S, y+6*S+i*12*S, px, l, col)
		}
	}
}
