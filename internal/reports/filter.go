package reports

import (
	"strings"
	"time"
	_ "time/tzdata"
)

// NormalizeFilter validates exact date syntax and a maximum of366 WIB dates.
func NormalizeFilter(k Kind, f Filter, now time.Time) (Filter, Window, error) {
	var w Window
	if k != Summary && k != Methods && k != Products && k != Categories {
		return f, w, ErrInvalid
	}
	loc, _ := time.LoadLocation("Asia/Jakarta")
	if f.StartDate == "" && f.EndDate == "" {
		f.StartDate = now.In(loc).Format("2006-01-02")
		f.EndDate = f.StartDate
	}
	a, e := time.ParseInLocation("2006-01-02", f.StartDate, loc)
	if e != nil {
		return f, w, ErrInvalid
	}
	b, e := time.ParseInLocation("2006-01-02", f.EndDate, loc)
	if e != nil || b.Before(a) || b.AddDate(0, 0, 1).Sub(a) > 366*24*time.Hour {
		return f, w, ErrInvalid
	}
	if f.Page == 0 {
		f.Page = 1
	}
	if f.PageSize == 0 {
		f.PageSize = 20
	}
	if f.Page < 1 || f.Page > 1000000 || f.PageSize < 1 || f.PageSize > 100 {
		return f, w, ErrInvalid
	}
	f.Query = strings.TrimSpace(f.Query)
	if len([]rune(f.Query)) > 200 || ((k == Summary || k == Methods) && f.Query != "") {
		return f, w, ErrInvalid
	}
	if f.Sort == "" {
		f.Sort = "quantity_desc"
	}
	if f.Sort != "quantity_desc" && f.Sort != "net_desc" && f.Sort != "name_asc" {
		return f, w, ErrInvalid
	}
	w = Window{Start: a.UTC(), EndExclusive: b.AddDate(0, 0, 1).UTC(), StartDate: f.StartDate, EndDate: f.EndDate, Timezone: "Asia/Jakarta"}
	return f, w, nil
}
func inWindow(t time.Time, w Window) bool { return !t.Before(w.Start) && t.Before(w.EndExclusive) }
