package main

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/riclolsen/tase2/tase2"
)

func TestTASE2LogHandler(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	savedLevel := currentLogLevel()
	defer func() { setLogLevel(savedLevel) }()

	at := time.Date(2026, 9, 27, 10, 11, 12, 345000000, time.Local)
	rec := tase2.LogRecord{
		Time: at, Level: tase2.LogLevelInfo, Package: "mms", Message: "association up",
		ConnID: 7, RemoteAddr: "10.0.0.9:40001", PeerAPTitle: "1.1.999.2", PeerAEQualifier: 12,
	}
	h := tase2LogHandler("CONN_A")

	// Library Info maps to the driver's detailed level: filtered at normal.
	setLogLevel(LogLevelNormal)
	h(rec)
	if buf.Len() != 0 {
		t.Fatalf("info record logged at normal level: %q", buf.String())
	}

	setLogLevel(LogLevelDetailed)
	h(rec)
	want := "2026-09-27T10:11:12.345" + at.Format("Z07:00") +
		" - TASE2 CONN_A [conn 7 10.0.0.9:40001 1.1.999.2/12] mms - association up"
	if !strings.Contains(buf.String(), want) {
		t.Errorf("log line = %q, want it to contain %q (record time, label, conn context)", buf.String(), want)
	}

	// Library errors always pass; process-level records have no conn context.
	buf.Reset()
	setLogLevel(LogLevelMin)
	tase2LogHandler("")(tase2.LogRecord{Time: at, Level: tase2.LogLevelError, Package: "iso", Message: "bad TPKT"})
	if !strings.Contains(buf.String(), " - TASE2 iso - bad TPKT") {
		t.Errorf("error record = %q", buf.String())
	}

	// Library debug needs the driver's debug level.
	buf.Reset()
	setLogLevel(LogLevelDetailed)
	h(tase2.LogRecord{Time: at, Level: tase2.LogLevelDebug, Package: "tase2", Message: "pdu"})
	if buf.Len() != 0 {
		t.Errorf("debug record logged at detailed level: %q", buf.String())
	}
}
