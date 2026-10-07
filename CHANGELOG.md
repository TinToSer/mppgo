# Changelog

## Unreleased

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