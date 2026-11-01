package report_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/lidisoft/horizon/internal/report"
)

func TestFlowReports(t *testing.T) {
	f, svc, _ := sprintFixture(t)
	ctx := context.Background()
	wc := f.wc(f.owner)
	week := report.Request{Projects: []string{"HZN"}, From: "2026-11-02", To: "2026-11-06"}

	cfd, err := svc.CumulativeFlow(ctx, wc, week)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, s := range cfd.Series {
		got[s.Key] = vals(s.Values)
	}
	want := map[string]string{"completed": "0 0 0 1 1", "started": "0 1 1 0 0", "unstarted": "4 3 3 3 3", "abandoned": "0 0 0 0 0"}
	if fmt.Sprint(got) != fmt.Sprint(want) || cfd.Series[0].Key != "completed" {
		t.Errorf("cfd = %v", got)
	}
	byCat := week
	byCat.By = "category"
	cfd, err = svc.CumulativeFlow(ctx, wc, byCat)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range cfd.Series {
		names = append(names, s.Name+":"+vals(s.Values))
	}
	if fmt.Sprint(names) != "[Done:0 0 0 1 1 In progress:0 1 1 0 0 Backlog:4 3 3 3 3 Todo:0 0 0 0 0 Canceled:0 0 0 0 0]" {
		t.Errorf("cfd by category = %v", names)
	}

	ct, err := svc.CycleTime(ctx, wc, report.Request{Projects: []string{"HZN"}})
	if err != nil || len(ct.Items) != 1 || ct.Items[0].Key != "HZN-1" || *ct.Items[0].CycleDays != 2 || ct.Items[0].LeadDays != 3 || *ct.Cycle.P50 != 2 {
		t.Errorf("cycle time = %+v, %v", ct, err)
	}

	tp, err := svc.Throughput(ctx, wc, report.Request{Projects: []string{"HZN"}, From: "2026-11-02", Metric: report.MetricEstimate})
	if err != nil || fmt.Sprint(tp.Periods) != "[2026-11-02]" || vals(tp.Series[0].Values) != "3" || tp.Average != 3 {
		t.Errorf("throughput = %+v, %v", tp, err)
	}
	cr, err := svc.CreatedResolved(ctx, wc, report.Request{Projects: []string{"HZN"}, From: "2026-11-02"})
	if err != nil || vals(cr.Series[0].Values) != "4" || vals(cr.Series[1].Values) != "1" {
		t.Errorf("created vs resolved = %+v, %v", cr, err)
	}

	d, err := svc.Distribution(ctx, wc, report.Request{Projects: []string{"HZN"}, Group: "type", Metric: report.MetricEstimate})
	if err != nil || len(d.Groups) != 1 || d.Groups[0].Name != "Task" || d.Groups[0].Value != 11 {
		t.Errorf("distribution = %+v, %v", d, err)
	}
	d, err = svc.Distribution(ctx, wc, report.Request{Projects: []string{"HZN"}})
	if err != nil || d.Group != "assignee" || len(d.Groups) != 1 || d.Groups[0].Key != "" || d.Groups[0].Value != 3 {
		t.Errorf("by assignee = %+v, %v", d, err)
	}
	if _, err := svc.Distribution(ctx, wc, report.Request{Group: "colour"}); err == nil {
		t.Error("unknown group accepted")
	}
}
