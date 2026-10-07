# Changelog

## Unreleased

### 2026-10-07 (complete field coverage)

#### Added

- Every task, resource and assignment field an MPP file's field map
  locates is decoded into the new `Fields` maps, by name, from generated
  field definitions (`mpp/fielddefs_gen.go`, from MPXJ's tables; see
  NOTICE). Field-map entries are block-aware and meta-data flags are no
  longer mistaken for fixed data.
- Typed task fields: GUID, task mode and manual duration, total slack,
  critical, estimated, stop/resume, leveling delay, contact, hyperlink,
  earned value (BCWS/BCWP/ACWP, method, physical % complete), regular and
  overtime work/cost, fixed cost accrual, effort driven, rollup, hide bar,
  marked, leveling flags, ignore resource calendar, subproject/external
  markers and the recurring-task pattern.
- Typed resource fields: GUID, generic, budget, can level, accrue at,
  phonetics, Windows account, material label, booking type, hyperlink,
  rate units, peak units, actual/remaining/regular/overtime work and cost,
  earned value, start/finish, availability window and creation date.
  Baseline Work/Cost are read wherever the file stores them.
- Typed assignment fields: GUID, cost, actual/remaining/overtime cost and
  work, earned value, % work complete, actual start/finish, stop/resume,
  delay, leveling delay, work contour, cost rate table, confirmed, response
  pending, hyperlink, created, and custom fields (Text/Start/Finish/Number/
  Date/Duration/Cost/Flag) incl. the alternate keys some files use.
- Timephased actual work, with irregular work ranges spliced in, and actual
  overtime work. Planned work after progress starts at the Resume date.
  MS Project's implied flat span is supplied when an assignment stores
  none.
- `File.ProjectSummaryTask`: the unique-ID-0 task with project rollups,
  from both readers and written back to Project XML.
- Project properties: GUID, OLE summary and custom document properties,
  creation/saved/printed dates, revision, schedule-from, default start/end
  times, default units/task type/rates, critical slack limit, currency,
  week start and fiscal year, honour constraints, split/updating options,
  new-tasks-manual, baseline calendar and baseline dates, resource pool.
- Calendar work weeks (`Calendar.WorkWeeks`), resolved by `WorkingOn`/
  `HoursOn`, from MPP and Project XML and written to Project XML.
- Custom field lookup-table values: fields that store a lookup reference
  resolve to the value.
- RTF notes honour `\ansicpg` and per-font `\fcharset` code pages.
- Project XML reads and writes all of the above.

#### Changed

- `Resource.StandardRate`/`OvertimeRate` are now in their displayed rate
  units (`StandardRateUnits`/`OvertimeRateUnits`), like the cost rate
  tables; they were per hour.
- Assignments with no resource are included, with `ResourceUniqueID` 0, by
  both readers. Duplicate task/resource assignments, assignments of tasks
  that were not read, and resources with no var data (deleted leftovers)
  are dropped, as MPXJ does.
- A manually scheduled task's `Duration` is its manual duration.
- The Project XML reader returns the UID 0 task as `ProjectSummaryTask`.
- Custom-field aliases that collide keep the later field's generic name
  instead of overwriting.

#### Fixed

- Recurring calendar exceptions other than daily and yearly-by-date use
  MPXJ's verified record layout (weekly/monthly frequency and weekday
  offsets).
- RTF field results (hyperlink text in notes) are no longer dropped.
- Calendar lookups are faster.

### 2026-10-07 (review fixes)

#### Fixed

- MPP task `Start`/`Finish` are read from the file's primary Start/Finish
  fields. The secondary copy used before is blank for summary tasks and, in
  some files, for every task.
- MPP field-map lookups track which fixed-data block each field lives in,
  and no longer fall back to a hardcoded offset when the file's own map
  places the field elsewhere (that offset belongs to another field).
  Resource Baseline Work/Cost are left unset when the file stores them as
  var data, rather than read from unrelated bytes.
- Recurring calendar exceptions (a holiday every 1 January, ...) are
  expanded into their individual occurrences, by both the MPP and the
  Project XML reader. They were read as one
  continuous exception, making every day between the first and last
  occurrence non-working. Expansion is bounded for corrupt files, and day
  and exception working-period counts are capped at the five stored slots.
- Compound files with 4096-byte sectors read sectors from the correct file
  offset, and empty streams no longer return another stream's data.
- Project XML export: elements follow MSPDI schema order; durations are
  written as `PTnHnMnS`; tasks and resources without their own calendar no
  longer claim calendar UID 0; cost-rate tables convert between the model's
  displayed-unit rates and MSPDI's per-hour rates in both directions; alias
  definitions are written only for fields with a known definition.
- The Project XML reader leaves out the UID 0 project summary task, as the
  MPP reader does, and reads task `CalendarUID` -1 as no task calendar.
- Calendar exception lookups no longer rebuild time values per exception
  per scanned day.

### 2026-10-07

#### Added

- `cfb.PatchStreams` for transactional, same-size stream edits with allocation
  validation, overlap rejection, unrelated-byte preservation and read-back
  verification.
- `mpp.Patch`, `NativeEdit` and `NativeWritableFields` for verified native
  MPP14 template edits. Supported fields include task name, stored WBS, notes
  and priority; resource identity fields and notes; existing Text, Number and
  Cost custom fields; and Flag fields. Custom-field aliases are supported.
- `mpp.ReadRawStream`, `Props.Keys` and `VarMeta.UniqueIDs` for inspecting
  stored streams, properties and variable-record inventories, including data
  not decoded by the typed model.
- `mspdi.Write` and no-overwrite `mspdi.WriteFile` for Project XML model
  exports covering properties, calendars, hierarchy, tasks, resources,
  assignments, dependencies, baselines and supported custom fields.
- XML reading/writing for resource rate tables A-E, availability periods,
  numbered resource baseline dates, and assignment remaining, actual and
  baseline timephased work/cost.
- Custom-field alias retention through `project.File.CustomFieldAliases`,
  calendar GUID retention, and additive actual/raw assignment timephased
  model fields. Unknown assignment XML timephased types remain available as
  raw records and are retained during re-export.
- Synthetic compound-container tests, native MPP fixture integration tests,
  XML round-trip/no-overwrite tests and focused encoding regressions.

#### Fixed

- XML custom-field lookup now accepts full Microsoft Project field IDs in
  addition to the existing low-ID representation.
- Native RTF notes correctly combine UTF-16 surrogate pairs for supplementary
  Unicode characters.
- XML elapsed durations use fixed calendar-time conversion factors rather
  than the project's working-day factors.
- XML percentage dependency lag is retained without conversion to tenths
  of a minute.

#### Documentation

- Updated the README with native/XML write examples, supported APIs, raw
  inspection capabilities and explicit format limitations.

#### Limitations

- Native writes require an existing MPP14 template using 512-byte CFB sectors
  and 64-byte mini sectors. Variable values must already exist, fit their
  capacity and not share a record with another field. Structural edits,
  variable-record growth and scheduling-field changes are rejected.
- Every native patch is reopened and compared with the intended complete
  parsed model. This does not certify compatibility with the Microsoft
  Project UI; validate output copies before production use.
- Project XML export is a model export, not a lossless MPP/XML source
  round trip. Unmodelled features, RTF formatting and unknown XML elements
  are not retained. Unsupported custom-field definitions fail explicitly;
  timephased per-hour rates may be rederived from totals and calendars.
- No scheduling engine or external runtime dependency is introduced.
  Legacy/encrypted MPP formats, actual MPP timephased work, named calendar
  work weeks and universal proprietary-format coverage remain unsupported.