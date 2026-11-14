// Fable Buddy - simulation and software renderer (platform independent).
package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
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
	'%': {5, 1, 2, 4, 5}, '+': {0, 2, 7, 2, 0}, '/': {1, 1, 2, 4, 4}, '(': {1, 2, 2, 2, 1},
	')': {4, 2, 2, 2, 4}, '<': {1, 2, 4, 2, 1}, '>': {4, 2, 1, 2, 4}, '=': {0, 7, 0, 7, 0},
	'"': {5, 5, 0, 0, 0},
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
	Busy // doing an activity
)

const (
	canvasW = 240 // in scale units
	canvasH = 190
)

type particle struct {
	x, y, vx, vy, life, max float64
	kind                    int // see the particle drawing code in Draw
	col                     uint32
	g                       float64
}

type Buddy struct {
	S          int    // integer UI scale
	Areas      []Area // work area of every screen, left to right
	HomeScreen int    // index into Areas; -1 = main screen
	Paused     bool   // e.g. while the right-click menu is open
	leap       bool
	leapArea   int
	act        *Act
	actT       float64
	lastAct    int
	X, Y       float64 // feet centre in screen pixels
	VX, VY     float64
	State      int
	HomeRight  bool

	t, idleT, awayWait, nextAct float64
	blinkIn, blinkT, happy      float64
	squash, lookX, lookY        float64
	walkTo, face                float64
	offX, offY, moved, zT       float64
	sleepF                      float64 // 0 awake .. 1 tucked in
	text                        string
	textAge, textDur            float64
	parts                       []particle
	pets, lastHour              int
	started                     time.Time
}

// Area is the usable part of one screen (taskbar excluded), in screen pixels.
type Area struct {
	L, T, R, B float64
	Primary    bool
}

func NewBuddy(scale int, areas []Area, homeScreen int, homeRight bool) *Buddy {
	bd := &Buddy{S: scale, HomeRight: homeRight, HomeScreen: homeScreen, blinkIn: 2, nextAct: 12, started: time.Now()}
	bd.SetAreas(areas)
	a := bd.Areas[bd.HomeIdx()]
	bd.X = bd.homeX()
	bd.Y = a.T + (a.B-a.T)*0.55 // drop in from above
	bd.State = Air
	bd.lastHour = time.Now().Hour()
	bd.Say(pick(greetingLines(time.Now())))
	return bd
}

// SetAreas updates the screen layout (any number of screens, any arrangement).
func (b *Buddy) SetAreas(as []Area) {
	if len(as) == 0 {
		return
	}
	as = append([]Area(nil), as...)
	sort.SliceStable(as, func(i, j int) bool {
		if as[i].L != as[j].L {
			return as[i].L < as[j].L
		}
		return as[i].T < as[j].T
	})
	old := len(b.Areas)
	b.Areas = as
	if old > 0 && old != len(as) {
		if len(as) > old {
			b.Say(fmt.Sprintf("OOH, A NEW SCREEN! NOW WE HAVE %d.", len(as)))
		} else {
			b.Say("A SCREEN WENT AWAY. I'LL STAY OVER HERE.")
		}
		b.idleT, b.awayWait = 0, 1
	}
}

func outside(v, lo, hi float64) float64 {
	if v < lo {
		return lo - v
	}
	if v > hi {
		return v - hi
	}
	return 0
}

// areaAt returns the screen a point belongs to (the nearest one if it is in a gap).
func (b *Buddy) areaAt(x, y float64) int {
	best, bestD := 0, math.Inf(1)
	for i, a := range b.Areas {
		d := 3*outside(x, a.L, a.R) + outside(y, a.T, a.B) + math.Abs(a.B-y)*1e-6
		if d < bestD {
			best, bestD = i, d
		}
	}
	return best
}

// areaHolding returns the screen containing the point, or -1.
func (b *Buddy) areaHolding(x, y float64) int {
	for i, a := range b.Areas {
		if x >= a.L && x <= a.R && y >= a.T && y <= a.B {
			return i
		}
	}
	return -1
}

func (b *Buddy) cur() Area { return b.Areas[b.areaAt(b.X, b.Y)] }

// HomeIdx is the index of the screen the buddy lives on.
func (b *Buddy) HomeIdx() int {
	if b.HomeScreen >= 0 && b.HomeScreen < len(b.Areas) {
		return b.HomeScreen
	}
	for i, a := range b.Areas {
		if a.Primary {
			return i
		}
	}
	return 0
}

func (b *Buddy) homeX() float64 {
	a := b.Areas[b.HomeIdx()]
	m := 60 * float64(b.S)
	if b.HomeRight {
		return a.R - m
	}
	return a.L + m
}

// ScreenName is a human label for screen i, e.g. "Screen 2 - right (main)".
func (b *Buddy) ScreenName(i int) string {
	a := b.Areas[i]
	name := fmt.Sprintf("Screen %d", i+1)
	if len(b.Areas) == 2 {
		o := b.Areas[1-i]
		dx := (a.L+a.R)/2 - (o.L+o.R)/2
		dy := (a.T+a.B)/2 - (o.T+o.B)/2
		switch {
		case math.Abs(dx) >= math.Abs(dy) && dx < 0:
			name += " - left"
		case math.Abs(dx) >= math.Abs(dy):
			name += " - right"
		case dy < 0:
			name += " - top"
		default:
			name += " - bottom"
		}
	}
	if a.Primary {
		name += " (main)"
	}
	return name
}

// BodyRect is the buddy's body (antenna included) in screen pixels.
func (b *Buddy) BodyRect() (x0, y0, x1, y1 int) {
	S := float64(b.S)
	return int(b.X - 40*S), int(b.Y - 80*S), int(b.X + 40*S), int(b.Y + 2*S)
}

func (b *Buddy) Hush() { b.text = "" }

// sayHook lets tests see everything the buddy says.
var sayHook func(string)

func (b *Buddy) Say(s string) {
	if sayHook != nil {
		sayHook(s)
	}
	b.text = strings.ToUpper(s)
	b.textAge = 0
	b.textDur = 2.5 + float64(len(s))*0.07
}

func pick(s []string) string { return s[rand.Intn(len(s))] }

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
	"I'M SMALL BUT I'M MIGHTY.",
	"YOUR DESKTOP IS MY PLAYGROUND.",
	"FUN FACT: OCTOPUSES HAVE THREE HEARTS.",
	"FUN FACT: HONEY NEVER SPOILS.",
	"FUN FACT: A DAY ON VENUS IS LONGER THAN ITS YEAR.",
	"FUN FACT: BANANAS ARE BERRIES. STRAWBERRIES AREN'T.",
	"FUN FACT: OTTERS HOLD HANDS WHEN THEY SLEEP.",
	"HAVE YOU TRIED TURNING IT OFF AND ON AGAIN?",
	"POSTURE CHECK! SIT UP STRAIGHT.",
	"BLINK A FEW TIMES. YOUR EYES WILL THANK YOU.",
	"A SNACK WOULDN'T HURT.",
	"PROGRESS, NOT PERFECTION.",
	"I'D HELP YOU TYPE BUT MY LEGS ARE TOO SHORT.",
	"CLOSE A FEW TABS. JUST A FEW.",
	"I'M NOT LAZY, I'M IN ENERGY-SAVING MODE.",
	"YOU + ME = BEST TEAM.",
	"LEAVE ME ALONE FOR 2 MINUTES AND I'LL NAP.",
	"TODAY IS A GOOD DAY TO FINISH SOMETHING.",
	"I TRIED COUNTING YOUR PIXELS. I GAVE UP.",
	"MUSIC MAKES EVERYTHING BETTER.",
	"WHEN IN DOUBT, TAKE A WALK.",
	"BEEP BOOP. JUST CHECKING IN.",
	"<3",
	"IF YOU'RE STUCK, EXPLAIN IT TO ME. I'M A GREAT LISTENER.",
	"ROME WASN'T BUILT IN A DAY. BUT THEY WERE LAYING BRICKS EVERY HOUR.",
	"DONE IS BETTER THAN PERFECT.",
	"I LIKE IT HERE.",
}

var petLines = []string{
	"MORE PETS, PLEASE!", "PURR... WAIT, AM I A CAT?", "YAY!", "YOU'RE MY FAVORITE HUMAN.",
	"HEHE!", "BEST. DAY. EVER.", "<3 <3 <3", "AGAIN!", "THAT'S THE SPOT!", "I FEEL SO LOVED.",
}

var throwLines = []string{
	"WHEEEEE!", "I CAN FLY!", "AGAIN! AGAIN!", "WOOOAH!", "TO INFINITY!",
	"AIRMAIL!", "I'M A BIRD NOW.", "NO BRAKES!", "THIS IS FINE.",
}

var wakeLines = []string{
	"*YAWN* OH! WELCOME BACK.", "I WASN'T SLEEPING. I WAS RESTING MY EYES.",
	"YOU'RE BACK! I MISSED YOU.", "*STRETCH* READY WHEN YOU ARE.", "HUH? WHAT? OH, HI!",
	"I HAD THE BEST DREAM ABOUT PIXELS.",
}

var visitLines = []string{
	"BRB, CHECKING OUT THE OTHER SCREEN!", "FIELD TRIP!", "WHAT'S OVER HERE?",
	"JUST VISITING. BACK SOON!", "NEW SCREEN, WHO DIS?",
}

var homeLines = []string{"BACK TO MY CORNER!", "HOME SWEET HOME.", "OKAY, GOING HOME."}

// screenLines each take the number of screens.
var screenLines = []string{
	"%d SCREENS? SO MUCH ROOM TO RUN!",
	"I CAN SEE ALL %d OF YOUR SCREENS FROM HERE.",
	"%d SCREENS. YOU MUST BE A PRO.",
	"RIGHT-CLICK ME TO PICK WHICH OF YOUR %d SCREENS I LIVE ON.",
	"THROW ME TO ANOTHER SCREEN. ALL %d ARE MY HOME.",
}

// hourLines each take the hour.
var hourLines = []string{
	"IT'S %02d:00! QUICK STRETCH?", "%02d:00 ALREADY. WATER BREAK?", "DING! IT'S %02d:00.",
}

func greetingLines(now time.Time) []string {
	switch h := now.Hour(); {
	case h < 5:
		return []string{"STILL UP? I'LL KEEP YOU COMPANY.", "NIGHT OWL MODE: ON."}
	case h < 12:
		return []string{"GOOD MORNING! LET'S BUILD SOMETHING.", "MORNING! COFFEE FIRST?", "RISE AND SHINE!"}
	case h < 18:
		return []string{"GOOD AFTERNOON! I'M RIGHT HERE.", "HELLO AGAIN! WHAT ARE WE DOING?"}
	default:
		return []string{"GOOD EVENING! NICE TO SEE YOU.", "EVENING! LET'S KEEP IT CHILL."}
	}
}

func timeLines(now time.Time) []string {
	var l []string
	switch now.Weekday() {
	case time.Monday:
		l = append(l, "MONDAY AGAIN? WE GOT THIS.")
	case time.Wednesday:
		l = append(l, "HALFWAY THROUGH THE WEEK!")
	case time.Friday:
		l = append(l, "IT'S FRIDAY! ALMOST THERE.")
	case time.Saturday, time.Sunday:
		l = append(l, "WORKING ON A WEEKEND? RESPECT.", "WEEKEND VIBES.")
	}
	switch h := now.Hour(); {
	case h >= 23 || h < 5:
		l = append(l, "IT'S LATE. SLEEP IS A FEATURE TOO.")
	case h >= 11 && h < 13:
		l = append(l, "LUNCH TIME SOON?")
	case h >= 15 && h < 17:
		l = append(l, "AFTERNOON SLUMP? HAVE SOME WATER.")
	}
	return append(l, "HAPPY "+strings.ToUpper(now.Weekday().String())+"!")
}

func (b *Buddy) Quip() {
	now := time.Now()
	switch r := rand.Intn(10); {
	case r == 0:
		b.Say(now.Format("IT'S 15:04. TIME FLIES!"))
	case r == 1:
		m := int(time.Since(b.started).Minutes())
		b.Say(fmt.Sprintf("WE'VE HUNG OUT FOR %d MIN. YOU PETTED ME %d TIMES.", m, b.pets))
	case r == 2 && len(b.Areas) > 1:
		b.Say(fmt.Sprintf(pick(screenLines), len(b.Areas)))
	case r == 3:
		b.Say(pick(timeLines(now)))
	default:
		b.Say(pick(quips))
	}
}

func (b *Buddy) Hop(power float64) {
	if b.State == Drag {
		return
	}
	b.VY = -power * float64(b.S)
	b.State, b.leap = Air, false
}

func (b *Buddy) wakeForMove() {
	b.idleT, b.awayWait = 0, 0.3
	if b.State == Sleep {
		b.State = Idle
	}
}

func (b *Buddy) SetHome(right bool) {
	b.HomeRight = right
	b.wakeForMove()
	b.Say("MOVING! BE RIGHT THERE.")
}

func (b *Buddy) SetHomeScreen(i int) {
	b.HomeScreen = i
	b.wakeForMove()
	b.Say("OFF TO MY NEW SCREEN!")
}

// LeapTo jumps in an arc to a floor point, possibly on another screen.
func (b *Buddy) LeapTo(tx, ty float64) {
	S := float64(b.S)
	g := 2000 * S
	dx, dy := tx-b.X, ty-b.Y
	T := math.Max(0.7, math.Min(1.6, 0.5+math.Hypot(dx, dy)/(2400*S)))
	b.VX, b.VY = dx/T, dy/T-0.5*g*T
	b.State, b.leap, b.leapArea = Air, true, b.areaAt(tx, ty)
	b.face = math.Copysign(1, dx)
}

func (b *Buddy) spawn(kind, n int) {
	S := float64(b.S)
	for i := 0; i < n; i++ {
		p := particle{kind: kind, x: (rand.Float64()*40 - 20) * S, y: -50 * S}
		switch kind {
		case 0:
			p.vx, p.vy, p.max = (rand.Float64()*40-20)*S, -(40+rand.Float64()*30)*S, 1.15+rand.Float64()*0.3
		case 1:
			p.x, p.y = (10+rand.Float64()*4)*S, -56*S
			p.vx, p.vy, p.max = 4*S, -16*S, 2.2
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
	b.State, b.leap, b.act = Drag, false, nil
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
		if b.Y >= b.cur().B-1 {
			b.VY = -330 * S
		}
		switch {
		case b.pets == 1:
			b.Say("HEHE! THAT TICKLES.")
		case b.pets%25 == 0:
			b.Say(fmt.Sprintf("%d PETS! I'M THE LUCKIEST BUDDY.", b.pets))
		case rand.Intn(3) == 0:
			b.Say(pick(petLines))
		}
		return
	}
	lim := 3200 * S
	b.VX = math.Max(-lim, math.Min(lim, b.VX))
	b.VY = math.Max(-lim, math.Min(lim, b.VY))
	if math.Hypot(b.VX, b.VY) > 900*S {
		b.Say(pick(throwLines))
	}
}

func (b *Buddy) Update(dt, cx, cy, userIdle float64) {
	dt = math.Max(0.001, math.Min(0.05, dt))
	S := float64(b.S)
	b.t += dt
	half := 36 * S
	ci := b.areaAt(b.X, b.Y)
	cur := b.Areas[ci]

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
	if b.State == Sleep {
		b.sleepF = math.Min(1, b.sleepF+dt*2.5)
	} else {
		b.sleepF = math.Max(0, b.sleepF-dt*5)
	}

	switch b.State {
	case Drag:
		nx, ny := cx+b.offX, cy+b.offY
		ny = math.Min(ny, b.Areas[b.areaAt(nx, ny)].B)
		b.VX += ((nx-b.X)/dt - b.VX) * 0.3
		b.VY += ((ny-b.Y)/dt - b.VY) * 0.3
		b.moved += math.Abs(nx-b.X) + math.Abs(ny-b.Y)
		b.X, b.Y = nx, ny
	case Air:
		b.VY += 2000 * S * dt
		nx, ny := b.X+b.VX*dt, b.Y+b.VY*dt
		if !b.leap {
			// screen edges are walls, unless another screen continues there
			if nx < cur.L+half && b.areaHolding(cur.L-1, ny-20*S) < 0 {
				nx, b.VX = cur.L+half, math.Abs(b.VX)*0.6
			}
			if nx > cur.R-half && b.areaHolding(cur.R+1, ny-20*S) < 0 {
				nx, b.VX = cur.R-half, -math.Abs(b.VX)*0.6
			}
			if ny < cur.T+70*S && b.areaHolding(nx, cur.T-1) < 0 {
				ny, b.VY = cur.T+70*S, math.Abs(b.VY)*0.4
			}
		}
		// land on the floor we just crossed: the current screen's, or the
		// next one's if we moved over; a leap only lands on its target
		oy := b.Y
		b.X, b.Y = nx, ny
		cands := []int{ci, b.areaAt(b.X, b.Y)}
		if b.leap {
			cands = []int{b.leapArea}
		}
		landed := -1
		for _, i := range cands {
			a := b.Areas[min(i, len(b.Areas)-1)]
			if b.VY > 0 && oy <= a.B+1 && b.Y >= a.B && b.X >= a.L && b.X <= a.R {
				landed = i
				break
			}
		}
		if landed >= 0 {
			b.Y = b.Areas[min(landed, len(b.Areas)-1)].B
			switch {
			case b.leap:
				b.leap = false
				b.VX, b.VY, b.State, b.idleT, b.squash = 0, 0, Idle, 0, 0.3
				b.spawn(2, 8)
			case b.VY > 320*S:
				b.squash = math.Min(0.4, b.VY/(3500*S))
				b.spawn(2, 6)
				b.VY, b.VX = -b.VY*0.42, b.VX*0.7
			default:
				b.VX, b.VY, b.State, b.idleT, b.squash = 0, 0, Idle, 0, 0.18
			}
		}
		b.rescue()
	case Walk:
		if b.Paused {
			b.State = Idle
			break
		}
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
		if b.Paused {
			break
		}
		hi := b.HomeIdx()
		home := b.homeX()
		away := ci != hi || math.Abs(b.X-home) > 2
		switch {
		case away && b.idleT > b.awayWait:
			if ci == hi && math.Abs(b.X-home) < 500*S {
				b.walkTo, b.State = home, Walk
			} else {
				b.LeapTo(home, b.Areas[hi].B)
				if rand.Intn(2) == 0 {
					b.Say(pick(homeLines))
				}
			}
		case !away && userIdle > 120:
			b.State, b.zT, b.text = Sleep, 0.5, ""
		case !away:
			if b.nextAct -= dt; b.nextAct < 0 {
				b.nextAct = 18 + rand.Float64()*35
				switch r := rand.Intn(20); r {
				case 12, 13:
					b.Hop(300)
				case 14, 15:
					dir := 1.0
					if b.HomeRight {
						dir = -1
					}
					to := home + dir*(60+rand.Float64()*200)*S
					b.walkTo = math.Max(cur.L+half, math.Min(cur.R-half, to))
					b.State, b.awayWait = Walk, 4
				case 16:
					if len(b.Areas) < 2 {
						b.Quip()
						break
					}
					j := (hi + 1 + rand.Intn(len(b.Areas)-1)) % len(b.Areas)
					a := b.Areas[j]
					b.LeapTo(a.L+100*S+rand.Float64()*math.Max(0, a.R-a.L-200*S), a.B)
					b.Say(pick(visitLines))
					b.awayWait = 6
				case 17, 18, 19:
					b.Quip()
				default:
					b.StartActivity(-1)
				}
			}
		}
	case Busy:
		b.actT += dt
		if b.act.Emit != nil {
			b.act.Emit(b, b.actT, dt)
		}
		if b.actT >= b.act.Dur {
			b.State, b.act, b.idleT = Idle, nil, 0
		}
	case Sleep:
		if b.zT -= dt; b.zT < 0 {
			b.zT = 0.75
			b.spawn(1, 1)
		}
		if userIdle < 1 {
			b.State, b.idleT = Idle, 0
			b.Say(pick(wakeLines))
			b.Hop(260)
		}
	}
	// stay on a screen if the layout changed under us
	if b.State == Idle || b.State == Walk || b.State == Sleep || b.State == Busy {
		a := b.Areas[b.areaAt(b.X, b.Y)]
		b.X = math.Max(a.L+half, math.Min(a.R-half, b.X))
		if b.Y > a.B {
			b.Y = a.B
		} else if b.Y < a.B-1 {
			b.State = Air
		}
	}
	if now := time.Now(); now.Hour() != b.lastHour {
		b.lastHour = now.Hour()
		if b.State != Sleep {
			b.Say(fmt.Sprintf(pick(hourLines), now.Hour()))
		}
	}
	n := 0
	for _, p := range b.parts {
		p.life -= dt
		if p.kind == 2 {
			p.vy += 500 * S * dt
		}
		p.vy += p.g * dt
		if p.kind == 4 || p.kind == 6 || p.kind == 10 {
			p.x += math.Sin(p.life*5+p.max*13) * 10 * S * dt
		}
		if p.kind == 0 {
			p.vx = math.Sin(p.life*6+p.max*7) * 14 * S
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

// rescue puts the buddy back home if it somehow left every screen.
func (b *Buddy) rescue() {
	S := float64(b.S)
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, a := range b.Areas {
		lo, hi = math.Min(lo, a.T), math.Max(hi, a.B)
	}
	if b.Y > hi+1500*S || b.Y < lo-5000*S {
		a := b.Areas[b.HomeIdx()]
		b.leap, b.VX, b.VY = false, 0, 0
		b.X, b.Y = b.homeX(), a.T+(a.B-a.T)/2
	}
}

func (b *Buddy) anchor() (ax, ay int, right bool) {
	a := b.cur()
	right = b.X > (a.L+a.R)/2
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
	dir := 1.0
	if right {
		dir = -1
	}
	d := &dc{c: c, u: 2 * Sf, ax: float64(ax), ay: float64(ay), dir: dir}
	var act *Act
	pz := pose{}
	if b.State == Busy && b.act != nil {
		act = b.act
		if act.Pose != nil {
			act.Pose(b.actT, &pz)
		}
	}
	bx, by := float64(ax)+dir*pz.ox*2*Sf, float64(ay)+pz.oy*2*Sf

	// squash & stretch
	sq := b.squash
	if b.State == Air {
		sq -= math.Min(0.18, math.Abs(b.VY)/(9000*Sf))
	} else if b.State == Sleep {
		sq += 0.05 + 0.04*math.Sin(b.t*1.6)
	} else {
		sq += 0.025 * math.Sin(b.t*2.6)
	}
	sq += pz.sq
	sx, sy := 1+sq*0.8, 1-sq
	cell := func(fx, fy, fw, fh float64, col uint32) {
		x0 := int(math.Round(bx + (fx-8)*P*sx))
		x1 := int(math.Round(bx + (fx+fw-8)*P*sx))
		y0 := int(math.Round(by - (14-fy)*P*sy))
		y1 := int(math.Round(by - (14-fy-fh)*P*sy))
		c.Rect(x0, y0, x1-x0, y1-y0, col)
	}

	// ground shadow
	if h := b.cur().B - b.Y; h < 60*Sf && !pz.noShadow {
		f := 1 - h/(60*Sf)
		w := int(26 * Sf * (0.5 + 0.5*f) * sx)
		a := uint32(70 * f)
		c.Rect(ax-w, ay-2*S, 2*w, 4*S, a<<24)
		c.Rect(ax-w+3*S, ay-3*S, 2*w-6*S, 6*S, a<<24)
	}

	// pillows (behind the body)
	sl := b.sleepF
	if sl > 0 {
		rise := (1 - sl) * 3
		pil := func(x, y, w, h float64, col, hi uint32) {
			y += rise
			cell(x+1, y-0.5, w-2, h+1, fade(cOutline, sl))
			cell(x-0.5, y+1, w+1, h-2, fade(cOutline, sl))
			cell(x, y+0.5, w, h-1, fade(cOutline, sl))
			cell(x+1, y, w-2, h, fade(col, sl))
			cell(x, y+1, w, h-2, fade(col, sl))
			cell(x+1.5, y+0.6, w-5, 0.8, fade(hi, sl))
		}
		pil(-2.4, 5.5, 20.8, 7.5, 0xFFB9A7F0, 0xFFD5C9FA) // big lilac pillow
		pil(-1.6, 0.2, 19.2, 8, 0xFFFDF8EE, 0xFFFFFFFF)   // cream pillow on top
	}

	if act != nil && act.Back != nil {
		act.Back(d, b.actT)
	}

	// legs animation
	step := -1
	if b.State == Walk || pz.walk {
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
	em := pz.eyes
	for ei, exx := range []float64{3 + fo, 10 + fo} {
		outer := (ei == 0) == (dir > 0)
		switch {
		case sl > 0.5:
			// handled by the eye mask below
		case em == eClosed || em == eWink && outer:
			cell(exx, 5.4, 3, 0.8, cDark)
		case em == eHappy:
			cell(exx, 5.6, 1, 1, cDark)
			cell(exx+1, 4.6, 1, 1, cDark)
			cell(exx+2, 5.6, 1, 1, cDark)
		case em == eDizzy:
			for _, q := range [][2]float64{{0, 4}, {2, 4}, {1, 5}, {0, 6}, {2, 6}} {
				cell(exx+q[0], q[1], 1, 1, cDark)
			}
		case em == eWide:
			cell(exx, 4, 3, 3, 0xFFFFFFFF)
			cell(exx+1+pz.lx*dir*0.6, 5+pz.ly*0.6, 1, 1, cDark)
		case (em == eLook || em == eWink) && b.blinkT <= 0:
			cell(exx, 4, 3, 3, 0xFFFFFFFF)
			px := exx + 0.5 + pz.lx*dir*0.9
			py := 4.5 + pz.ly*0.9
			cell(px, py, 2, 2, cDark)
			cell(px+0.2, py+0.2, 0.7, 0.7, 0xFFFFFFFF)
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
	if sl > 0.5 { // eye mask
		mf := (sl - 0.5) * 2
		navy, lash := fade(0xFF3B3F8F, mf), fade(0xFFC9CEFF, mf)
		cell(0, 4.9, 16, 1.1, fade(0xFF2A2D6B, mf)) // strap
		for _, mx := range []float64{2.2, 9.2} {
			cell(mx+0.6, 3.6, 3.4, 3.8, navy)
			cell(mx, 4.2, 4.6, 2.6, navy)
			cell(mx+1, 5.6, 0.7, 0.6, lash) // stitched sleepy eye
			cell(mx+1.7, 6, 1.2, 0.6, lash)
			cell(mx+2.9, 5.6, 0.7, 0.6, lash)
		}
	}
	// mouth
	switch {
	case pz.mouth == mHide:
	case pz.mouth == mO:
		cell(7.25+fo, 7.5, 1.5, 1.5, cDark)
	case pz.mouth == mBig:
		cell(6.5+fo, 7.3, 3, 2, cDark)
		cell(7+fo, 8.4, 2, 0.9, 0xFFE85D75)
	case pz.mouth == mSmile:
		cell(6.5+fo, 7.6, 3, 1.3, cDark)
		cell(7+fo, 8.2, 2, 0.7, 0xFFE85D75)
	case pz.mouth == mFlat:
		cell(6.5+fo, 8, 3, 0.5, cDark)
	case pz.mouth == mChew:
		if int(b.t*7)&1 == 0 {
			cell(7+fo, 7.7, 2, 1.2, cDark)
		} else {
			cell(6.8+fo, 8.1, 2.4, 0.5, cDark)
		}
	case b.happy > 0:
		cell(6.5+fo, 7.6, 3, 1.3, cDark)
		cell(7+fo, 8.2, 2, 0.7, 0xFFE85D75)
	case b.text != "" && b.textAge < float64(len(b.text))/38 && int(b.t*9)&1 == 0:
		cell(7+fo, 7.8, 2, 1.3, cDark)
	case b.State == Sleep:
		cell(7.5+fo, 7.3, 1, 0.5+0.4*(1+math.Sin(b.t*1.6)), cDark)
	default:
		cell(7+fo, 8, 2, 0.6, cDark)
	}

	if act != nil {
		if act.Front != nil {
			act.Front(d, b.actT)
		}
		for _, a := range pz.arms {
			d.arm(a[0]+pz.ox, a[1]+pz.oy)
		}
	}

	// blanket (slides up over the body when falling asleep)
	if sl > 0 {
		top := 14.4 - 5.6*sl
		bh := 14.4 - top
		cell(-2.2, top-0.5, 20.4, bh+0.5, cOutline)
		cell(-2.7, top+0.5, 21.4, bh-0.5, cOutline)
		cell(-1.7, top, 19.4, bh, 0xFF4F86E8)
		cell(-2.2, top+1, 20.4, bh-1, 0xFF4F86E8)
		cell(-1.7, top, 19.4, math.Min(1.3, bh), 0xFFE6F0FF) // folded edge
		for i := 0; i < 9; i++ {                             // little star pattern
			px, py := -0.8+float64(i)*2.2, top+2.1+float64(i%2)*1.6
			if py+0.7 < 14.2 {
				cell(px, py, 0.7, 0.7, 0xFFFFD166)
			}
		}
		if bh > 3 {
			cell(-1.7, 13.4, 19.4, 0.6, 0xFF3A6BC4) // shaded hem
		}
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
			x += int(math.Sin(p.life*4) * 2 * Sf)
			for r, row := range []string{"####", "..#.", ".#..", "####"} {
				for k, ch := range row {
					if ch == '#' {
						c.Rect(x+k*S, y+r*S, S, S, fade(0xFFD6E4FF, f))
					}
				}
			}
		case 2:
			c.Rect(x, y, 2*S, 2*S, fade(0xFFFFE08A, f))
		case 3: // music note
			for r, row := range []string{"..##", "..#.", "..#.", "###.", "##.."} {
				for k, ch := range row {
					if ch == '#' {
						c.Rect(x+k*S, y+r*S, S, S, fade(p.col, f))
					}
				}
			}
		case 4: // soap bubble
			for r, row := range []string{".##.", "#..#", "#..#", ".##."} {
				for k, ch := range row {
					if ch == '#' {
						c.Rect(x+k*S, y+r*S, S, S, fade(0xFFBFEFFF, f))
					}
				}
			}
			c.Rect(x+S, y+S, S, S, fade(0xFFFFFFFF, f))
		case 5: // steam / smoke
			c.Rect(x, y, 2*S, 2*S, fade(p.col, f*0.5))
		case 6: // confetti
			c.Rect(x, y, 2*S, S+S*(int(p.life*9)&1), fade(p.col, f))
		case 7: // rain
			c.Rect(x, y, S, 3*S, fade(0xFF9CC8FF, f))
		case 8: // twinkle star
			c.Rect(x+S, y, S, 3*S, fade(p.col, f))
			c.Rect(x, y+S, 3*S, S, fade(p.col, f))
		case 9: // drop
			c.Rect(x, y, S, 2*S, fade(p.col, f))
		case 10: // snow
			c.Rect(x, y, S+S/2, S+S/2, fade(0xFFFFFFFF, f))
		case 11: // shooting star
			c.Rect(x, y, 2*S, 2*S, fade(0xFFFFF3C4, f))
			c.Rect(x+2*S, y-S, 6*S, S, fade(0x80FFD166, f))
		default: // dot
			c.Rect(x, y, 2*S, 2*S, fade(p.col, f))
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
		bot := ay - 88*S + int((1-pop)*6*Sf) + int(math.Min(0, pz.oy)*2*Sf)
		bot = max(bot, h+2*S) // stay on the canvas when the buddy is up high
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
