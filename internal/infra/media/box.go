package media

import (
	"encoding/binary"
	"fmt"
	"io"
)

// box is one ISO base media file box: a length, a four-character type, and the
// bytes between them.
//
// `size` is always the whole box including its header, resolved at read time, and
// `headerLength` says how wide that header turned out to be. Both are settled here
// rather than by every reader because the format has three ways of writing the
// length and the payload always begins after whichever one was used.
type box struct {
	boxType      string
	start        int64
	size         int64
	headerLength int64
}

// payloadStart is the first byte after the header, which is where this box's
// children or its fields begin.
func (b box) payloadStart() int64 { return b.start + b.headerLength }

// end is the first byte after the box.
func (b box) end() int64 { return b.start + b.size }

// boxReader reads boxes out of a source it can seek in.
//
// It is deliberately random access: a `moov` box is commonly written after the
// media data, so finding it means stepping over an `mdat` whose length is a number
// in its header. Nothing large is read — every access here is 8, 16 or 4 bytes.
type boxReader struct {
	source io.ReaderAt
	limit  int64
}

// header reads the box that starts at offset, resolving the length field.
//
// A box that runs past the end of the file is an error rather than something to
// clamp: the caller asked about a file of a stated size, and a box whose own
// length disagrees with that is the file being shorter than it was claimed to be.
func (r *boxReader) header(offset int64) (box, error) {
	raw, err := r.bytes(offset, offset+boxHeaderSize)
	if err != nil {
		return box{}, err
	}
	size := int64(binary.BigEndian.Uint32(raw[0:4]))
	boxType := string(raw[4:8])
	headerLength := int64(boxHeaderSize)
	switch size {
	case boxSizeToEndOfFile:
		// The writer did not know the length when it wrote the header, so the box
		// runs to wherever the file does.
		size = r.limit - offset
	case boxSizeLargeForm:
		large, err := r.bytes(offset+boxHeaderSize, offset+boxHeaderLarge)
		if err != nil {
			return box{}, err
		}
		headerLength = boxHeaderLarge
		size = int64(binary.BigEndian.Uint64(large))
	}
	if size < headerLength {
		return box{}, fmt.Errorf("box %q at offset %d declares %d bytes, less than its own header", boxType, offset, size)
	}
	if offset+size > r.limit {
		return box{}, fmt.Errorf("%w: box %q at offset %d runs past the end of the file", io.ErrUnexpectedEOF, boxType, offset)
	}
	return box{boxType: boxType, start: offset, size: size, headerLength: headerLength}, nil
}

// bytes reads a half-open range. A request that leaves the file is refused before
// the read, so a short read is never silently taken for the bytes asked for.
func (r *boxReader) bytes(start, end int64) ([]byte, error) {
	if start < 0 || end < start || end > r.limit {
		return nil, io.ErrUnexpectedEOF
	}
	buffer := make([]byte, end-start)
	if _, err := r.source.ReadAt(buffer, start); err != nil {
		return nil, err
	}
	return buffer, nil
}

func (r *boxReader) uint8(offset int64) (uint8, error) {
	raw, err := r.bytes(offset, offset+1)
	if err != nil {
		return 0, err
	}
	return raw[0], nil
}

func (r *boxReader) uint32(offset int64) (uint32, error) {
	raw, err := r.bytes(offset, offset+4)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(raw), nil
}

func (r *boxReader) uint64(offset int64) (uint64, error) {
	raw, err := r.bytes(offset, offset+8)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(raw), nil
}
