package agent

import (
	"strings"
	"testing"
	"time"
)

// fakeHC replays scripted `hc wait` results and records every call.
type fakeHC struct {
	waits  []string // JSON for successive `hc wait` calls
	screen string   // what `hc read` returns
	calls  []string
}

func (f *fakeHC) Run(args ...string) ([]byte, error) {
	f.calls = append(f.calls, strings.Join(args, " "))
	switch args[0] {
	case "run":
		return []byte(`{"address":"t9","state":"live"}`), nil
	case "wait":
		if len(f.waits) == 0 {
			return []byte(`{"ended":false,"reason":"timeout","turn":1}`), nil
		}
		w := f.waits[0]
		f.waits = f.waits[1:]
		return []byte(w), nil
	case "read":
		return []byte(f.screen), nil
	}
	return nil, nil
}

func (f *fakeHC) called(prefix string) bool {
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

func newSession(hc *fakeHC) *Session {
	return &Session{
		HC: hc, Name: "agent-x", Dir: "/wt", Argv: []string{"claude", "hi"},
		Labels: map[string]string{"b": "2", "a": "1"}, Timeout: time.Hour, Poll: time.Minute,
		Log: func(string, ...any) {}, sleep: func(time.Duration) {},
	}
}

func TestClassify(t *testing.T) {
	one := 1
	cases := []struct {
		w    waitResult
		want state
	}{
		{waitResult{Reason: "timeout"}, stateWorking},
		{waitResult{Ended: true, Reason: "status", Status: "idle", Turn: 0}, stateStarting},
		{waitResult{Ended: true, Reason: "status", Status: "idle", Turn: 2}, stateDone},
		{waitResult{Ended: true, Reason: "status", Status: "done", Turn: 1}, stateDone},
		{waitResult{Ended: true, Reason: "status", Status: "blocked", Turn: 1}, stateBlocked},
		{waitResult{Ended: true, Reason: "status", Status: "error", Turn: 1}, stateError},
		{waitResult{Ended: true, ExitCode: &one}, stateExited},
		{waitResult{Ended: true, Reason: "exit"}, stateExited},
	}
	for _, c := range cases {
		if got := classify(c.w); got != c.want {
			t.Errorf("classify(%+v) = %v, want %v", c.w, got, c.want)
		}
	}
}

func TestSessionStart_PassesLabelsSortedAndArgvLast(t *testing.T) {
	hc := &fakeHC{}
	addr, err := newSession(hc).Start()
	if err != nil || addr != "t9" {
		t.Fatalf("Start = %q, %v", addr, err)
	}
	want := "run --bg --json --outlive-parent --name agent-x --cwd /wt --label a=1 --label b=2 claude hi"
	if hc.calls[0] != want {
		t.Errorf("hc call\n got %q\nwant %q", hc.calls[0], want)
	}
}

func TestSessionWait_DoneKillsTerminal(t *testing.T) {
	hc := &fakeHC{waits: []string{
		`{"ended":false,"reason":"timeout","turn":0}`,
		`{"ended":true,"reason":"status","status":"working","turn":0}`,
		`{"ended":true,"reason":"status","status":"done","turn":1}`,
	}}
	out, err := newSession(hc).Wait("t9")
	if err != nil || out.Status != "done" {
		t.Fatalf("Wait = %+v, %v", out, err)
	}
	if !hc.called("kill t9") {
		t.Error("a finished session should be closed")
	}
}

func TestSessionWait_StartupPromptIsAnswered(t *testing.T) {
	hc := &fakeHC{
		screen: "Quick safety check ...\n ❯ No, exit\n   Yes, I trust this folder",
		waits: []string{
			`{"ended":false,"reason":"timeout","turn":0}`,
			`{"ended":true,"reason":"status","status":"done","turn":1}`,
		},
	}
	if _, err := newSession(hc).Wait("t9"); err != nil {
		t.Fatal(err)
	}
	if !hc.called("keys t9 down enter") {
		t.Errorf("trust prompt was not answered; calls: %v", hc.calls)
	}
}

func TestSessionWait_BlockedIsLeftOpen(t *testing.T) {
	hc := &fakeHC{
		screen: "Which database should I migrate? (1) prod (2) staging",
		waits:  []string{`{"ended":true,"reason":"status","status":"blocked","turn":1}`},
	}
	out, err := newSession(hc).Wait("t9")
	if err != nil || out.Status != "blocked" {
		t.Fatalf("Wait = %+v, %v", out, err)
	}
	if hc.called("kill") || hc.called("keys") {
		t.Errorf("a blocked session must be left for a person; calls: %v", hc.calls)
	}
}

func TestSessionWait_UnknownQuestionBeforeFirstTurnIsBlocked(t *testing.T) {
	hc := &fakeHC{
		screen: "Some other question?",
		waits:  []string{`{"ended":true,"reason":"status","status":"blocked","turn":0}`},
	}
	out, _ := newSession(hc).Wait("t9")
	if out.Status != "blocked" || hc.called("keys") {
		t.Errorf("Wait = %+v; calls %v", out, hc.calls)
	}
}

func TestSessionWait_TimeoutKills(t *testing.T) {
	hc := &fakeHC{}
	s := newSession(hc)
	s.Timeout = -time.Second
	out, err := s.Wait("t9")
	if err != nil || out.Status != "timeout" || !hc.called("kill t9") {
		t.Errorf("Wait = %+v, %v; calls %v", out, err, hc.calls)
	}
}

func TestSessionWait_ExitCode(t *testing.T) {
	hc := &fakeHC{waits: []string{`{"ended":true,"exit_code":3,"reason":"exit","turn":1}`}}
	out, _ := newSession(hc).Wait("t9")
	if out.Status != "exited" || out.ExitCode != 3 {
		t.Errorf("Wait = %+v", out)
	}
}

func TestSessionWait_PollsFasterBeforeFirstTurn(t *testing.T) {
	hc := &fakeHC{waits: []string{
		`{"ended":false,"reason":"timeout","turn":0}`,
		`{"ended":false,"reason":"timeout","turn":1}`,
		`{"ended":true,"reason":"status","status":"done","turn":1}`,
	}}
	s := newSession(hc)
	s.Poll = 5 * time.Minute
	if _, err := s.Wait("t9"); err != nil {
		t.Fatal(err)
	}
	var waits []string
	for _, c := range hc.calls {
		if strings.HasPrefix(c, "wait") {
			waits = append(waits, c)
		}
	}
	if waits[0] != "wait t9 --json --timeout 15" || waits[2] != "wait t9 --json --timeout 300" {
		t.Errorf("wait calls = %v", waits)
	}
}
