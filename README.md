# mppgo

A Go library for reading Microsoft Project files, written from scratch with
no external dependencies (standard library only).

Modelled on [MPXJ](https://github.com/joniles/mpxj), the Java library that
reverse engineered the undocumented MPP binary format. See [NOTICE](NOTICE).

## Status

Early. The container and binary-primitive layers are complete and verified
against a real Project 2016/365 file; entity readers are being built on top.

| Area | State |
| --- | --- |
| CFB / OLE2 container | Complete |
| MPP14 binary primitives | Complete |
| Project properties | Partial — file path, default calendar, application version |
| Calendars | Complete — working weeks, hours, exceptions, inheritance |
| Tasks / Resources / Assignments | Not yet implemented |
| Custom fields / baselines | Not yet implemented |
| MSPDI (XML) read/write | Not yet implemented |
| MPP write | Not yet implemented |

Scope is MPP14 (Project 2010 through 365) plus MSPDI. Legacy MPP8/9/12 and
the non-Microsoft formats MPXJ supports are out of scope.

## Install

```sh
go get github.com/tintoser/mppgo
```

## Usage

```go
import "github.com/tintoser/mppgo/mpp"

pf, err := mpp.ReadFile("plan.mpp")
if err != nil {
    log.Fatal(err)
}

for _, cal := range pf.Calendars {
    fmt.Println(cal.Name, cal.IsWorkingDay(time.Monday))
}

// Resolves the weekly pattern, parent inheritance and exceptions together.
christmas := time.Date(2025, 12, 25, 0, 0, 0, 0, time.UTC)
fmt.Println(pf.DefaultCalendar.WorkingOn(christmas)) // false
```

Password-protected files return `mpp.ErrPasswordProtected`; non-MPP14 files
return `mpp.ErrUnsupportedFormat`.

## Packages

- `cfb` — Compound File Binary (OLE2) container reader. Generic MS-CFB, not
  Project-specific.
- `mpp` — MPP14 reader: binary block primitives and entity readers.
- `project` — format-agnostic data model that readers and writers target.
- `cmd/inspect` — dump a compound file's storage/stream tree.
- `cmd/dumpcalendars` — dump a plan's properties and calendars.

## Calendars

MPP stores a derived calendar as a sparse overlay: only the days a user
actually changed carry values, and everything else means "inherit from the
base calendar". The model reflects this with `DayDefault`, so read days
through the resolving accessors rather than the raw maps:

```go
cal.IsWorkingDay(day)  // resolves inheritance
cal.HoursFor(day)      // resolves inheritance
cal.WorkingOn(date)    // resolves inheritance + exceptions
cal.HoursOn(date)      // resolves inheritance + exceptions

cal.Days[day]          // this calendar's own override only
```

## Testing

```sh
go test ./...                                   # unit tests
go test ./mpp/ -fuzz FuzzRead -fuzztime 5m       # fuzz the parser
```

Tests cover the binary primitives with synthetic fixtures and the calendar
inheritance logic in isolation — these run with no setup. A further set of
tests exercises end-to-end parsing against a real MPP file; since such files
carry project-specific data, no fixture is checked into the repo. To run
that subset, drop your own sample file at `testdata/sample.mpp` (ignored by
git) — those tests skip automatically when it's absent. The parser is
fuzzed because it consumes untrusted binary input: malformed files must
return errors, never panic or exhaust memory.

## Design notes

- **Bounds-safe accessors.** Byte readers return zero values rather than
  panicking on short input. Real MPP files are frequently truncated or
  internally inconsistent, and a library should degrade rather than crash.
- **Unsigned 16-bit reads.** MPP encodes many fields as unsigned shorts and
  uses 65535 as a "not applicable" sentinel. Reading these as signed silently
  corrupts every date field, so `getShort` is unsigned by definition.
- **No trusted lengths.** Counts and sizes from the file are validated
  against the actual file length before being used to size allocations.

## License

MIT — see [LICENSE](LICENSE). Format knowledge is attributed separately in
[NOTICE](NOTICE).
