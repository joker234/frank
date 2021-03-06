package main

import (
	"database/sql"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"log"
	"regexp"
	"strings"
	"time"
)

const INTERVAL_PERIOD time.Duration = 5 * time.Minute
const HOUR_TO_TICK int = 0
const MINUTE_TO_TICK int = 0
const SECOND_TO_TICK int = 1
const SEPARATOR = "|"

// I would prefer 🕖, but it’s not available in most fonts
const ROBOT_BLOCK_IDENTIFIER = "ꜰ"

var regexTomorrow = regexp.MustCompile(`(?i)\smorgen:?\s`)
var regexToday = regexp.MustCompile(`(?i)\sheute:?\s`)

func TopicChanger() {
	ticker := time.NewTicker(INTERVAL_PERIOD)
	for {
		setTopic("#chaos-hd")
		<-ticker.C
	}
}

func setTopic(channel string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("most likely coding error: %v", r)
		}
	}()

	topic, err := TopicGet(channel)
	if err != nil {
		log.Printf("cannot update topic: TopicGet reports: %s", err)
		return
	}

	newtopic := insertNextEvent(topic)
	newtopic = advanceDates(newtopic)

	if topic == newtopic {
		return
	}

	log.Printf("%s OLD TOPIC: %s", channel, topic)
	log.Printf("%s NEW TOPIC: %s", channel, newtopic)

	Topic(channel, newtopic)
}

func advanceDates(topic string) string {
	parts := splitTopic(topic)
	new := []string{}

	dateToday := time.Now()
	dateTomorrow := time.Now().AddDate(0, 0, 1)

	for _, part := range parts {
		if strings.Contains(part, dateToday.Format("2006-01-02")) {
			part = strings.Replace(part, dateToday.Format("2006-01-02"), "HEUTE ("+dateToday.Format("02.Jan")+")", -1)
			new = append(new, part)

		} else if strings.Contains(part, dateTomorrow.Format("2006-01-02")) {
			part = strings.Replace(part, dateTomorrow.Format("2006-01-02"), "MORGEN ("+dateTomorrow.Format("02.Jan")+")", -1)
			new = append(new, part)

		} else if regexTomorrow.MatchString(part) {
			// tomorrow → today
			match := regexTomorrow.FindStringSubmatch(part)[0]
			r := " heute"
			if strings.HasSuffix(match, ": ") {
				r += ":"
			}
			r += " "

			if strings.HasPrefix(match, " MOR") {
				r = strings.ToUpper(r)
			}

			n := regexTomorrow.ReplaceAllString(part, r)
			new = append(new, n)

		} else if regexToday.MatchString(part) {
			// today → (remove)

		} else {
			// keep
			new = append(new, part)
		}
	}
	return joinTopic(new)
}

func insertNextEvent(topic string) string {
	event := " " + ROBOT_BLOCK_IDENTIFIER + " " + getNextEventString() + " "

	parts := splitTopic(topic)

	eventIdx := -1
	for i, part := range parts {
		if strings.Contains(part, ROBOT_BLOCK_IDENTIFIER) {
			eventIdx = i
			break
		}
	}

	if eventIdx < 0 {
		parts = append(parts, event)
	} else {
		parts[eventIdx] = event
	}

	return joinTopic(parts)
}

func splitTopic(topic string) []string {
	return strings.Split(" "+topic+" ", SEPARATOR)
}

func joinTopic(parts []string) string {
	return strings.TrimSpace(strings.Join(parts, SEPARATOR))
}

// stores all required data for the next event to accurately
// describe it everyone who listens.
type event struct {
	Stammtisch bool
	Override   sql.NullString
	Location   sql.NullString
	Date       time.Time
	Topic      sql.NullString
}

// retrieves the next event from the database and parses it into
// an “event”. Returns nil if the DB connection or query fails.
// Function is defined in this way so it may easily be overwritten
// when testing.
var getNextEvent = func() *event {
	db, err := sqlx.Connect("postgres", "dbname=nnev user=anon host=/var/run/postgresql sslmode=disable")
	if err != nil {
		log.Println(err)
		return nil
	}

	defer db.Close()

	evt := event{}
	err = db.Get(&evt, `
		SELECT stammtisch, override, location, termine.date, topic
		FROM termine
		LEFT JOIN vortraege
		ON termine.date = vortraege.date
		WHERE termine.date >= $1
		ORDER BY termine.date ASC
		LIMIT 1`, time.Now().Format("2006-01-02"))

	if err != nil {
		log.Println(err)
		return nil
	}

	if *verbose {
		log.Printf("event from SQL: %v", evt)
	}

	return &evt
}

// converts an event (retrieved from the database) into a condensed
// single string in human readable form
func getNextEventString() string {
	evt := getNextEvent()
	if evt == nil {
		return "SQL Error, see logs"
	}

	t := evt.Date.Format("2006-01-02") + ": "

	if toStr(evt.Override) != "" {
		t += "Ausnahmsweise: " + toStr(evt.Override)

	} else if evt.Stammtisch {
		t += "Stammtisch @ " + strOrDefault(toStr(evt.Location), "TBA")
		t += " https://www.noname-ev.de/yarpnarp.html"
		t += " bitte zu/absagen"

	} else {
		t += "c¼h: " + strOrDefault(toStr(evt.Topic), "noch keine ◉︵◉")
	}

	return strings.TrimSpace(t)
}

// returns the first argument “str”, unless it is empty. If so,
// it will instead return the second argument “def”.
func strOrDefault(str string, def string) string {
	if str == "" {
		return def
	} else {
		return str
	}
}

func toStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	} else {
		return ""
	}
}
