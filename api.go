package main

import (
	"encoding/json"
	"fmt"
	"github.com/wcharczuk/go-chart"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

type APIServer struct {
	crawler  *Crawler
	addr     string
	hostname string
}

func NewAPIServer(addr string, crawler *Crawler, hostname string) *APIServer {
	return &APIServer{crawler, addr, hostname}
}

func (a *APIServer) serve() {
	http.HandleFunc("/", a.handleIndex)
	http.HandleFunc("/singleCharts/", a.handleSingleCharts)
	http.HandleFunc("/useragents", a.handleUserAgents)
	http.HandleFunc("/peers", a.handlePeers)
	http.HandleFunc("/count", a.handleCount)
	http.HandleFunc("/charts/useragents", a.handleChartsUserAgents)
	http.HandleFunc("/charts/nodes", a.handleChartsNodes)
	http.HandleFunc("/charts/listings", a.handleChartsListings)
	http.HandleFunc("/charts/ratings", a.handleChartsRatings)
	http.HandleFunc("/charts/vendors", a.handleChartsVendors)
	http.ListenAndServe(a.addr, nil)
}

func (a *APIServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	s := strings.Split(r.URL.Path, "/")
	var ret string
	if s[len(s)-1] == "styles.css" {
		b, err := Asset("static/styles.css")
		if err != nil {
			fmt.Println("1")
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Add("Content-Type", " text/css")
		ret = string(b)
	} else {
		b, err := Asset("static/index.html")
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		s := strings.Replace(string(b), "{HOSTNAME}", a.hostname, -1)
		ret = s
	}
	fmt.Fprint(w, ret)
}

func (a *APIServer) handleSingleCharts(w http.ResponseWriter, r *http.Request) {
	s := strings.Split(r.URL.Path, "/")
	var ret string
	if s[len(s)-1] == "styles.css" {
		b, err := Asset("static/styles.css")
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
		}
		w.Header().Add("Content-Type", " text/css")
		ret = string(b)
	} else {
		b, err := Asset(path.Join("static", "singleCharts", s[len(s)-1]))
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
		}
		s := strings.Replace(string(b), "{HOSTNAME}", a.hostname, -1)
		ret = s
	}
	fmt.Fprint(w, ret)
}

func (a *APIServer) handlePeers(w http.ResponseWriter, r *http.Request) {
	a.crawler.lock.RLock()
	defer a.crawler.lock.RUnlock()
	only := r.URL.Query().Get("only")
	var peers []string
	for peer, nd := range a.crawler.theList {
		if len(nd.PeerInfo.Addrs) == 0 {
			continue
		}
		if only == "" {
			peers = append(peers, peer)
			continue
		}
		nodeType := GetNodeType(nd.PeerInfo.Addrs)

		switch {
		case only == "tor" && nodeType == TorOnly:
			peers = append(peers, peer)
		case only == "dualstack" && nodeType == DualStack:
			peers = append(peers, peer)
		case only == "clearnet" && nodeType == Clearnet:
			peers = append(peers, peer)
		}
	}
	out, err := json.MarshalIndent(peers, "", "    ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err.Error())
	}
	if string(out) == "null" {
		out = []byte("[]")
	}
	fmt.Fprint(w, string(out))
}

func (a *APIServer) handleCount(w http.ResponseWriter, r *http.Request) {
	lastActive := r.URL.Query().Get("lastActive")
	only := r.URL.Query().Get("only")
	var d time.Duration
	var err error
	if lastActive != "" {
		d, err = time.ParseDuration(lastActive)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, err.Error())
			return
		}
	}
	count := 0
	for _, nd := range a.crawler.theList {
		if len(nd.PeerInfo.Addrs) == 0 {
			continue
		}
		if lastActive != "" && nd.LastConnect.Add(d).Before(time.Now()) {
			continue
		}
		if only == "" {
			count++
			continue
		}
		nodeType := GetNodeType(nd.PeerInfo.Addrs)

		switch {
		case only == "tor" && nodeType == TorOnly:
			count++
		case only == "dualstack" && nodeType == DualStack:
			count++
		case only == "clearnet" && nodeType == Clearnet:
			count++
		}
	}
	fmt.Fprint(w, count)
}

func (a *APIServer) handleUserAgents(w http.ResponseWriter, r *http.Request) {
	lastActive := r.URL.Query().Get("lastActive")
	var d time.Duration
	var err error
	if lastActive != "" {
		d, err = time.ParseDuration(lastActive)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, err.Error())
			return
		}
	}
	ua := make(map[string]int)
	for _, nd := range a.crawler.theList {
		if lastActive != "" && nd.LastConnect.Add(d).Before(time.Now()) {
			continue
		}
		if nd.UserAgent != "" {
			count, ok := ua[nd.UserAgent]
			if !ok {
				ua[nd.UserAgent] = 1
			} else {
				ua[nd.UserAgent] = count + 1
			}
		}
	}
	out, err := json.MarshalIndent(ua, "", "    ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err.Error())
	}
	fmt.Fprint(w, string(out))
}

func (a *APIServer) handleChartsUserAgents(w http.ResponseWriter, r *http.Request) {
	lastActive := r.URL.Query().Get("lastActive")
	var d time.Duration
	var err error
	if lastActive != "" {
		d, err = time.ParseDuration(lastActive)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, err.Error())
			return
		}
	}
	ua := make(map[string]int)
	for _, nd := range a.crawler.theList {
		if lastActive != "" && nd.LastConnect.Add(d).Before(time.Now()) {
			continue
		}
		if nd.UserAgent != "" {
			count, ok := ua[nd.UserAgent]
			if !ok {
				ua[nd.UserAgent] = 1
			} else {
				ua[nd.UserAgent] = count + 1
			}
		}
	}
	pie := chart.PieChart{
		Width:  1000,
		Height: 1000,
		Values: []chart.Value{},
		Title:  "User Agents",
		Background: chart.Style{
			Padding: chart.Box{
				Top:    100,
				Left:   100,
				Bottom: 100,
				Right:  100,
			},
		},
	}
	for value, count := range ua {
		pie.Values = append(pie.Values, chart.Value{Value: float64(count), Label: value + " (" + strconv.Itoa(count) + ")"})
	}

	w.Header().Set("Content-Type", "image/png")
	err = pie.Render(chart.PNG, w)
	if err != nil {
		fmt.Printf("Error rendering pie chart: %v\n", err)

	}
}

func (a *APIServer) handleChartsNodes(w http.ResponseWriter, r *http.Request) {
	if a.crawler.db == nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Stats database unavailable")
		return
	}
	only := r.URL.Query().Get("only")
	lastActive := r.URL.Query().Get("lastActive")
	var d time.Duration
	var err error
	if lastActive != "" {
		d, err = time.ParseDuration(lastActive)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, err.Error())
			return
		}
	}
	timeFrame := r.URL.Query().Get("timeFrame")
	var dt time.Duration
	if timeFrame != "" {
		dt, err = time.ParseDuration(timeFrame)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, err.Error())
			return
		}
	}

	// All
	statsAll, err := a.crawler.db.GetStats(StatAll)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err.Error())
		return
	}
	sort.Sort(timeSlice(statsAll))
	var xvals []time.Time
	var yvals []float64
	for _, s := range statsAll {
		if s.Timestamp.Before(time.Now().Add(-dt)) {
			continue
		}
		log.Notice(s, d, GetStatLevel(s, d))
		yvals = append(yvals, float64(GetStatLevel(s, d)))
		xvals = append(xvals, s.Timestamp)
	}
	allSeries := chart.TimeSeries{
		Name: "All",
		Style: chart.Style{
			Show:        true,
			StrokeColor: chart.ColorCyan,
			FillColor:   chart.ColorCyan.WithAlpha(64),
		},
		XValues: xvals,
		YValues: yvals,
	}

	// Clearnet
	statsClearnet, err := a.crawler.db.GetStats(StatClearnet)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err.Error())
		return
	}
	sort.Sort(timeSlice(statsClearnet))
	var xvals2 []time.Time
	var yvals2 []float64
	for _, s := range statsClearnet {
		if s.Timestamp.Before(time.Now().Add(-dt)) {
			continue
		}
		yvals2 = append(yvals2, float64(GetStatLevel(s, d)))
		xvals2 = append(xvals2, s.Timestamp)
	}
	clearnetSeries := chart.TimeSeries{
		Name: "Clearnet",
		Style: chart.Style{
			Show:        true,
			StrokeColor: chart.ColorBlue,
			FillColor:   chart.ColorBlue.WithAlpha(64),
		},
		XValues: xvals2,
		YValues: yvals2,
	}

	// TorOnly
	statsTor, err := a.crawler.db.GetStats(StatTorOnly)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err.Error())
		return
	}
	sort.Sort(timeSlice(statsTor))
	var xvals3 []time.Time
	var yvals3 []float64
	for _, s := range statsTor {
		if s.Timestamp.Before(time.Now().Add(-dt)) {
			continue
		}
		yvals3 = append(yvals3, float64(GetStatLevel(s, d)))
		xvals3 = append(xvals3, s.Timestamp)
	}
	torSeries := chart.TimeSeries{
		Name: "Tor",
		Style: chart.Style{
			Show:        true,
			StrokeColor: chart.ColorGreen,
			FillColor:   chart.ColorGreen.WithAlpha(64),
		},
		XValues: xvals3,
		YValues: yvals3,
	}

	// DualStack
	statsDualStack, err := a.crawler.db.GetStats(StatDualStack)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err.Error())
		return
	}
	sort.Sort(timeSlice(statsDualStack))
	var xvals4 []time.Time
	var yvals4 []float64
	for _, s := range statsDualStack {
		if s.Timestamp.Before(time.Now().Add(-dt)) {
			continue
		}
		yvals4 = append(yvals4, float64(GetStatLevel(s, d)))
		xvals4 = append(xvals4, s.Timestamp)
	}
	dualStackSeries := chart.TimeSeries{
		Name: "Dualstack",
		Style: chart.Style{
			Show:        true,
			StrokeColor: chart.ColorOrange,
			FillColor:   chart.ColorOrange.WithAlpha(64),
		},
		XValues: xvals4,
		YValues: yvals4,
	}

	graph := chart.Chart{
		Width:  1080,
		Height: 620,
		Background: chart.Style{
			Padding: chart.Box{
				Top: 50,
			},
		},
		YAxis: chart.YAxis{
			Name:      "Nodes",
			NameStyle: chart.StyleShow(),
			Style:     chart.StyleShow(),
			TickStyle: chart.Style{
				TextRotationDegrees: 45.0,
			},
			ValueFormatter: func(v interface{}) string {
				return fmt.Sprintf("%d", int(v.(float64)))
			},
		},
		XAxis: chart.XAxis{
			Name:      "Time",
			NameStyle: chart.StyleShow(),
			Style: chart.Style{
				Show: true,
			},
			ValueFormatter: chart.TimeHourValueFormatter,
			GridMajorStyle: chart.Style{
				Show:        true,
				StrokeColor: chart.ColorAlternateGray,
				StrokeWidth: 1.0,
			},
		},
	}
	switch strings.ToLower(only) {
	case "clearnet":
		graph.Series = []chart.Series{clearnetSeries}
	case "tor":
		graph.Series = []chart.Series{torSeries}
	case "dualstack":
		graph.Series = []chart.Series{dualStackSeries}
	default:
		graph.Series = []chart.Series{allSeries, clearnetSeries, torSeries, dualStackSeries}
	}

	graph.Elements = []chart.Renderable{chart.LegendThin(&graph)}

	w.Header().Set("Content-Type", chart.ContentTypePNG)
	graph.Render(chart.PNG, w)
}

func (a *APIServer) handleChartsListings(w http.ResponseWriter, r *http.Request) {
	if a.crawler.db == nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Stats database unavailable")
		return
	}
	lastActive := r.URL.Query().Get("lastActive")
	var d time.Duration
	var err error
	if lastActive != "" {
		d, err = time.ParseDuration(lastActive)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, err.Error())
			return
		}
	}
	timeFrame := r.URL.Query().Get("timeFrame")
	var dt time.Duration
	if timeFrame != "" {
		dt, err = time.ParseDuration(timeFrame)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, err.Error())
			return
		}
	}

	// Listings
	statsListings, err := a.crawler.db.GetStats(StatListings)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err.Error())
		return
	}
	sort.Sort(timeSlice(statsListings))
	var xvals []time.Time
	var yvals []float64
	for _, s := range statsListings {
		if s.Timestamp.Before(time.Now().Add(-dt)) {
			continue
		}

		yvals = append(yvals, float64(GetStatLevel(s, d)))
		xvals = append(xvals, s.Timestamp)
	}
	listingSeries := chart.TimeSeries{
		Name: "Listings",
		Style: chart.Style{
			Show:        true,
			StrokeColor: chart.ColorCyan,
			FillColor:   chart.ColorCyan.WithAlpha(64),
		},
		XValues: xvals,
		YValues: yvals,
	}

	graph := chart.Chart{
		Width:  1080,
		Height: 620,
		Background: chart.Style{
			Padding: chart.Box{
				Top: 50,
			},
		},
		YAxis: chart.YAxis{
			Name:      "Listings",
			NameStyle: chart.StyleShow(),
			Style:     chart.StyleShow(),
			TickStyle: chart.Style{
				TextRotationDegrees: 45.0,
			},
			ValueFormatter: func(v interface{}) string {
				return fmt.Sprintf("%d", int(v.(float64)))
			},
		},
		XAxis: chart.XAxis{
			Name:      "Time",
			NameStyle: chart.StyleShow(),
			Style: chart.Style{
				Show: true,
			},
			ValueFormatter: chart.TimeHourValueFormatter,
			GridMajorStyle: chart.Style{
				Show:        true,
				StrokeColor: chart.ColorAlternateGray,
				StrokeWidth: 1.0,
			},
		},
		Series: []chart.Series{listingSeries},
	}

	graph.Elements = []chart.Renderable{chart.LegendThin(&graph)}

	w.Header().Set("Content-Type", chart.ContentTypePNG)
	graph.Render(chart.PNG, w)
}

func (a *APIServer) handleChartsRatings(w http.ResponseWriter, r *http.Request) {
	if a.crawler.db == nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Stats database unavailable")
		return
	}
	lastActive := r.URL.Query().Get("lastActive")
	var d time.Duration
	var err error
	if lastActive != "" {
		d, err = time.ParseDuration(lastActive)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, err.Error())
			return
		}
	}
	timeFrame := r.URL.Query().Get("timeFrame")
	var dt time.Duration
	if timeFrame != "" {
		dt, err = time.ParseDuration(timeFrame)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, err.Error())
			return
		}
	}

	// Ratings
	statsRatings, err := a.crawler.db.GetStats(StatRatings)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err.Error())
		return
	}
	sort.Sort(timeSlice(statsRatings))
	var xvals []time.Time
	var yvals []float64
	for _, s := range statsRatings {
		if s.Timestamp.Before(time.Now().Add(-dt)) {
			continue
		}

		yvals = append(yvals, float64(GetStatLevel(s, d)))
		xvals = append(xvals, s.Timestamp)
	}
	ratingsSeries := chart.TimeSeries{
		Name: "Listings",
		Style: chart.Style{
			Show:        true,
			StrokeColor: chart.ColorCyan,
			FillColor:   chart.ColorCyan.WithAlpha(64),
		},
		XValues: xvals,
		YValues: yvals,
	}

	graph := chart.Chart{
		Width:  1080,
		Height: 620,
		Background: chart.Style{
			Padding: chart.Box{
				Top: 50,
			},
		},
		YAxis: chart.YAxis{
			Name:      "Ratings",
			NameStyle: chart.StyleShow(),
			Style:     chart.StyleShow(),
			TickStyle: chart.Style{
				TextRotationDegrees: 45.0,
			},
			ValueFormatter: func(v interface{}) string {
				return fmt.Sprintf("%d", int(v.(float64)))
			},
		},
		XAxis: chart.XAxis{
			Name:      "Time",
			NameStyle: chart.StyleShow(),
			Style: chart.Style{
				Show: true,
			},
			ValueFormatter: chart.TimeHourValueFormatter,
			GridMajorStyle: chart.Style{
				Show:        true,
				StrokeColor: chart.ColorAlternateGray,
				StrokeWidth: 1.0,
			},
		},
		Series: []chart.Series{ratingsSeries},
	}

	graph.Elements = []chart.Renderable{chart.LegendThin(&graph)}

	w.Header().Set("Content-Type", chart.ContentTypePNG)
	graph.Render(chart.PNG, w)
}

func (a *APIServer) handleChartsVendors(w http.ResponseWriter, r *http.Request) {
	if a.crawler.db == nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Stats database unavailable")
		return
	}
	lastActive := r.URL.Query().Get("lastActive")
	var d time.Duration
	var err error
	if lastActive != "" {
		d, err = time.ParseDuration(lastActive)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, err.Error())
			return
		}
	}
	timeFrame := r.URL.Query().Get("timeFrame")
	var dt time.Duration
	if timeFrame != "" {
		dt, err = time.ParseDuration(timeFrame)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, err.Error())
			return
		}
	}

	// Vendors
	statsVendors, err := a.crawler.db.GetStats(StatVendors)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err.Error())
		return
	}
	sort.Sort(timeSlice(statsVendors))
	var xvals []time.Time
	var yvals []float64
	for _, s := range statsVendors {
		if s.Timestamp.Before(time.Now().Add(-dt)) {
			continue
		}

		yvals = append(yvals, float64(GetStatLevel(s, d)))
		xvals = append(xvals, s.Timestamp)
	}
	vendorSeries := chart.TimeSeries{
		Name: "Vendors",
		Style: chart.Style{
			Show:        true,
			StrokeColor: chart.ColorCyan,
			FillColor:   chart.ColorCyan.WithAlpha(64),
		},
		XValues: xvals,
		YValues: yvals,
	}

	graph := chart.Chart{
		Width:  1080,
		Height: 620,
		Background: chart.Style{
			Padding: chart.Box{
				Top: 50,
			},
		},
		YAxis: chart.YAxis{
			Name:      "Vendors",
			NameStyle: chart.StyleShow(),
			Style:     chart.StyleShow(),
			TickStyle: chart.Style{
				TextRotationDegrees: 45.0,
			},
			ValueFormatter: func(v interface{}) string {
				return fmt.Sprintf("%d", int(v.(float64)))
			},
		},
		XAxis: chart.XAxis{
			Name:      "Time",
			NameStyle: chart.StyleShow(),
			Style: chart.Style{
				Show: true,
			},
			ValueFormatter: chart.TimeHourValueFormatter,
			GridMajorStyle: chart.Style{
				Show:        true,
				StrokeColor: chart.ColorAlternateGray,
				StrokeWidth: 1.0,
			},
		},
		Series: []chart.Series{vendorSeries},
	}

	graph.Elements = []chart.Renderable{chart.LegendThin(&graph)}

	w.Header().Set("Content-Type", chart.ContentTypePNG)
	graph.Render(chart.PNG, w)
}

func GetStatLevel(stat Snapshot, d time.Duration) int {
	switch {
	case d == 0:
		return stat.AllTime
	case d >= time.Hour*99999:
		return stat.AllTime
	case d >= time.Hour*24*356:
		return stat.Year
	case d >= time.Hour*24*182:
		return stat.HalfYear
	case d >= time.Hour*24*90:
		return stat.NinetyDay
	case d >= time.Hour*24*30:
		return stat.ThirtyDay
	case d >= time.Hour*24*14:
		return stat.FourteenDay
	case d >= time.Hour*24*7:
		return stat.SevenDay
	case d >= time.Hour*24*3:
		return stat.ThreeDay
	case d >= time.Hour*24:
		return stat.Day
	case d >= time.Hour*6:
		return stat.SixHour
	case d >= time.Hour:
		return stat.Hour
	}
	return stat.AllTime
}
