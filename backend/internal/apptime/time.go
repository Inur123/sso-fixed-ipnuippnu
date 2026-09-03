// Package apptime is the single wall-clock policy for the SSO application.
package apptime

import (
	"time"
	_ "time/tzdata" // Keep IANA time zones available in minimal production images.
)

const Zone = "Asia/Jakarta"

var Jakarta = mustLoadJakarta()

func mustLoadJakarta() *time.Location {
	location, err := time.LoadLocation(Zone)
	if err != nil {
		panic("cannot load application time zone: " + err.Error())
	}
	return location
}

// Configure runs once at startup, before workers or database connections start.
// It also makes process logs and database timestamp decoding use Jakarta.
func Configure() {
	time.Local = Jakarta
}

func Now() time.Time {
	return time.Now().In(Jakarta)
}
