// Package media reads the facts a video file declares about itself.
//
// The preparation pipeline has already decided whether the bytes are the right
// bytes — the size and the sha256 are checked against the object store's own
// reading — so this package answers a different question: is what arrived a video
// at all, or an HTML error page that happened to have a `.mp4` name. Nothing here
// decodes pictures.
//
// It parses the ISO base media file format (MP4) directly rather than shelling out
// to FFmpeg. FFmpeg would be a second binary to install on every worker, its
// output format would become a parsing contract of its own, and the facts actually
// wanted — duration, dimensions, codec — are each an integer or a four-character
// code sitting at a fixed offset inside a box.
package media

import (
	"errors"
	"fmt"
	"io"
)

// Media is what a file declares about itself. Every field is best-effort: a value
// this package could not reach is left at its zero value rather than guessed, and
// `DurationMS == 0` is therefore "not known", not "zero length". That distinction
// is deliberate and is why a probe of a truncated file succeeds with no duration:
// completeness is the size and hash check's job, and a probe that failed here
// would be reporting a settled question as an open one.
type Media struct {
	Container  string `json:"container"`
	Brand      string `json:"brand,omitempty"`
	DurationMS int64  `json:"duration_ms"`

	HasVideo   bool   `json:"has_video"`
	VideoCodec string `json:"video_codec,omitempty"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
}

// ErrNotVideo is what a reader gets when the bytes are not an ISO base media file
// at all. It is a distinct sentinel because the caller answers it differently from
// an I/O failure: one means the address served something else, the other means the
// read broke.
var ErrNotVideo = errors.New("the bytes are not an ISO base media file")

// A box header is 8 bytes — a 32-bit length and a four-character type — except
// when the length field is the sentinel 1, which says the real length follows as a
// 64-bit number and makes the header 16.
const (
	boxHeaderSize  = 8
	boxHeaderLarge = 16

	// boxSizeToEndOfFile is the length field's way of saying "this box runs to the
	// end of the file", which a streaming writer uses when it cannot know the length
	// up front.
	boxSizeToEndOfFile = 0

	// boxSizeLargeForm is the length field's way of saying "the real length is the
	// 64-bit number after this header", for boxes too large for 32 bits.
	boxSizeLargeForm = 1
)

// Probe reads what `r` declares about itself.
//
// `r` is random access rather than a stream because a `moov` box is commonly
// written after the media data, so finding it means seeking past an `mdat` whose
// size is a number in its header — nothing large is read.
func Probe(r io.ReaderAt, size int64) (Media, error) {
	if r == nil {
		return Media{}, errors.New("media probe requires a reader")
	}
	if size <= 0 {
		return Media{}, fmt.Errorf("media probe requires a positive size, got %d", size)
	}
	reader := &boxReader{source: r, limit: size}
	first, err := reader.header(0)
	if err != nil {
		// Too few bytes to hold a box header, or a source that will not read them.
		// The first is "this is not a media file" and the second is reported as
		// itself, because a broken read and a served non-video are answered
		// differently by the caller.
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return Media{}, fmt.Errorf("%w: too short to hold a box header", ErrNotVideo)
		}
		return Media{}, err
	}
	if !isContainerBox(first.boxType) {
		return Media{}, fmt.Errorf("%w: first box is %q", ErrNotVideo, first.boxType)
	}
	media := Media{Container: "mp4", Brand: reader.brand(first)}
	for offset := int64(0); offset < size; {
		header, err := reader.header(offset)
		if err != nil {
			// The walk ran out of file. Whatever was read before it stands, so the
			// container is reported without the facts that would have come after.
			break
		}
		if header.boxType == boxMoov {
			reader.readMovie(header, &media)
			break
		}
		// A box header is at least eight bytes long, so this always advances: there
		// is no way for the walk to stand still on a malformed length.
		offset = header.end()
	}
	return media, nil
}

const (
	boxMoov = "moov"
	boxMvhd = "mvhd"
	boxTrak = "trak"
	boxTkhd = "tkhd"
	boxMdia = "mdia"
	boxMinf = "minf"
	boxStbl = "stbl"
	boxStsd = "stsd"
)

// isContainerBox reports whether a box type can begin an ISO base media file.
// `ftyp` is the ordinary one; `styp` and `moof` begin a fragmented segment, and
// `free`/`skip` are padding a writer may put first.
//
// `moov` is deliberately not on the list even though this package would happily
// read one: a file that begins with a movie box and no `ftyp` is not a file any
// writer produces, and admitting it would mean accepting a fragment somebody
// uploaded by hand as a video.
func isContainerBox(boxType string) bool {
	switch boxType {
	case "ftyp", "styp", "moof", "free", "skip", "wide":
		return true
	default:
		return false
	}
}

// brand reads the major brand out of an `ftyp`-shaped box. A box that is not one
// answers empty rather than something derived from whatever bytes are there.
func (r *boxReader) brand(header box) string {
	switch header.boxType {
	case "ftyp", "styp":
	default:
		return ""
	}
	payload := header.payloadStart()
	raw, err := r.bytes(payload, payload+4)
	if err != nil {
		return ""
	}
	return string(raw)
}

// readMovie walks the `moov` children and fills in what they say. Every failure is
// swallowed into "that fact is not known": a `moov` that is present but unreadable
// still tells the caller the file is a video, which is the question being asked.
func (r *boxReader) readMovie(moov box, media *Media) {
	for _, child := range r.children(moov) {
		switch child.boxType {
		case boxMvhd:
			media.DurationMS = r.durationMS(child)
		case boxTrak:
			r.readTrack(child, media)
		}
	}
}

// durationMS reads the movie header's timescale and duration.
//
// The header is versioned and the two versions differ in the width of the
// timestamps: version 1 widens creation, modification and duration to 64 bits,
// which moves every field after them. Reading a version-0 offset out of a
// version-1 box would land on a timestamp and produce a plausible number, so the
// version is checked rather than assumed — and a version neither of the two is
// refused rather than read with version 0's layout, which would be a guess
// presented as a fact.
func (r *boxReader) durationMS(mvhd box) int64 {
	body := mvhd.payloadStart()
	version, err := r.uint8(body)
	if err != nil {
		return 0
	}
	if version != 0 && version != 1 {
		return 0
	}
	var timescale uint32
	var duration uint64
	if version == 1 {
		if timescale, err = r.uint32(body + 20); err != nil {
			return 0
		}
		if duration, err = r.uint64(body + 24); err != nil {
			return 0
		}
	} else {
		if timescale, err = r.uint32(body + 12); err != nil {
			return 0
		}
		value, err := r.uint32(body + 16)
		if err != nil {
			return 0
		}
		duration = uint64(value)
	}
	if timescale == 0 {
		return 0
	}
	return int64(duration) * 1000 / int64(timescale)
}

// readTrack fills in the video track's codec and dimensions.
//
// A file with several tracks — video plus audio plus subtitles — has one `trak`
// each, and only the one whose handler is `vide` describes a picture. The handler
// is read rather than assuming track order, because assuming order is how an
// audio track's absent dimensions become "the video has no size".
func (r *boxReader) readTrack(trak box, media *Media) {
	var isVideo bool
	var codec string
	var width, height int
	for _, child := range r.children(trak) {
		switch child.boxType {
		case boxTkhd:
			// Held rather than published: the header comes before the handler that
			// says whether these numbers describe a picture or a sound.
			width, height = r.dimensions(child)
		case boxMdia:
			for _, mediaChild := range r.children(child) {
				switch mediaChild.boxType {
				case "hdlr":
					if r.handlerType(mediaChild) == "vide" {
						isVideo = true
					}
				case boxMinf:
					codec = r.videoCodec(mediaChild)
				}
			}
		}
	}
	if !isVideo {
		// An audio or subtitle track. It is not reported, and — importantly — it does
		// not clear what an earlier video track already reported.
		return
	}
	media.HasVideo = true
	media.Width, media.Height = width, height
	media.VideoCodec = codec
}

// handlerType reads the four-character handler type out of an `hdlr` box.
func (r *boxReader) handlerType(hdlr box) string {
	body := hdlr.payloadStart()
	// version+flags, creation, modification, then the handler type.
	raw, err := r.bytes(body+8, body+12)
	if err != nil {
		return ""
	}
	return string(raw)
}

// dimensions reads the track header's width and height, which are 16.16 fixed
// point and sit at the very end of the header.
//
// Version 1 widens the two timestamps at the front from 32 to 64 bits, which moves
// everything after them by 12 bytes — so the width field is at 76 for a version-0
// header and 88 for a version-1 one. Reading one with the other's offset lands
// inside the transformation matrix and yields a plausible small number, which is
// why the version is read rather than assumed.
func (r *boxReader) dimensions(tkhd box) (int, int) {
	body := tkhd.payloadStart()
	version, err := r.uint8(body)
	if err != nil {
		return 0, 0
	}
	if version != 0 && version != 1 {
		return 0, 0
	}
	offset := body + 76
	if version == 1 {
		offset = body + 88
	}
	width, err := r.uint32(offset)
	if err != nil {
		return 0, 0
	}
	height, err := r.uint32(offset + 4)
	if err != nil {
		return 0, 0
	}
	return int(width >> 16), int(height >> 16)
}

// videoCodec reads the four-character code of the first sample description inside
// a track's sample table. It is the codec's common name (`avc1`, `hvc1`, …) and is
// reported as-is rather than mapped, because a name this package has not heard of
// is still the truth about the file.
func (r *boxReader) videoCodec(minf box) string {
	for _, child := range r.children(minf) {
		if child.boxType != boxStbl {
			continue
		}
		for _, stblChild := range r.children(child) {
			if stblChild.boxType != boxStsd {
				continue
			}
			body := stblChild.payloadStart()
			// version+flags(4), entry count(4), then the first entry: size(4) type(4).
			raw, err := r.bytes(body+12, body+16)
			if err != nil {
				return ""
			}
			return string(raw)
		}
	}
	return ""
}

// children walks a box's direct children.
//
// It stops at the first thing it cannot read rather than reporting a failure: a
// parent whose children are unreadable still answered the question this package is
// asked — the file is a video — and refusing the whole probe over a field nobody
// needs would turn a readable file into a rejected one.
func (r *boxReader) children(parent box) []box {
	children := make([]box, 0, 4)
	for offset := parent.payloadStart(); offset < parent.end(); {
		header, err := r.header(offset)
		if err != nil {
			return children
		}
		children = append(children, header)
		// As in `Probe`, a header is at least eight bytes, so this advances.
		offset = header.end()
	}
	return children
}
