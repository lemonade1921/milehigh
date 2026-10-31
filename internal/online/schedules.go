package online

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lemonade1921/milehigh/internal/model"
)

// Status is a flight as AeroDataBox knows it. Times are local airport times.
type Status struct {
	Airline, Number, From, To                  string
	DepScheduled, ArrScheduled                 *model.LocalTime
	DepRevised, ArrRevised                     *model.LocalTime // estimate before, actual after
	Departed, Arrived, Canceled                bool
	DepTerminal, DepGate, ArrTerminal, ArrGate string
	Aircraft, Tail                             string
}

type adbMovement struct {
	Airport struct {
		IATA string `json:"iata"`
	} `json:"airport"`
	ScheduledTime *adbTime `json:"scheduledTime"`
	RevisedTime   *adbTime `json:"revisedTime"`
	PredictedTime *adbTime `json:"predictedTime"`
	RunwayTime    *adbTime `json:"runwayTime"`
	Terminal      string   `json:"terminal"`
	Gate          string   `json:"gate"`
}

type adbTime struct {
	Local string `json:"local"`
}

func (t *adbTime) local() *model.LocalTime {
	if t == nil || len(t.Local) < 16 {
		return nil
	}
	v, err := model.ParseLocalTime(strings.Replace(t.Local[:16], " ", "T", 1))
	if err != nil {
		return nil
	}
	return &v
}

// FlightStatus asks AeroDataBox (with the user's own RapidAPI key) for a flight on its date.
func FlightStatus(key string, f *model.Flight) (*Status, error) {
	if strings.TrimSpace(key) == "" {
		return nil, fmt.Errorf("no AeroDataBox key (set one with: milehigh config scheduleKey <key>)")
	}
	code := strings.ToUpper(strings.TrimSpace(f.Airline)) + strings.TrimLeft(strings.TrimSpace(f.Number), "0")
	var raw []json.RawMessage
	err := get(fmt.Sprintf("https://aerodatabox.p.rapidapi.com/flights/number/%s/%s?withAircraftImage=false&withLocation=false", code, f.Date),
		map[string]string{"X-RapidAPI-Key": strings.TrimSpace(key), "X-RapidAPI-Host": "aerodatabox.p.rapidapi.com"}, &raw)
	if err != nil {
		return nil, err
	}
	var first *Status
	for _, r := range raw {
		var o struct {
			Number    string       `json:"number"`
			Status    string       `json:"status"`
			Departure *adbMovement `json:"departure"`
			Arrival   *adbMovement `json:"arrival"`
			Airline   *struct {
				IATA string `json:"iata"`
			} `json:"airline"`
			Aircraft *struct {
				Model string `json:"model"`
				Reg   string `json:"reg"`
			} `json:"aircraft"`
		}
		if json.Unmarshal(r, &o) != nil || o.Departure == nil || o.Arrival == nil {
			continue
		}
		al, num, ok := SplitNumber(o.Number)
		if !ok {
			continue
		}
		if o.Airline != nil && len(o.Airline.IATA) == 2 {
			al = o.Airline.IATA
		}
		dep, arr := o.Departure, o.Arrival
		departed := dep.RunwayTime != nil
		switch o.Status {
		case "Departed", "EnRoute", "Approaching", "Arrived", "Diverted":
			departed = true
		}
		arrived := o.Status == "Arrived"
		s := &Status{
			Airline: al, Number: num, From: dep.Airport.IATA, To: arr.Airport.IATA,
			DepScheduled: dep.ScheduledTime.local(), ArrScheduled: arr.ScheduledTime.local(),
			Departed: departed, Arrived: arrived, Canceled: strings.HasPrefix(o.Status, "Cancel"),
			DepTerminal: dep.Terminal, DepGate: dep.Gate, ArrTerminal: arr.Terminal, ArrGate: arr.Gate,
		}
		s.DepRevised = dep.RevisedTime.local()
		if s.DepRevised == nil && departed {
			s.DepRevised = dep.RunwayTime.local()
		}
		s.ArrRevised = arr.RevisedTime.local()
		if s.ArrRevised == nil {
			s.ArrRevised = arr.PredictedTime.local()
		}
		if s.ArrRevised == nil && arrived {
			s.ArrRevised = arr.RunwayTime.local()
		}
		if o.Aircraft != nil {
			s.Aircraft, s.Tail = o.Aircraft.Model, o.Aircraft.Reg
		}
		if first == nil {
			first = s
		}
		if strings.EqualFold(s.From, f.From) {
			return s, nil
		}
	}
	if first == nil {
		return nil, ErrNotFound
	}
	return first, nil
}

// Apply brings a logged flight up to date with a status, the way Skyline's refresh does.
// Returns a changed copy.
func (s *Status) Apply(f model.Flight) model.Flight {
	or := func(a, b *model.LocalTime) *model.LocalTime {
		if a != nil {
			return a
		}
		return b
	}
	str := func(v string, old *string) *string {
		if v != "" {
			return &v
		}
		return old
	}
	f.DepLocal = or(f.DepLocal, s.DepScheduled)
	f.ArrLocal = or(f.ArrLocal, s.ArrScheduled)
	if s.Departed {
		f.DepActual = or(s.DepRevised, f.DepActual)
	} else {
		f.DepEstimated = or(s.DepRevised, f.DepEstimated)
	}
	if s.Arrived {
		f.ArrActual = or(s.ArrRevised, f.ArrActual)
	} else {
		f.ArrEstimated = or(s.ArrRevised, f.ArrEstimated)
	}
	f.DepTerminal = str(s.DepTerminal, f.DepTerminal)
	f.DepGate = str(s.DepGate, f.DepGate)
	f.ArrTerminal = str(s.ArrTerminal, f.ArrTerminal)
	f.ArrGate = str(s.ArrGate, f.ArrGate)
	if f.Aircraft == nil && s.Aircraft != "" {
		f.Aircraft = &s.Aircraft
	}
	if f.Tail == nil && s.Tail != "" {
		f.Tail = &s.Tail
	}
	f.Canceled = f.Canceled || s.Canceled
	return f
}
