# Slice library CLI

Run from the Cloud checkout with `go run ./cmd/slice-video`, or use the released
`bin/slice-video`. It loads the same Cloud config/credentials as other workers,
without starting discovery/scheduler loops. Apply migrations first.

`media/slice_library.toml` configures bounded inputs, timeout, encoding and pool
prefix. The source download, contact sheet, audio and review JSON stay in a local
run directory; no cloud temporary prefix is used.

```sh
bin/slice-video prepare --keyword '动漫舞蹈 配乐' --count 1 --offset 0 \
  --dir /absolute/local/run --report /absolute/local/run/prepare.json
bin/slice-video publish --candidate /absolute/local/run/candidate-xxx/candidate.json \
  --decision /absolute/local/run/review.json --max-clips 2 \
  --report /absolute/local/run/published.json
bin/slice-video verify --report /absolute/local/run/verified.json
```

`list` reads the published index without downloading the library. `verify --source <video-ID>` audits one source; omit source for an explicit full-library audit. Normal additions need only audit their new sources.

The Skill inspects `frames.jpg` and available `audio.wav` / media facts before
writing a review. A review contains `source_id`, `sha256`, `suitable` (boolean),
`reviewed_seconds` (from the candidate), `short_title` and `reason`. An unsuitable review removes that local candidate and
publishes nothing. A suitable one is bound to the exact downloaded bytes and sampled range. Its frozen clip profile cannot expand when the publication configuration changes.

The database reserves a source's short basename and original rendering profile.
Clips retain their original audio, complete durations only, names `title_1.mp4`,
`title_2.mp4`. Another source with that basename receives a source-ID suffix.
Identical source retry reuses the first profile and ready clips. Changed bytes
under the same source ID are rejected. Failures retain local artifacts for retry;
a successful run can remove its local run directory after verification.

A dedicated DB connection holds one advisory lock across publication. Each clip
is decoded and read back with SHA256 before membership is saved. The index is
rebuilt from verified sources and read back before success; failed index writes
are retried by publishing the same reviewed candidate again. The bucket remains
private; the index stores durable S3 references, never credentials or CDN URLs.
