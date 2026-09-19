{ version, revision ? "unknown", release ? false }:

let
  versionMatch = builtins.match
    "((0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*))(-dev)?"
    version;
  baseVersion = builtins.elemAt versionMatch 0;
  cleanRevision = builtins.replaceStrings [ "-dirty" ] [ "" ] revision;
  dirty = cleanRevision != revision;
  gitRevision = builtins.match "[0-9a-f]{7,64}(-dirty)?" revision != null;
in
assert versionMatch != null;
assert gitRevision || revision == "unknown";
assert !release || (version == baseVersion && gitRevision && !dirty);
if release then
  baseVersion
else
  baseVersion + "-dev." + (
    if gitRevision then
      builtins.substring 0 12 cleanRevision + (if dirty then ".dirty" else "")
    else
      "unknown"
  )
