package main

import "time"

type Snapshot struct {
	Timestamp   time.Time `json:""`
	Hour        int       `json:"hour"`
	sixHour     int       `json:"sixHour"`
	Day         int       `json:"day"`
	ThreeDay    int       `json:"threeDay"`
	SevenDay    int       `json:"sevenDay"`
	FourteenDay int       `json:"fourteenDay"`
	ThirtyDay   int       `json:"thirtyDay"`
	NinetyDay   int       `json:"ninetyDay"`
	HalfYear    int       `json:"halfYear"`
	Year        int       `json:"year"`
	AllTime     int       `json:"allTime"`
}

type StatType int

const (
	StatAll StatType = iota
	StatClearnet
	StatTorOnly
	StatDualStack
)

type StatsLogger struct {
	db       Datastore
	getNodes func() []Node
}

func NewStatsLogger(db Datastore, getNodes func() []Node) *StatsLogger {
	sl := &StatsLogger{
		db:       db,
		getNodes: getNodes,
	}
	return sl
}

func (sl *StatsLogger) run() {
	t := time.NewTicker(time.Hour)
	for range t.C {
		nodes := sl.getNodes()

		all := Snapshot{Timestamp: time.Now()}
		clearnet := Snapshot{Timestamp: time.Now()}
		torOnly := Snapshot{Timestamp: time.Now()}
		dualStack := Snapshot{Timestamp: time.Now()}

		for _, node := range nodes {
			var rec Snapshot
			all.AllTime++
			if node.LastConnect.Add(time.Hour).After(time.Now()) {
				rec.Hour = 1
				all.Hour++
			}
			if node.LastConnect.Add(time.Hour * 6).After(time.Now()) {
				rec.sixHour = 1
				all.sixHour++
			}
			if node.LastConnect.Add(time.Hour * 24).After(time.Now()) {
				rec.Day = 1
				all.Day++
			}
			if node.LastConnect.Add(time.Hour * 24 * 3).After(time.Now()) {
				rec.ThreeDay = 1
				all.ThreeDay++
			}
			if node.LastConnect.Add(time.Hour * 24 * 7).After(time.Now()) {
				rec.SevenDay = 1
				all.SevenDay++
			}
			if node.LastConnect.Add(time.Hour * 24 * 14).After(time.Now()) {
				rec.FourteenDay = 1
				all.FourteenDay++
			}
			if node.LastConnect.Add(time.Hour * 24 * 30).After(time.Now()) {
				rec.ThirtyDay = 1
				all.ThirtyDay++
			}
			if node.LastConnect.Add(time.Hour * 24 * 90).After(time.Now()) {
				rec.NinetyDay = 1
				all.NinetyDay++
			}
			if node.LastConnect.Add(time.Hour * 24 * 182).After(time.Now()) {
				rec.HalfYear = 1
				all.HalfYear++
			}
			if node.LastConnect.Add(time.Hour * 24 * 365).After(time.Now()) {
				rec.Year = 1
				all.Year++
			}
			nodeType := GetNodeType(node.PeerInfo.Addrs)
			switch nodeType {
			case Clearnet:
				clearnet.Hour += rec.Hour
				clearnet.sixHour += rec.sixHour
				clearnet.Day += rec.Day
				clearnet.ThreeDay += rec.ThreeDay
				clearnet.SevenDay += rec.SevenDay
				clearnet.FourteenDay += rec.FourteenDay
				clearnet.ThirtyDay += rec.ThirtyDay
				clearnet.NinetyDay += rec.NinetyDay
				clearnet.HalfYear += rec.HalfYear
				clearnet.Year += rec.Year
				clearnet.AllTime++
			case TorOnly:
				torOnly.Hour += rec.Hour
				torOnly.sixHour += rec.sixHour
				torOnly.Day += rec.Day
				torOnly.ThreeDay += rec.ThreeDay
				torOnly.SevenDay += rec.SevenDay
				torOnly.FourteenDay += rec.FourteenDay
				torOnly.ThirtyDay += rec.ThirtyDay
				torOnly.NinetyDay += rec.NinetyDay
				torOnly.HalfYear += rec.HalfYear
				torOnly.Year += rec.Year
				torOnly.AllTime++
			case DualStack:
				dualStack.Hour += rec.Hour
				dualStack.sixHour += rec.sixHour
				dualStack.Day += rec.Day
				dualStack.ThreeDay += rec.ThreeDay
				dualStack.SevenDay += rec.SevenDay
				dualStack.FourteenDay += rec.FourteenDay
				dualStack.ThirtyDay += rec.ThirtyDay
				dualStack.NinetyDay += rec.NinetyDay
				dualStack.HalfYear += rec.HalfYear
				dualStack.Year += rec.Year
				dualStack.AllTime++
			}
		}
		sl.db.PutStat(StatAll, all)
		sl.db.PutStat(StatClearnet, clearnet)
		sl.db.PutStat(StatTorOnly, torOnly)
		sl.db.PutStat(StatDualStack, dualStack)
	}
}
