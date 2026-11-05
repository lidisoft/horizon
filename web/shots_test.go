package web_test

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	cdppage "github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/testenv"
)

var shotsDir = flag.String("shots", "", "write wireframe and app screenshots, side by side, to this directory (TestWireframeShots)")

// A board of docs/wireframes and the app page that implements it. setup,
// when set, runs on the loaded page before the screenshot (opening a
// dialog, a panel).
type shot struct {
	board string
	props string // extra viewer props, e.g. "state=loading"
	path  string // app path under /w/lidisoft; empty: wireframe only
	setup string
	w, h  int64
}

var shots = []shot{
	{board: "Main", path: "/p/HZN/issues", w: 1440, h: 900},
	{board: "IssuePeek", path: "/p/HZN/issues", setup: `peek`, w: 1440, h: 900},
	{board: "IssueDetail", path: "/issues/HZN-1", w: 1440, h: 1100},
	{board: "Board", path: "/p/HZN/board", w: 1440, h: 900},
	{board: "Backlog", path: "/p/HZN/backlog", w: 1440, h: 1000},
	{board: "CommandPalette", path: "/p/HZN/issues", setup: `palette`, w: 1440, h: 900},
	{board: "Inbox", path: "/inbox", w: 1440, h: 900},
	{board: "States", props: "state=empty", path: "/p/WEB/issues", w: 1440, h: 900},
	{board: "States", props: "state=no-results", path: "/search?q=zzzz", w: 1440, h: 900},
	{board: "States", props: "state=loading", w: 1440, h: 900},
	{board: "Table", path: "/p/HZN/table", w: 1440, h: 900},
	{board: "Timeline", path: "/p/HZN/timeline", w: 1440, h: 900},
	{board: "Calendar", path: "/p/HZN/calendar", w: 1440, h: 1000},
	{board: "Overview", path: "/p/HZN", w: 1440, h: 1000},
	{board: "Report", path: "/p/HZN/reports", w: 1440, h: 1000},
	{board: "Workflow", path: "/settings/workflows", w: 1440, h: 900},
	{board: "IssueTypes", path: "/settings/issue-types", w: 1440, h: 900},
	{board: "Roles", path: "/settings/roles", w: 1440, h: 900},
	{board: "MoveIssue", path: "/issues/HZN-1", setup: `move`, w: 1440, h: 900},
	{board: "StatusCategories", path: "/settings/categories", w: 1440, h: 900},
	{board: "Labels", path: "/settings/labels", w: 1440, h: 900},
	{board: "Estimation", path: "/settings/estimation", w: 1440, h: 900},
	{board: "CalendarSettings", path: "/settings/calendar", w: 1440, h: 900},
	{board: "General", props: "scope=workspace", path: "/settings/general", w: 1440, h: 1000},
	{board: "General", props: "scope=project", path: "/settings/projects/HZN", w: 1440, h: 1000},
	{board: "Members", props: "view=members", path: "/settings/members", w: 1440, h: 900},
	{board: "IterationSettings", path: "/settings/iterations", w: 1440, h: 900},
	{board: "Sidebar", w: 1440, h: 900},
	{board: "SettingsNav", w: 1440, h: 900},
	{board: "ViewSwitch", w: 1440, h: 900},
	{board: "StatusIcon", w: 1440, h: 900},
	{board: "PriorityIcon", w: 1440, h: 900},
}

// TestWireframeShots screenshots each wireframe board next to the page that
// implements it, seeded with the boards' sample data, for comparing by eye:
//
//	go test ./web -run TestWireframeShots -args -shots /tmp/shots
//
// It writes <board>[-props].wire.png and .app.png; it checks nothing.
func TestWireframeShots(t *testing.T) {
	if *shotsDir == "" {
		t.Skip("-shots not set")
	}
	if err := os.MkdirAll(*shotsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	only := os.Getenv("SHOTS") // comma-separated board names, to redo a few

	wire := newPage(t)
	wireBase := testenv.Serve(t, http.FileServer(http.Dir("../docs/wireframes")))
	a := appServer(t)
	app := newPage(t)
	seedWireframeData(t, a, app)

	for _, s := range shots {
		if only != "" && !strings.Contains(","+only+",", ","+s.board+",") {
			continue
		}
		name := s.board
		if s.props != "" {
			name += "-" + strings.NewReplacer("=", "-", "&", "-").Replace(s.props)
		}
		t.Log("shot", name)
		wire.do(chromedp.EmulateViewport(s.w, s.h))
		wire.do(chromedp.Navigate(wireBase + "/index.html#/" + s.board + "?theme=light&" + s.props))
		wire.poll(`document.querySelector(".hz-frame")?.children.length && "board"`)
		time.Sleep(500 * time.Millisecond)
		var box [4]float64
		if err := json.Unmarshal([]byte(wire.poll(`(() => { const b = document.querySelector(".hz-frame").getBoundingClientRect();
			return JSON.stringify([b.x + scrollX, b.y + scrollY, b.width, b.height]); })()`)), &box); err != nil {
			t.Fatal(err)
		}
		save(t, filepath.Join(*shotsDir, name+".wire.png"), wire, clip(box))

		if s.path == "" {
			continue
		}
		app.do(chromedp.EmulateViewport(s.w, s.h))
		app.do(chromedp.Navigate(a.base + "/w/lidisoft" + s.path))
		app.poll(`document.querySelector("hzn-app")?.shadowRoot?.getElementById("outlet")?.firstElementChild?.shadowRoot?.childElementCount && "view"`)
		time.Sleep(time.Second)
		switch s.setup {
		case "peek":
			app.click(view + `.querySelector('.row[data-key="HZN-1"], [data-key="HZN-1"]')`)
		case "palette":
			app.eval(`document.activeElement?.blur()`)
			app.do(chromedp.KeyEvent("k", chromedp.KeyModifiers(chromedp.ModifierCtrl)))
		case "move":
			app.eval(`(() => { const d = ` + view + `.querySelector("hzn-issue-detail")?.shadowRoot ?? ` + view + `;
				d.querySelector('[data-action="move"]')?.click(); })()`)
		}
		time.Sleep(time.Second)
		save(t, filepath.Join(*shotsDir, name+".app.png"), app, chromedp.CaptureScreenshot())
	}
}

// clip captures a region of the page, beyond the viewport if need be.
func clip(box [4]float64) chromedp.Action[[]byte] {
	return func(ctx context.Context, t *chromedp.Target) ([]byte, error) {
		yes := true
		r, err := cdp.Call(ctx, t, cdppage.CaptureScreenshot, cdppage.CaptureScreenshotParams{
			Clip: &cdppage.Viewport{X: box[0], Y: box[1], Width: box[2], Height: box[3], Scale: 1}, CaptureBeyondViewport: &yes, FromSurface: &yes,
		})
		if err != nil {
			return nil, err
		}
		return r.Data, nil
	}
}

func save(t *testing.T, file string, p *page, shot chromedp.Action[[]byte]) {
	t.Helper()
	ctx, cancel := context.WithTimeout(p.ctx, 20*time.Second)
	defer cancel()
	buf, err := chromedp.Run(ctx, shot)
	if err != nil {
		t.Errorf("%s: %v", file, err)
		return
	}
	if err := os.WriteFile(file, buf, 0o644); err != nil {
		t.Fatal(err)
	}
}

// seedWireframeData fills the lidisoft workspace with the wireframes'
// sample data: three people, projects Horizon, Design system and Website,
// HZN issues in Backlog, Todo and In progress, labels and an active sprint.
// Jean signs in.
func seedWireframeData(t *testing.T, a app, pg *page) {
	t.Helper()
	ctx := context.Background()
	people := map[string]uuid.UUID{}
	for _, u := range []struct{ initials, email, name string }{
		{"JW", "jean@lidisoft.com", "Jean Wasilewski"}, {"RL", "romain@lidisoft.com", "Romain Libaert"}, {"MC", "maya@lidisoft.com", "Maya Chen"},
	} {
		created, err := a.ids.CreateUser(ctx, uuid.NullUUID{}, identity.NewUser{Email: u.email, Name: u.name, Password: testPassword, InstanceAdmin: u.initials == "JW"})
		if err != nil {
			t.Fatal(err)
		}
		people[u.initials] = created.ID
	}
	pg.do(chromedp.EmulateViewport(1440, 900))
	pg.do(chromedp.Navigate(a.base + "/"))
	pg.signIn("jean@lidisoft.com", testPassword)
	pg.typeInto(view+`.querySelector('[name="name"]')`, "LiDiSoft")
	pg.eval(view + `.querySelector("form").requestSubmit()`)
	pg.poll(`location.pathname === "/w/lidisoft" && "workspace"`)
	var ws struct{ ID uuid.UUID }
	_, body := pg.api("GET", "/w/lidisoft", "")
	_ = json.Unmarshal([]byte(body), &ws)
	for _, who := range []string{"RL", "MC"} {
		if err := a.ws.AddMember(ctx, ws.ID, people[who]); err != nil {
			t.Fatal(err)
		}
	}

	must := func(method, path, body string) string {
		t.Helper()
		status, out := pg.api(method, path, body)
		if status >= 300 {
			t.Fatalf("%s %s: %d %s", method, path, status, out)
		}
		return out
	}
	must("POST", "/w/lidisoft/projects", `{"key":"HZN","name":"Horizon","colour":"blue"}`)
	must("POST", "/w/lidisoft/projects", `{"key":"LDS","name":"Design system","colour":"purple"}`)
	must("POST", "/w/lidisoft/projects", `{"key":"WEB","name":"Website","colour":"green"}`)

	labels := map[string]string{}
	for name, colour := range map[string]string{"backend": "grey", "web": "blue", "bug": "coral", "realtime": "purple"} {
		var l struct{ ID string }
		_ = json.Unmarshal([]byte(must("POST", "/w/lidisoft/config/labels", `{"name":"`+name+`","colour":"`+colour+`"}`)), &l)
		labels[name] = l.ID
	}
	var resolved struct {
		IssueTypes []struct{ ID, Name string } `json:"issue_types"`
		Workflows  []struct {
			Statuses []struct{ ID, Name string } `json:"statuses"`
		} `json:"workflows"`
	}
	_ = json.Unmarshal([]byte(must("GET", "/w/lidisoft/projects/HZN/config/resolved", "")), &resolved)
	types, statuses := map[string]string{}, map[string]string{}
	for _, it := range resolved.IssueTypes {
		types[it.Name] = it.ID
	}
	for _, st := range resolved.Workflows[0].Statuses {
		statuses[st.Name] = st.ID
	}

	// The Main board's rows, in its order within each status.
	type row struct {
		title, status, typ, priority, estimate, who, start, due string
		labels                                                  []string
	}
	rows := []row{
		{"Outbox relay publishes domain events to JetStream", "In progress", "Story", "high", "5", "RL", "2026-10-28", "2026-11-04", []string{"backend"}},
		{"Workflow transition rules: conditions and validators", "In progress", "Story", "medium", "8", "JW", "2026-10-29", "2026-11-06", []string{"backend"}},
		{"Issue list keyboard navigation (j/k, x to select)", "In progress", "Task", "medium", "3", "MC", "", "", []string{"web"}},
		{"Rank issues with fractional index keys", "Todo", "Task", "high", "3", "JW", "2026-11-03", "2026-11-06", []string{"backend"}},
		{"WebSocket replay after reconnect drops events", "Todo", "Bug", "urgent", "2", "RL", "", "2026-10-30", []string{"bug", "realtime"}},
		{"Filter bar: chips, operators and @me placeholder", "Todo", "Story", "medium", "5", "MC", "", "", []string{"web"}},
		{"Status categories with outcomes in workflow editor", "Todo", "Story", "low", "5", "JW", "", "", []string{"web"}},
		{"Cascade delete confirmation lists descendants", "Backlog", "Task", "none", "", "MC", "", "", []string{"web"}},
		{"Report projections: status intervals table", "Backlog", "Story", "low", "8", "RL", "", "", []string{"backend"}},
		{"Working calendars with non-working dates", "Backlog", "Story", "none", "", "JW", "", "", nil},
	}
	var sprint []string
	for i, r := range rows {
		ids := []string{}
		for _, l := range r.labels {
			ids = append(ids, `"`+labels[l]+`"`)
		}
		body := fmt.Sprintf(`{"project":"HZN","type_id":%q,"title":%q,"priority":%q,"assignee_id":%q,"labels":[%s]`,
			types[r.typ], r.title, r.priority, people[r.who], strings.Join(ids, ","))
		if r.estimate != "" {
			body += `,"estimate":"` + r.estimate + `"`
		}
		if r.start != "" {
			body += `,"start_date":"` + r.start + `"`
		}
		if r.due != "" {
			body += `,"due_date":"` + r.due + `"`
		}
		must("POST", "/w/lidisoft/issues", body+"}")
		key := fmt.Sprintf("HZN-%d", i+1)
		path := map[string][]string{"Todo": {"Todo"}, "In progress": {"Todo", "In progress"}}[r.status]
		for _, st := range path {
			must("POST", "/w/lidisoft/issues/"+key+"/transition", `{"status_id":"`+statuses[st]+`"}`)
		}
		if r.status != "Backlog" {
			sprint = append(sprint, `"`+key+`"`)
		}
	}
	must("POST", "/w/lidisoft/issues", `{"project":"HZN","type_id":"`+types["Task"]+`","title":"Outbox table and relay worker"}`)
	for _, st := range []string{"Todo", "In progress", "Done"} {
		must("POST", "/w/lidisoft/issues/HZN-11/transition", `{"status_id":"`+statuses[st]+`"}`)
	}
	must("POST", "/w/lidisoft/issues/HZN-1/comments", `{"body":"Relay is in; replay after reconnect is HZN-5."}`)

	var it struct{ ID string }
	_ = json.Unmarshal([]byte(must("POST", "/w/lidisoft/projects/HZN/iterations",
		`{"name":"Sprint 12","goal":"Events reach the browser","start_date":"2026-10-28","end_date":"2026-11-10"}`)), &it)
	must("POST", "/w/lidisoft/projects/HZN/iterations/"+it.ID+"/issues", `{"add":[`+strings.Join(sprint, ",")+`,"HZN-11"]}`)
	must("POST", "/w/lidisoft/projects/HZN/iterations/"+it.ID+"/start", `{}`)

	must("POST", "/w/lidisoft/views", `{"project":"HZN","name":"Open bugs","layout":"list","shared":true,`+
		`"query":{"filter":{"field":"labels","op":"contains","value":["`+labels["bug"]+`"]}}}`)
	must("POST", "/w/lidisoft/views", `{"project":"HZN","name":"Due this week","layout":"list","shared":true,`+
		`"query":{"filter":{"field":"due_date","op":"within","value":"+7d"}}}`)

	// Romain comments on and reassigns Jean's issues: Jean's inbox fills.
	romain := newPage(t)
	romain.do(chromedp.Navigate(a.base + "/"))
	romain.signIn("romain@lidisoft.com", testPassword)
	for _, call := range [][3]string{
		{"POST", "/w/lidisoft/issues/HZN-5/comments", `{"body":"The client sends the last event id per topic, but replay starts after the newest one across topics. Can you check HZN-1 before we fix the cursor?"}`},
		{"POST", "/w/lidisoft/issues/HZN-7/assign", `{"assignee_id":"` + people["JW"].String() + `"}`},
		{"POST", "/w/lidisoft/issues/HZN-2/comments", `{"body":"Validators run before post-actions, right?"}`},
	} {
		if status, out := romain.api(call[0], call[1], call[2]); status >= 300 {
			t.Fatalf("%s %s: %d %s", call[0], call[1], status, out)
		}
	}
}
