# mppgo

A Go library for reading Microsoft Project files, exporting Project XML,
and verified non-scheduling edits of native MPP templates, written with
no external dependencies (standard library only).

Modelled on [MPXJ](https://github.com/joniles/mpxj), the Java library that
reverse engineered the undocumented MPP binary format. See [NOTICE](NOTICE).

## Status

Reads the full task/resource/assignment schedule — hierarchy, dates,
dependencies, calendars, notes, baselines and custom fields — from either
MPP (binary) or MSPDI (XML) files, verified against real Project 2016/365
files and, for MSPDI, against a hand-built document covering every area
this reader supports (there is no reverse-engineering involved in MSPDI —
see the `mspdi` package below — but also no real-world sample file to
verify against, the way `mpp` has). MSPDI model export and constrained,
byte-preserving native MPP template edits are implemented. This is not a
general proprietary-format writer or a scheduling engine.

| Area | MPP (binary) | MSPDI (XML) |
| --- | --- | --- |
| All stored fields | Every task, resource and assignment field the file's field map locates (~1,900 field definitions), decoded into `Fields` by name | Typed fields below |
| Project properties | Title/metadata (OLE summary properties preferred, as MS Project writes them), custom document properties, creation/saved dates, revision, GUID, scheduling defaults, currency, critical slack limit, week/fiscal year settings, baseline dates | Same, where MSPDI carries them |
| Project summary task | `File.ProjectSummaryTask` (unique ID 0, project-wide rollups) | Same |
| Calendars | Weekly patterns, hours, exceptions (recurring ones expanded per occurrence), work weeks, inheritance | Same |
| Tasks | Hierarchy, WBS, all date pairs, duration (manual duration for manual tasks), slack incl. total slack, critical, estimated, task mode, stop/resume, work/cost incl. overtime and earned value, constraint, type, leveling, flags, hyperlink, contact, subproject/external markers, recurring-task pattern | Same typed fields (recurrence as a flag only: MSPDI stores no pattern) |
| Resources | Identity, type, generic/budget, rates with their units, max/peak units, work/cost incl. actual, remaining and overtime, earned value, accrual, booking type, dates, hyperlink, calendar | Same |
| Assignments | Links (incl. unassigned work), units, work/cost incl. actual, remaining and overtime, actual dates, stop/resume, delay, leveling delay, contour, rate table, earned value, flags, custom fields, hyperlink | Same |
| Task dependencies | Complete — predecessors/successors, relation type, lag | Complete |
| Notes | Complete — RTF stripped to plain text (`Notes`) honouring code pages, original kept as `RTFNotes` | Complete — MSPDI stores notes as plain text already, so `RTFNotes` is always empty |
| Baselines | Complete — primary baseline plus Baseline1-10, for tasks, resources and assignments | Complete |
| Custom fields | Text1-30, Start1-10, Finish1-10, Number1-20, Date1-10, Duration1-10, Cost1-10, Flag1-20 (tasks, resources, assignments), lookup-table values, resolved Outline Code1-10 paths, with aliases | Same except outline codes; enterprise definitions unsupported |
| Resource cost rate tables (A-E) | Supported | Supported |
| Resource availability table | Supported | Supported |
| Timephased data | Planned, actual (incl. irregular ranges) and actual overtime work; baseline work/cost for all 11 baselines | Assignment remaining/actual/actual overtime/baseline work and baseline cost; raw records, including unknown types, retained |
| Write | Same-size verified template patches for advertised non-scheduling fields | Model-based schedule export in MSPDI schema order; not a lossless source-document round trip |

Not read: views, tables, filters, groups and other display settings;
blank (null) task rows; resource pools beyond the pool file name;
enterprise custom field definitions; East Asian double-byte characters in
RTF notes (shown as U+FFFD).

Scope is MPP14 (Project 2010 through 365) plus MSPDI. Legacy MPP8/9/12 and
the non-Microsoft formats MPXJ supports are out of scope.

Native patching supports 512-byte compound-file sectors with 64-byte mini
sectors. Stored variable fields must already exist, fit their existing
capacity and not be shared. Unknown bytes and the container layout are
preserved. Every patch is reopened and compared against the full intended
parsed model; Microsoft Project UI compatibility still requires validation
in Microsoft Project.

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

    // Both ends of a relation reached this way always resolve, so no
    // nil check is needed here.
    for _, r := range t.Predecessors {
        pred := pf.TaskByID(r.PredecessorUniqueID)
        fmt.Printf("    after %q (%s, lag %.1f%s)\n",
            pred.Name, r.Type, r.Lag.Amount, r.Lag.Units)
    }
}
```

Password-protected files return `mpp.ErrPasswordProtected`; non-MPP14 files
return `mpp.ErrUnsupportedFormat`.

An MSPDI (Project XML) file targets the same model, through the `mspdi`
package instead:

```go
import "github.com/tintoser/mppgo/mspdi"

pf, err := mspdi.ReadFile("plan.xml")
if err != nil {
    log.Fatal(err)
}
// pf is a *project.File, identical in shape to what mpp.ReadFile returns.
```

## Writing

Export a parsed or newly constructed model as Project XML:

```go
err := mspdi.WriteFile("export.xml", pf)
```

`WriteFile` creates a new file and refuses existing destinations. `Write`
accepts an `io.Writer`. XML export includes modelled properties, calendars,
tasks, dependencies, resources, assignments, baselines, supported custom
fields/aliases, resource tables and assignment timephased records. It does
not retain unmodelled features, unknown XML elements, views, macros or RTF
formatting. Unsupported custom-field definitions return an error rather
than silently omitting values. Timephased totals are exported; per-hour
rates may be rederived from the calendar. No schedule is recalculated.

Native MPP patching returns bytes without changing the source file:

```go
original, err := os.ReadFile("plan.mpp")
if err != nil {
  return err
}
patched, err := mpp.Patch(original, []mpp.NativeEdit{
  {Entity: "tasks", UniqueID: 123, Field: "priority", Value: 700},
})
```

Use `mpp.NativeWritableFields()` for the exact editable surface. Task names,
stored WBS, notes, priority, resource identity fields, existing Text/Number/
Cost custom fields and Flag bits are supported. Aliases are preserved in
`project.File.CustomFieldAliases`. Structural/scheduling edits, variable
growth and shared records are rejected. Output publishing, root boundaries,
revision/hash checks and backups belong to the calling application.

For raw inspection, `mpp.ReadRawStream` optionally removes known XOR
obfuscation; `Props.Keys()` and `VarMeta.UniqueIDs()` enumerate stored keys
and entities, including data outside the typed model. Unknown types remain
opaque rather than being guessed.

## Packages

- `cfb` — Compound File Binary (OLE2) container reader and same-size stream patcher. Generic MS-CFB, not
  Project-specific.
- `mpp` — MPP14 reader, raw inspection and verified native template edits.
- `mspdi` — MSPDI (Project XML) reader and model writer, targeting the same `project.File`
  model as `mpp`.
- `project` — format-agnostic data model that readers and writers target.
- `cmd/inspect` — dump a compound file's storage/stream tree.
- `cmd/dumpcalendars` — dump a plan's properties and calendars.
- `cmd/dumptasks` — dump a plan's task hierarchy, dates and dependencies;
  reads either an `.mpp` or an `.xml` (MSPDI) file.

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
- **Durations depend on project settings.** A duration is stored as a raw
  count plus a units code; turning that into the days or weeks MS Project
  displays uses the file's own `MinutesPerDay`/`MinutesPerWeek`/
  `DaysPerMonth`, so the same stored value means different things in a
  file set to an 8-hour day and one set to 12.
- **Document metadata comes from the Props stream.** MS Project writes the
  title, author and so on both there and into the OLE
  `\x05SummaryInformation` property set. This reader uses the Props copy
  and does not parse that property set, so a value MS Project left only in
  the latter is missed.
- **Notes are always RTF, even when the user typed plain text.** MS Project
  wraps every note — task, resource or assignment — as an RTF document.
  `Notes` is that RTF run through a from-scratch parser down to the plain
  text MS Project itself shows in the Notes box; `RTFNotes` keeps the
  original for a caller that wants the formatting.
- **A baseline is a snapshot, not a rolling history.** `Baseline` is the
  primary "Set Baseline" snapshot; `Baselines` holds the ten numbered ones
  (`Baseline1`..`Baseline10`) MS Project also supports, keyed by that
  number. Both are nil until the corresponding baseline has actually been
  set — there is no "baseline equal to current schedule" default.
- **A custom field absent from `CustomFields` means unset, not zero.** MS
  Project only writes a Text/Number/Date/Duration/Cost field once it has a
  value, so this reader mirrors that: a field with no data for a given task
  or resource is left out of the map entirely rather than present with a
  zero value indistinguishable from a real one. Keys use the alias a user
  gave the field (Customize Fields) when it has one, falling back to the
  generic name (`Text1`, `Number3`, ...) otherwise. A `Flag` field follows
  the same convention in spirit but never appears as `false`: since MS
  Project treats an unset flag as `No`, only a flag that is actually set
  (`true`) is added.
- **An Outline Code field resolves to its full path, not a raw index.**
  What a task or resource actually stores for `Outline Code1`..`Outline
  Code10` is a numeric ID into one shared, project-wide value tree (MS
  Project's Outline Code lookup table), not the field's own value — this
  reader follows that indirection and resolves it to the "parent \| child \|
  ..." path MS Project itself displays, so `CustomFields["Outline Code3"]`
  is a plain string like any other custom field.
- **Custom-field aliases and definitions live per entity, not per project.**
  Despite being genuinely project-wide data (one alias table covers task,
  resource and assignment fields alike), MS Project writes a separate copy
  of the alias/definition block to each entity's own Props stream
  (`TBkndTask/Props`, `TBkndRsc/Props`, `TBkndAssn/Props`) rather than the
  project-level one — this reader reads and merges all three.
- **A resource's cost rate table A is often absent from the file, not
  empty.** MS Project only writes a resource's five cost rate tables (A-E)
  when at least one differs from "the resource's own rate, always"; table A
  in particular is frequently left out entirely as an economy, in which
  case `CostRateTables[0]` is synthesized from `StandardRate`/
  `OvertimeRate`/`CostPerUse` as a single open-ended entry. Tables B-E stay
  nil when genuinely never customized — there's no equivalent fallback for
  them.
- **Timephased data is stored as cumulative totals, not per-span amounts.**
  MS Project writes each span's running total as of that span's end, not
  the amount for that span alone — every reader here (planned work,
  baseline work, baseline cost) subtracts the previous span's cumulative
  figure to recover it, which is also why a truncated or reordered data
  block would silently produce wrong amounts rather than an error: there is
  no per-span checksum to catch it against.
- **`Assignment.TimephasedWork` is remaining work, not always total work.**
  The single stream backing it means one thing before a task starts and
  another once it's partly done: MS Project reuses it to mean "work not yet
  done" the moment any progress is recorded, so its total can be less than
  `Assignment.Work` for an in-progress assignment — that is expected, not a
  sign of a misread file.
- **Calendar arithmetic here exists to interpret stored spans, not to
  compute new ones.** `Calendar.WorkMinutesBetween`/`NextWorkStart`/
  `AdvanceByWork` turn a calendar's working days/hours into the same kind of
  minute-level arithmetic MS Project's own scheduler uses internally, but
  this reader only calls them to reconstruct what a *stored* timephased
  span means (its per-hour rate, where a span's end actually falls) — it
  does not use them to schedule or reschedule anything itself. A resource's
  calendar wins over its task's when both apply; this reader does not
  reproduce MS Project's further step of intersecting the two when a task
  also carries its own explicit calendar.
- **An MSPDI duration's display unit is guessed from its own text, not
  read from a separate field.** `<Duration>PT24H0M0S</Duration>` carries no
  unit tag of its own; MS Project (and this reader, matching it exactly)
  picks the *largest* non-zero component the xsd:duration string itself
  contains — hours here, since neither days, months nor years are set —
  computes the value in that unit, and only then rescales it to the
  field's own `DurationFormat` (or the project's default). A duration
  expressed purely in elapsed clock time can therefore come out larger
  than it looks: 24 real hours is 3 working days at 8 hours each, not 1.
- **MSPDI custom Cost fields are scaled by 100; the built-in Cost fields
  are not.** An `ExtendedAttribute` value for a Cost-typed custom field is
  written in hundredths of a currency unit, the same convention the MPP
  binary format uses everywhere — but a task or resource's own built-in
  `<Cost>`/`<ActualCost>`/etc. elements are a plain decimal already. Mixing
  the two conventions up gives an answer 100x off in one direction or the
  other, so the two are deliberately handled by different code paths
  rather than one field reader used for both.
- **A document that declares a non-UTF-8 charset is read as Windows-1252,
  not rejected.** Go's XML decoder refuses a declared encoding it doesn't
  recognise unless a `CharsetReader` is supplied, and this project takes no
  external dependencies to do that conversion properly; MSPDI files that
  aren't UTF-8 are, in practice, essentially always Windows-1252 (an older
  MS Project export convention), so that's the one encoding this reader
  bothers to handle. A UTF-8-declared (or undeclared, XML's own default)
  document never touches this path at all — encoding/xml only invokes a
  CharsetReader for something other than UTF-8/US-ASCII in the first
  place.

## License

MIT — see [LICENSE](LICENSE). Format knowledge is attributed separately in
[NOTICE](NOTICE).
