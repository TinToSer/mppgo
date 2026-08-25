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
| Project properties | Partial — file path, calendar, version, start/finish/status dates, scheduling factors |
| Calendars | Complete — working weeks, hours, exceptions, inheritance |
| Tasks | Partial — hierarchy, WBS, early/late/actual dates, duration, work, cost, constraint, priority, flags |
| Resources | Partial — name, initials, type, group, rate, max units, work, cost, calendar |
| Assignments | Partial — task/resource links, units, work |
| Task dependencies | Complete — predecessors/successors, relation type, lag |
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

Tasks come back in ID order — the row order MS Project displays — with the
outline hierarchy, schedule dates and dependencies resolved:

```go
for _, t := range pf.Tasks {
    if t.Summary || t.Inactive {
        continue
    }
    fmt.Printf("%s %s  %.1f%s  %s..%s\n",
        t.WBS, t.Name,
        t.Duration.Amount, t.Duration.Units,
        t.Start.Format("2006-01-02"), t.Finish.Format("2006-01-02"))

    for _, r := range t.Predecessors {
        pred := pf.TaskByID(r.PredecessorUniqueID)
        fmt.Printf("    after %q (%s, lag %.1f%s)\n",
            pred.Name, r.Type, r.Lag.Amount, r.Lag.Units)
    }
}
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
- `cmd/dumptasks` — dump a plan's task hierarchy, dates and dependencies.

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
- **Field offsets come from the file, not just from defaults.** Unlike
  calendars, task/resource/assignment fields sit at byte offsets MS Project
  records in a per-file field map (a Props value) rather than at fixed
  positions — real files, especially ones from a continuously-updated
  Microsoft 365 build, can and do shift these relative to older reference
  layouts. This reader parses that field map when present and only falls
  back to the MPP14 defaults when it is absent.
- **WBS is synthesized when the file doesn't store one.** MS Project only
  persists a task's WBS when a user customizes it; otherwise it's
  auto-numbered from the outline structure and never written to the file.
  `Task.WBS` reproduces that auto-numbering (outline position, MS Project's
  own algorithm) rather than leaving it blank, since a blank WBS would
  otherwise be the common case, not the exception.
- **Inactive tasks keep their late dates but lose their current ones.** A
  task explicitly deactivated in MS Project (Project 2010+) has its
  Start/Finish cleared to "not applicable" while LateStart/LateFinish keep
  whatever they were before deactivation. `Task.Inactive` distinguishes this
  from genuinely missing data.
- **`Task.Start`/`Finish` are stored only for active leaf tasks.** Summary
  tasks carry no stored start or finish — MS Project rolls those up from
  the children on display — so those fields are zero for them.
  `EarlyStart`/`EarlyFinish` are populated for every task and are the ones
  to reach for when a summary row needs a date.
- **Units are percentages, not fractions.** `Assignment.Units` and
  `Resource.MaxUnits` report 100 for a full-time 100%, matching MS Project
  and MPXJ rather than normalising to 1.0.

## License

MIT — see [LICENSE](LICENSE). Format knowledge is attributed separately in
[NOTICE](NOTICE).
