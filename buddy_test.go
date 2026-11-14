package main

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"
	"time"
)

// Two side-by-side screens: a big main one and a smaller one placed lower.
var twoScreens = []Area{
	{0, 0, 1920, 1040, true},
	{1920, 200, 3200, 920, false},
}

func sim(b *Buddy, sec float64) {
	for t := 0.0; t < sec; t += 1.0 / 60 {
		b.Update(1.0/60, 0, 0, 0)
	}
}

func quiet(b *Buddy) *Buddy { b.nextAct = math.Inf(1); return b }

func TestEveryPhraseRenders(t *testing.T) {
	var all []string
	for _, l := range [][]string{quips, petLines, throwLines, wakeLines, visitLines, homeLines} {
		all = append(all, l...)
	}
	for _, f := range screenLines {
		all = append(all, fmt.Sprintf(f, 3))
	}
	for _, f := range hourLines {
		all = append(all, fmt.Sprintf(f, 9))
	}
	for d := 0; d < 7; d++ {
		for h := 0; h < 24; h++ {
			now := time.Date(2026, 11, 9+d, h, 0, 0, 0, time.Local)
			all = append(all, greetingLines(now)...)
			all = append(all, timeLines(now)...)
		}
	}
	all = append(all, "OOH, A NEW SCREEN! NOW WE HAVE 3.", "A SCREEN WENT AWAY. I'LL STAY OVER HERE.",
		"OFF TO MY NEW SCREEN!", "MOVING! BE RIGHT THERE.", "HEHE! THAT TICKLES.")
	for _, s := range all {
		for _, ch := range strings.ToUpper(s) {
			if _, ok := font[ch]; !ok && ch != ' ' {
				t.Errorf("%q: no glyph for %q", s, ch)
			}
		}
	}
}

func TestLeapsToChosenScreenAndBack(t *testing.T) {
	b := quiet(NewBuddy(2, twoScreens, -1, true))
	sim(b, 3)
	if b.areaAt(b.X, b.Y) != 0 || b.State != Idle {
		t.Fatalf("should start on the main screen, got area %d state %d", b.areaAt(b.X, b.Y), b.State)
	}
	b.SetHomeScreen(1)
	sim(b, 5)
	if b.areaAt(b.X, b.Y) != 1 || b.State != Idle || math.Abs(b.X-(3200-120)) > 3 || b.Y != 920 {
		t.Fatalf("did not reach screen 2 corner: x=%v y=%v state=%d", b.X, b.Y, b.State)
	}
	b.SetHomeScreen(0)
	sim(b, 5)
	if b.areaAt(b.X, b.Y) != 0 || b.Y != 1040 || b.State != Idle {
		t.Fatalf("did not come back: x=%v y=%v state=%d", b.X, b.Y, b.State)
	}
}

func TestThrowAcrossScreensThenGoHome(t *testing.T) {
	b := quiet(NewBuddy(2, twoScreens, -1, true))
	sim(b, 3)
	b.Down(b.X, b.Y-50)
	b.Up()
	b.Y = 600 // the user lifted it up before flinging
	b.State, b.VX, b.VY = Air, 2500*2, -900*2
	visited := false
	for i := 0; i < 60*15; i++ {
		b.Update(1.0/60, 0, 0, 0)
		if b.areaAt(b.X, b.Y) == 1 {
			visited = true
		}
	}
	if !visited {
		t.Fatal("throw never reached the second screen")
	}
	if b.areaAt(b.X, b.Y) != 0 || b.State != Idle || math.Abs(b.X-b.homeX()) > 3 {
		t.Fatalf("did not return home: x=%v y=%v state=%d", b.X, b.Y, b.State)
	}
}

func TestLowThrowHitsTheStepBetweenScreens(t *testing.T) {
	b := quiet(NewBuddy(2, twoScreens, -1, true))
	sim(b, 3)
	b.State, b.VX, b.VY = Air, 1500*2, -100*2 // too low to get over the 920 floor
	sim(b, 0.6)
	if b.X > 1920 {
		t.Fatalf("passed through the wall: x=%v y=%v", b.X, b.Y)
	}
}

func TestScreenUnplugged(t *testing.T) {
	b := quiet(NewBuddy(2, twoScreens, 1, true))
	sim(b, 4)
	b.SetAreas(twoScreens[:1])
	sim(b, 6)
	if b.State != Idle || b.Y != 1040 || math.Abs(b.X-(1920-120)) > 3 {
		t.Fatalf("not safely home on remaining screen: x=%v y=%v state=%d", b.X, b.Y, b.State)
	}
}

func TestStackedScreens(t *testing.T) {
	stacked := []Area{{0, -1080, 1920, 0, false}, {0, 0, 1920, 1040, true}}
	b := quiet(NewBuddy(2, stacked, 0, true))
	sim(b, 4)
	if b.Y != 0 || b.State != Idle {
		t.Fatalf("should stand on the bottom edge of the top screen: y=%v state=%d", b.Y, b.State)
	}
	if b.ScreenName(0) != "Screen 1 - top" || b.ScreenName(1) != "Screen 2 - bottom (main)" {
		t.Fatalf("names: %q %q", b.ScreenName(0), b.ScreenName(1))
	}
}

func TestPausedStaysPut(t *testing.T) {
	b := NewBuddy(2, twoScreens, -1, true)
	sim(b, 3)
	b.Paused, b.nextAct = true, 0
	x := b.X
	sim(b, 5)
	if b.X != x || b.State != Idle {
		t.Fatal("moved while the menu was open")
	}
}

func TestRandomThrowsAlwaysEndHome(t *testing.T) {
	layouts := [][]Area{
		{{0, 0, 1920, 1040, true}},
		twoScreens,
		{{-1280, 300, 0, 1024, false}, {0, 0, 2560, 1400, true}, {2560, -200, 3640, 1720, false}},
		{{0, -1080, 1920, 0, false}, {0, 0, 1920, 1040, true}},
		{{0, 0, 1920, 1040, true}, {2100, 0, 3000, 900, false}}, // gap between screens
	}
	r := rand.New(rand.NewSource(1))
	for li, l := range layouts {
		for n := 0; n < 40; n++ {
			b := quiet(NewBuddy(1+r.Intn(3), l, r.Intn(len(l)+1)-1, r.Intn(2) == 0))
			sim(b, 2+r.Float64()*3)
			b.Down(b.X, b.Y)
			b.Update(1.0/60, b.X+r.Float64()*600-300, b.Y-r.Float64()*600, 0)
			b.Up()
			b.VX, b.VY = r.Float64()*8000-4000, -r.Float64()*6000
			sim(b, 25)
			hi := b.HomeIdx()
			if b.State != Idle || b.areaAt(b.X, b.Y) != hi || math.Abs(b.X-b.homeX()) > 3 || b.Y != b.Areas[hi].B {
				t.Fatalf("layout %d run %d: lost at x=%.0f y=%.0f state=%d (home %d)", li, n, b.X, b.Y, b.State, hi)
			}
		}
	}
}

func TestActivities(t *testing.T) {
	if len(acts) < 50 {
		t.Fatalf("only %d activities", len(acts))
	}
	var said []string
	sayHook = func(s string) { said = append(said, s) }
	defer func() { sayHook = nil }()
	names := map[string]bool{}
	for i, a := range acts {
		if names[a.Name] {
			t.Errorf("duplicate activity %q", a.Name)
		}
		names[a.Name] = true
		for _, right := range []bool{true, false} {
			for _, S := range []int{1, 3} {
				b := quiet(NewBuddy(S, twoScreens, -1, right))
				sim(b, 3)
				c := &Canvas{W: canvasW * S, H: canvasH * S}
				c.Pix = make([]byte, c.W*c.H*4)
				b.StartActivity(i)
				if b.State != Busy {
					t.Fatalf("%s: did not start", a.Name)
				}
				x := b.X
				for k := 0; k < int(a.Dur*60)+5; k++ {
					b.Update(1.0/60, 0, 0, 0)
					b.Draw(c)
				}
				if b.State != Idle || b.X != x {
					t.Errorf("%s: did not end cleanly (state %d, moved %v)", a.Name, b.State, b.X-x)
				}
			}
		}
	}
	for _, s := range said {
		for _, ch := range strings.ToUpper(s) {
			if _, ok := font[ch]; !ok && ch != ' ' {
				t.Errorf("%q: no glyph for %q", s, ch)
			}
		}
	}
}

func TestDragInterruptsActivity(t *testing.T) {
	b := quiet(NewBuddy(2, twoScreens, -1, true))
	sim(b, 3)
	b.StartActivity(0)
	sim(b, 1)
	b.Down(b.X, b.Y-40)
	b.Update(1.0/60, b.X, b.Y-40, 0)
	b.Up()
	sim(b, 8)
	if b.State != Idle || b.act != nil {
		t.Fatalf("state %d after drag", b.State)
	}
}

func TestDoesActivitiesOnItsOwn(t *testing.T) {
	for pace, minActs := range []int{12, 6, 1} {
		b := NewBuddy(2, twoScreens, -1, true)
		b.Pace = pace
		started := 0
		var cur *Act
		for i := 0; i < 60*240; i++ { // four minutes with the user active
			b.Update(1.0/60, 500, 500, 0)
			if b.act != nil && b.act != cur {
				started++
			}
			cur = b.act
		}
		if started < minActs {
			t.Errorf("pace %d: only %d activities in 4 minutes", pace, started)
		}
		t.Logf("pace %d: %d activities in 4 minutes", pace, started)
	}
}
