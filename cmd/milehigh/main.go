// milehigh: your flight log in the terminal. Shares backups with Skyline on Android.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lemonade1921/milehigh/internal/data"
	"github.com/lemonade1921/milehigh/internal/model"
	"github.com/lemonade1921/milehigh/internal/online"
	"github.com/lemonade1921/milehigh/internal/stats"
	"github.com/lemonade1921/milehigh/internal/store"
	"github.com/lemonade1921/milehigh/internal/tui"
)

var version = "dev"

const usage = `milehigh — your flight log in the terminal

Usage:
  milehigh                      open the app
  milehigh import FILE          import a Skyline backup (.json) or a Flighty CSV export
  milehigh export [FILE|-]      write a backup Skyline on Android can restore
  milehigh add FLIGHT DATE [FROM TO]
                                add a flight, e.g.  milehigh add "VY 1234" 2026-11-04
  milehigh list [-year N]       list flights
  milehigh stats [-year N]      print your stats
  milehigh config [KEY [VALUE]] show or change settings (name, units, timeFormat, temp, scheduleKey)
  milehigh path                 show where the log is stored
  milehigh version

Options:
  -data DIR   use another log folder (or set MILEHIGH_HOME), e.g. one synced with Syncthing
`

func main() {
	online.Version = version
	fs := flag.NewFlagSet("milehigh", flag.ExitOnError)
	dir := fs.String("data", "", "log folder")
	year := fs.Int("year", 0, "year")
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	args := os.Args[1:]
	// Allow flags anywhere: pull them out first.
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if (a == "-data" || a == "--data" || a == "-year" || a == "--year") && i+1 < len(args) {
			_ = fs.Set(strings.TrimLeft(a, "-"), args[i+1])
			i++
			continue
		}
		if strings.HasPrefix(a, "--data=") || strings.HasPrefix(a, "-data=") || strings.HasPrefix(a, "--year=") || strings.HasPrefix(a, "-year=") {
			kv := strings.SplitN(strings.TrimLeft(a, "-"), "=", 2)
			_ = fs.Set(kv[0], kv[1])
			continue
		}
		if a == "-h" || a == "--help" || a == "help" {
			fmt.Print(usage)
			return
		}
		rest = append(rest, a)
	}
	if len(rest) > 0 && (rest[0] == "version" || rest[0] == "--version" || rest[0] == "-v") {
		fmt.Println("milehigh", version)
		return
	}
	st, err := store.Open(*dir)
	if err != nil {
		fail(err)
	}
	if len(rest) == 0 {
		if err := tui.Run(st); err != nil {
			fail(err)
		}
		return
	}
	switch rest[0] {
	case "import":
		need(rest, 2)
		msg, err := tui.ImportFile(st, rest[1])
		if err != nil {
			fail(err)
		}
		fmt.Println(msg)
	case "export":
		b, err := st.ExportBackup()
		if err != nil {
			fail(err)
		}
		path := fmt.Sprintf("skyline-backup-%s.json", time.Now().Format("2006-01-02"))
		if len(rest) > 1 {
			path = rest[1]
		}
		if path == "-" {
			os.Stdout.Write(append(b, '\n'))
			return
		}
		if err := os.WriteFile(path, b, 0o644); err != nil {
			fail(err)
		}
		fmt.Printf("Saved %d flights to %s\n", len(st.Flights()), path)
	case "add":
		add(st, rest[1:])
	case "list", "ls":
		list(st, *year)
	case "stats":
		printStats(st, *year)
	case "config":
		config(st, rest[1:])
	case "path":
		fmt.Println(st.Path())
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", rest[0], usage)
		os.Exit(2)
	}
}

func need(args []string, n int) {
	if len(args) < n {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "milehigh:", err)
	os.Exit(1)
}

func add(st *store.Store, args []string) {
	if len(args) < 2 {
		fail(fmt.Errorf("usage: milehigh add FLIGHT DATE [FROM TO]"))
	}
	al, num, ok := online.SplitNumber(args[0])
	if !ok {
		fail(fmt.Errorf("%q doesn't look like a flight number", args[0]))
	}
	d, err := model.ParseDate(args[1])
	if err != nil {
		fail(fmt.Errorf("date must look like 2026-11-04"))
	}
	f := &model.Flight{ID: model.NewID(), Airline: al, Number: num, Date: d, Source: "manual"}
	if len(args) >= 4 {
		f.From, f.To = strings.ToUpper(args[2]), strings.ToUpper(args[3])
	} else {
		r, err := online.LookupRoute(al, num)
		if err != nil {
			fail(fmt.Errorf("couldn't find the route of %s %s (%v); give it as: milehigh add %s %s FROM TO", al, num, err, args[0], args[1]))
		}
		f.From, f.To = r.From, r.To
	}
	if err := st.Put(f); err != nil {
		fail(err)
	}
	fmt.Printf("Added %s %s %s → %s on %s\n", al, num, f.From, f.To, d.Format("Mon 2 Jan 2006"))
}

func list(st *store.Store, year int) {
	s := st.Prefs().Settings()
	for _, f := range st.Flights() {
		if year != 0 && f.Date.Year() != year {
			continue
		}
		dist := f.DistanceKm()
		unit := "km"
		if s.Units == "MI" {
			dist /= stats.KmPerMile
			unit = "mi"
		}
		mark := " "
		if f.Canceled {
			mark = "✗"
		} else if !f.HasLanded(time.Now()) {
			mark = "›"
		}
		fmt.Printf("%s %s  %-8s %s → %s  %6.0f %s  %s\n", mark, f.Date, strings.TrimSpace(f.Airline+" "+f.Number),
			f.From, f.ArrivalAirport(), dist, unit, data.AirlineName(f.Airline))
	}
}

func printStats(st *store.Store, year int) {
	s := stats.Compute(st.Flights(), year, time.Now())
	set := st.Prefs().Settings()
	dist, unit := s.DistanceKm, "km"
	if set.Units == "MI" {
		dist, unit = dist/stats.KmPerMile, "mi"
	}
	label := "All time"
	if year != 0 {
		label = fmt.Sprint(year)
	}
	fmt.Printf("%s\n  %d flights · %.0f %s (%.2f× around the Earth) · %.0f h in the air\n", label, s.Flights, dist, unit,
		s.DistanceKm/stats.EarthCircumferenceKm, s.Airtime.Hours())
	fmt.Printf("  %d airports · %d airlines · %d countries · %d continents\n", len(s.Airports), len(s.Airlines), len(s.Countries), len(s.Continents))
	show := func(title string, cs []stats.Count) {
		if len(cs) == 0 {
			return
		}
		fmt.Printf("\n  %s\n", title)
		for i, c := range cs {
			if i == 5 {
				break
			}
			name := c.Key
			if c.Label != "" && c.Label != c.Key {
				name += "  " + c.Label
			}
			fmt.Printf("    %-34s %d\n", name, c.N)
		}
	}
	show("Airports", s.Airports)
	show("Airlines", s.Airlines)
	show("Routes", s.Routes)
	show("Countries", s.Countries)
	show("Aircraft", s.Aircraft)
}

func config(st *store.Store, args []string) {
	s := st.Prefs().Settings()
	vals := map[string]*string{"name": &s.Name, "units": &s.Units, "timeFormat": &s.TimeFormat, "temp": &s.Temp, "scheduleKey": &s.ScheduleKey}
	keys := []string{"name", "units", "timeFormat", "temp", "scheduleKey"}
	if len(args) == 0 {
		for _, k := range keys {
			v := *vals[k]
			if k == "scheduleKey" && v != "" {
				v = "(set)"
			}
			fmt.Printf("%-12s %s\n", k, v)
		}
		return
	}
	p, ok := vals[args[0]]
	if !ok {
		fail(fmt.Errorf("unknown setting %q (one of %s)", args[0], strings.Join(keys, ", ")))
	}
	if len(args) == 1 {
		fmt.Println(*p)
		return
	}
	v := strings.Join(args[1:], " ")
	allowed := map[string][]string{"units": {"KM", "MI"}, "timeFormat": {"SYSTEM", "H24", "H12"}, "temp": {"C", "F"}}
	if a, ok := allowed[args[0]]; ok {
		v = strings.ToUpper(v)
		good := false
		for _, x := range a {
			good = good || x == v
		}
		if !good {
			fail(fmt.Errorf("%s must be one of %s", args[0], strings.Join(a, ", ")))
		}
	}
	*p = v
	if err := st.UpdatePrefs(func(pr *store.Prefs) { pr.PutSettings(s) }); err != nil {
		fail(err)
	}
}
