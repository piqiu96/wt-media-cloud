package media

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

// The fixtures below are real boxes rather than a stand-in for the parser: a box
// is a length, a four-character type and a payload, and writing one takes three
// lines. Building them by hand is also what makes the offsets in the tests
// independent of the offsets in the implementation — a fixture that called the
// code under test to lay itself out would agree with any bug.

// box builds a box with the ordinary 32-bit length.
func boxOf(boxType string, payload []byte) []byte {
	out := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(out[0:4], uint32(8+len(payload)))
	copy(out[4:8], boxType)
	copy(out[8:], payload)
	return out
}

// boxOfLarge builds a box that uses the 64-bit length form, which a writer uses when
// the box could exceed four gigabytes.
func boxOfLarge(boxType string, payload []byte) []byte {
	out := make([]byte, 16+len(payload))
	binary.BigEndian.PutUint32(out[0:4], 1)
	copy(out[4:8], boxType)
	binary.BigEndian.PutUint64(out[8:16], uint64(16+len(payload)))
	copy(out[16:], payload)
	return out
}

// boxOfOpenEnded builds a box whose length field is zero: "this box runs to the end
// of the file", which a streaming writer emits when it cannot know the length yet.
func boxOfOpenEnded(boxType string, payload []byte) []byte {
	out := make([]byte, 8+len(payload))
	copy(out[4:8], boxType)
	copy(out[8:], payload)
	return out
}

func join(parts ...[]byte) []byte { return bytes.Join(parts, nil) }

func ftyp(brand string) []byte {
	payload := make([]byte, 8)
	copy(payload[0:4], brand)
	copy(payload[4:8], "isom")
	return boxOf("ftyp", payload)
}

func mvhd(version byte, timescale uint32, duration uint64) []byte {
	if version == 1 {
		body := make([]byte, 112)
		body[0] = version
		binary.BigEndian.PutUint32(body[20:24], timescale)
		binary.BigEndian.PutUint64(body[24:32], duration)
		return boxOf("mvhd", body)
	}
	body := make([]byte, 100)
	body[0] = version
	binary.BigEndian.PutUint32(body[12:16], timescale)
	binary.BigEndian.PutUint32(body[16:20], uint32(duration))
	return boxOf("mvhd", body)
}

func tkhd(version byte, width, height uint16) []byte {
	body := make([]byte, 84)
	matrix := 40
	if version == 1 {
		body = make([]byte, 96)
		matrix = 52
	}
	body[0] = version
	// The identity matrix, which is what a real file carries there and what makes
	// the version-dependent offsets matter: read with the wrong version's offset,
	// the size fields land inside this matrix and yield its numbers instead.
	binary.BigEndian.PutUint32(body[matrix:matrix+4], 0x00010000)
	binary.BigEndian.PutUint32(body[matrix+16:matrix+20], 0x00010000)
	binary.BigEndian.PutUint32(body[matrix+32:matrix+36], 0x40000000)
	offset := matrix + 36
	binary.BigEndian.PutUint32(body[offset:offset+4], uint32(width)<<16)
	binary.BigEndian.PutUint32(body[offset+4:offset+8], uint32(height)<<16)
	return boxOf("tkhd", body)
}

func hdlr(handler string) []byte {
	body := make([]byte, 24)
	copy(body[8:12], handler)
	return boxOf("hdlr", body)
}

func stsd(codec string) []byte {
	body := make([]byte, 16)
	binary.BigEndian.PutUint32(body[4:8], 1)
	copy(body[12:16], codec)
	return boxOf("stsd", body)
}

func track(version byte, handler, codec string, width, height uint16) []byte {
	return boxOf("trak", join(
		tkhd(version, width, height),
		boxOf("mdia", join(hdlr(handler), boxOf("minf", boxOf("stbl", stsd(codec))))),
	))
}

func movie(parts ...[]byte) []byte { return boxOf("moov", join(parts...)) }

func probeOK(t *testing.T, data []byte) Media {
	t.Helper()
	got, err := Probe(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Probe() error = %v, want the file to be readable", err)
	}
	return got
}

func TestProbeReadsAMinimalFile(t *testing.T) {
	// The major brand and the compatible brand differ here on purpose: they are
	// adjacent four-byte fields, and a probe that read the second would report a
	// brand this file does not claim.
	data := join(ftyp("mp42"), movie(mvhd(0, 1000, 5000), track(0, "vide", "avc1", 1920, 1080)))

	got := probeOK(t, data)
	if got.Container != "mp4" {
		t.Fatalf("Container = %q", got.Container)
	}
	if got.Brand != "mp42" {
		t.Fatalf("Brand = %q, want the major brand", got.Brand)
	}
	if got.DurationMS != 5000 {
		t.Fatalf("DurationMS = %d, want 5000", got.DurationMS)
	}
	if !got.HasVideo {
		t.Fatal("HasVideo = false, want true for a file with a video track")
	}
	if got.Width != 1920 || got.Height != 1080 {
		t.Fatalf("size = %dx%d, want 1920x1080", got.Width, got.Height)
	}
	if got.VideoCodec != "avc1" {
		t.Fatalf("VideoCodec = %q, want the sample description's four-character code", got.VideoCodec)
	}
}

// A `moov` written after the media data is the ordinary layout for a file that was
// produced by streaming rather than by a muxer holding everything in memory, and
// it is the reason this package seeks.
func TestProbeFindsTheMovieAfterTheMediaData(t *testing.T) {
	data := join(
		ftyp("isom"),
		boxOf("mdat", make([]byte, 4096)),
		movie(mvhd(0, 1000, 12000), track(0, "vide", "hvc1", 720, 480)),
	)

	got := probeOK(t, data)
	if got.DurationMS != 12000 {
		t.Fatalf("DurationMS = %d, want the movie box at the end of the file to be found", got.DurationMS)
	}
	if got.Width != 720 || got.Height != 480 {
		t.Fatalf("size = %dx%d, want 720x480", got.Width, got.Height)
	}
	if got.VideoCodec != "hvc1" {
		t.Fatalf("VideoCodec = %q", got.VideoCodec)
	}
}

// Both header versions, because they differ in where every field after the first
// timestamp sits and a file does not say which one it used anywhere but here.
func TestProbeReadsBothHeaderVersions(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		version   byte
		timescale uint32
		duration  uint64
		wantMS    int64
	}{
		// 5000 at a timescale of 1000 is five seconds; 900000 at 30000 is thirty.
		{"version 0", 0, 1000, 5000, 5000},
		{"version 1", 1, 30000, 900000, 30000},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			data := join(
				ftyp("isom"),
				movie(
					mvhd(testCase.version, testCase.timescale, testCase.duration),
					track(testCase.version, "vide", "avc1", 1280, 720),
				),
			)

			got := probeOK(t, data)
			if got.DurationMS != testCase.wantMS {
				t.Fatalf("DurationMS = %d, want %d", got.DurationMS, testCase.wantMS)
			}
			if got.Width != 1280 || got.Height != 720 {
				t.Fatalf("size = %dx%d, want 1280x720", got.Width, got.Height)
			}
		})
	}
}

// The 64-bit length form exists for boxes over four gigabytes, and a parser that
// read the sentinel `1` as a length would step one byte past the header and lose
// everything after it.
func TestProbeWalksPastABoxWithA64BitLength(t *testing.T) {
	data := join(
		ftyp("isom"),
		boxOfLarge("mdat", make([]byte, 64)),
		movie(mvhd(0, 1000, 2000), track(0, "vide", "avc1", 640, 360)),
	)

	got := probeOK(t, data)
	if got.DurationMS != 2000 {
		t.Fatalf("DurationMS = %d, want the movie box after the large-format one", got.DurationMS)
	}
	if got.Width != 640 || got.Height != 360 {
		t.Fatalf("size = %dx%d, want 640x360", got.Width, got.Height)
	}
}

func TestProbeReadsAMovieThatRunsToTheEndOfTheFile(t *testing.T) {
	data := join(
		ftyp("isom"),
		boxOfOpenEnded("moov", join(mvhd(0, 1000, 3000), track(0, "vide", "avc1", 320, 240))),
	)

	got := probeOK(t, data)
	if got.DurationMS != 3000 {
		t.Fatalf("DurationMS = %d, want the movie box's own length to be taken from the file's", got.DurationMS)
	}
	if got.Width != 320 || got.Height != 240 {
		t.Fatalf("size = %dx%d, want 320x240", got.Width, got.Height)
	}
}

// A large-format header is 16 bytes rather than 8, so a movie box written with one
// begins its children eight bytes later. A reader that always started a payload at
// the fixed header size would walk the 64-bit length field as if it were a box of
// its own and find nothing.
func TestProbeReadsAMovieWrittenWithA64BitLength(t *testing.T) {
	data := join(
		ftyp("isom"),
		boxOfLarge("moov", join(mvhd(0, 1000, 7000), track(0, "vide", "avc1", 640, 480))),
	)

	got := probeOK(t, data)
	if got.DurationMS != 7000 {
		t.Fatalf("DurationMS = %d, want the movie's children to start after the wide header", got.DurationMS)
	}
	if got.Width != 640 || got.Height != 480 {
		t.Fatalf("size = %dx%d, want 640x480", got.Width, got.Height)
	}
}

// The track header is versioned the same way, and an unknown version gets the same
// answer: the picture is reported, because the handler said so, and its size is not
// invented from a layout nobody has agreed on.
func TestProbeReportsNoSizeForATrackHeaderVersionNothingDefines(t *testing.T) {
	data := join(ftyp("isom"), movie(mvhd(0, 1000, 5000), track(2, "vide", "avc1", 1280, 720)))

	got := probeOK(t, data)
	if !got.HasVideo {
		t.Fatal("HasVideo = false, want the handler believed even where the header's layout is not")
	}
	if got.Width != 0 || got.Height != 0 {
		t.Fatalf("size = %dx%d, want none for a header version nothing defines", got.Width, got.Height)
	}
	if got.VideoCodec != "avc1" {
		t.Fatalf("VideoCodec = %q, want the sample description's code to stand", got.VideoCodec)
	}
}

// Padding before the file type is legal, and the brand is then genuinely unknown —
// reading four bytes from whatever the first box happens to be would invent one.
func TestProbeAcceptsPaddingBeforeTheFileType(t *testing.T) {
	data := join(
		boxOf("free", []byte("padding!")),
		ftyp("isom"),
		movie(mvhd(0, 1000, 1000), track(0, "vide", "avc1", 16, 16)),
	)

	got := probeOK(t, data)
	if got.Brand != "" {
		t.Fatalf("Brand = %q, want empty when the file type is not the first box", got.Brand)
	}
	if got.DurationMS != 1000 {
		t.Fatalf("DurationMS = %d, want the walk to continue past the padding", got.DurationMS)
	}
}

// A track is a picture only if its media handler says so, and track order is not
// the answer: a file may put its audio first. The audio fixtures carry non-zero
// dimensions on purpose, so a probe that read the first track's header would
// report the audio track's numbers rather than failing to report anything.
func TestProbeOnlyReportsATrackThatCarriesAPicture(t *testing.T) {
	t.Run("audio alone", func(t *testing.T) {
		data := join(ftyp("isom"), movie(mvhd(0, 1000, 4000), track(0, "soun", "mp4a", 111, 222)))

		got := probeOK(t, data)
		if got.DurationMS != 4000 {
			t.Fatalf("DurationMS = %d, want the duration even with no picture", got.DurationMS)
		}
		if got.HasVideo {
			t.Fatal("HasVideo = true, want false for a file whose only track is audio")
		}
		if got.VideoCodec != "" || got.Width != 0 || got.Height != 0 {
			t.Fatalf("video facts = %q %dx%d, want them unreported", got.VideoCodec, got.Width, got.Height)
		}
	})

	t.Run("audio before video", func(t *testing.T) {
		data := join(ftyp("isom"), movie(
			mvhd(0, 1000, 4000),
			track(0, "soun", "mp4a", 111, 222),
			track(0, "vide", "avc1", 1280, 720),
		))

		got := probeOK(t, data)
		if !got.HasVideo || got.Width != 1280 || got.Height != 720 || got.VideoCodec != "avc1" {
			t.Fatalf("video facts = %t %q %dx%d, want the second track's", got.HasVideo, got.VideoCodec, got.Width, got.Height)
		}
	})

	t.Run("audio after video", func(t *testing.T) {
		data := join(ftyp("isom"), movie(
			mvhd(0, 1000, 4000),
			track(0, "vide", "avc1", 1280, 720),
			track(0, "soun", "mp4a", 111, 222),
		))

		got := probeOK(t, data)
		if !got.HasVideo || got.Width != 1280 || got.Height != 720 {
			t.Fatalf("video facts = %t %dx%d, want the video track's to stand", got.HasVideo, got.Width, got.Height)
		}
	})
}

func TestProbeRefusesBytesThatAreNotAMediaFile(t *testing.T) {
	for _, testCase := range []struct {
		name string
		data []byte
	}{
		{"an html error page", []byte("<!DOCTYPE html><html><body>403 Forbidden</body></html>")},
		{"a png header", []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")},
		{"too short to hold a box header", []byte("mp4")},
		{"a box type nothing recognises", boxOf("html", []byte("<html>"))},
		// A movie box on its own is not a file: real writers always emit a file type
		// first, and accepting this would accept a hand-built fragment as a video.
		{"a movie box with no file type", movie(mvhd(0, 1000, 1000))},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := Probe(bytes.NewReader(testCase.data), int64(len(testCase.data))); !errors.Is(err, ErrNotVideo) {
				t.Fatalf("Probe() error = %v, want ErrNotVideo", err)
			}
		})
	}
}

// A file that stops early is not a rejection: the container is readable, and the
// facts that would have followed are reported as unknown. Whether the file is
// complete is the size and hash check's question, and this package exists so as
// not to answer it twice.
func TestProbeReportsWhatItCanFromATruncatedFile(t *testing.T) {
	// A second box whose declared length is larger than the file: the download was
	// cut short, or the writer stopped before it wrote what it had promised.
	truncated := []byte{0x00, 0x00, 0x10, 0x00, 'm', 'o', 'o', 'v'}
	data := join(ftyp("isom"), truncated)

	got := probeOK(t, data)
	if got.Container != "mp4" {
		t.Fatalf("Container = %q, want the readable part reported", got.Container)
	}
	if got.DurationMS != 0 {
		t.Fatalf("DurationMS = %d, want 0 when the movie box is not there to be read", got.DurationMS)
	}
	if got.HasVideo {
		t.Fatal("HasVideo = true, want false when no track was read")
	}
}

func TestProbeReportsAnUnknownDurationAsZero(t *testing.T) {
	for _, testCase := range []struct {
		name string
		data []byte
	}{
		{"a zero timescale", join(ftyp("isom"), movie(mvhd(0, 0, 5000)))},
		// Version 2 is not defined by the format, and this fixture puts the numbers
		// a version-0 header would carry exactly where a version-0 header carries
		// them: reading on regardless would report a duration for a layout nobody
		// has agreed on.
		{"a header version nothing defines", join(ftyp("isom"), movie(mvhd(2, 1000, 5000)))},
		{"a movie box with no movie header", join(ftyp("isom"), boxOf("moov", nil))},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got := probeOK(t, testCase.data)
			if got.DurationMS != 0 {
				t.Fatalf("DurationMS = %d, want 0 for a duration that is not known", got.DurationMS)
			}
		})
	}
}

// A box that declares more bytes than the file holds is a refusal, not a licence
// to read what the file does contain: the declared length and the actual length
// disagree, and this package's answer is the one that does not depend on which of
// them is right. This is what a download cut short looks like.
func TestProbeRefusesABoxThatRunsPastTheEndOfTheFile(t *testing.T) {
	body := join(mvhd(0, 1000, 5000), track(0, "vide", "avc1", 1920, 1080))
	oversized := make([]byte, 8)
	binary.BigEndian.PutUint32(oversized[0:4], uint32(8+len(body)+4096))
	copy(oversized[4:8], "moov")
	data := join(ftyp("isom"), oversized, body)

	got := probeOK(t, data)
	if got.DurationMS != 0 {
		t.Fatalf("DurationMS = %d, want an oversize movie box refused rather than read into", got.DurationMS)
	}
}

// A box's payload is not scanned for further boxes. A declared length too small to
// hold the box's own header is refused, and the fixture is shaped to make that
// refusal observable: the movie box starts exactly where a walk stepping by the
// declared length of four — 16 + 4 — would land, so a walk that took the length at
// face value would read the movie box and report its duration.
func TestProbeDoesNotScanInsideABoxForMoreBoxes(t *testing.T) {
	tooSmall := make([]byte, 4)
	binary.BigEndian.PutUint32(tooSmall, 4)
	hidden := movie(mvhd(0, 1000, 5000), track(0, "vide", "avc1", 1920, 1080))
	data := join(ftyp("isom"), tooSmall, hidden)

	got := probeOK(t, data)
	if got.DurationMS != 0 {
		t.Fatalf("DurationMS = %d, want the walk to stop rather than step inside the box", got.DurationMS)
	}
}

// The reader's range check, tested where it lives rather than through a parser that
// would refuse the input anyway. The backwards range is the case that is load
// bearing: `ReadAt` refuses a range past the end of the file by itself, but nothing
// but this check stops a negative-length slice being made.
func TestBoxReaderRefusesARangeItCannotServe(t *testing.T) {
	reader := &boxReader{source: bytes.NewReader(make([]byte, 32)), limit: 32}
	for _, testCase := range []struct {
		name       string
		start, end int64
	}{
		{"past the end of the file", 16, 48},
		{"backwards", 16, 8},
		{"negative", -1, 4},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := reader.bytes(testCase.start, testCase.end); err == nil {
				t.Fatalf("bytes(%d, %d) error = nil, want a refusal", testCase.start, testCase.end)
			}
		})
	}
}

// A field read is bounded by the file, not by the box it belongs to. This pins
// that deliberately: a malformed movie header can pick up the bytes of the box
// after it and report a duration that means nothing. The alternative — threading
// every box's end through every field read — would add a code path no valid file
// exercises, and the file has already been judged on its size and its hash.
func TestProbeDoesNotBoundAFieldReadToItsOwnBox(t *testing.T) {
	// A movie header holding nothing but its version, then a padding box whose
	// payload sits exactly where a movie header's timescale and duration would:
	// both are inside the movie box, so the walk reaches the header, and the
	// header's own length stops four bytes in.
	next := make([]byte, 8)
	binary.BigEndian.PutUint32(next[0:4], 1000)
	binary.BigEndian.PutUint32(next[4:8], 5000)
	data := join(ftyp("isom"), movie(boxOf("mvhd", []byte{0, 0, 0, 0}), boxOf("free", next)))

	got := probeOK(t, data)
	if got.DurationMS != 5000 {
		t.Fatalf("DurationMS = %d, want the following box's bytes to be read as this one's fields", got.DurationMS)
	}
}

func TestProbeRefusesAnInputItCannotUse(t *testing.T) {
	if _, err := Probe(nil, 16); err == nil {
		t.Fatal("Probe(nil, 16) error = nil, want a refusal")
	}
	for _, size := range []int64{0, -1} {
		// A size nobody measured is refused as a bad input, and reporting it as
		// ErrNotVideo would be a lie about the bytes: an empty reading and a served
		// error page are answered differently by the caller.
		if _, err := Probe(bytes.NewReader(make([]byte, 32)), size); err == nil {
			t.Fatalf("Probe(size = %d) error = nil, want a refusal", size)
		} else if errors.Is(err, ErrNotVideo) {
			t.Fatalf("Probe(size = %d) error = %v, want the input refused rather than the bytes judged", size, err)
		}
	}
}
