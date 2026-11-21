package reqresp

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/golang/snappy"
)

// ErrEmptyResponse reports a cleanly closed response stream that contained no
// response chunks. It unwraps to io.EOF for callers that classify stream ends.
var ErrEmptyResponse = fmt.Errorf("empty response stream: %w", io.EOF)

// WriteSSZSnappy writes one length-prefixed framed-snappy request body.
func WriteSSZSnappy(w io.Writer, payload []byte) error {
	var prefix [10]byte
	n := binary.PutUvarint(prefix[:], uint64(len(payload)))
	if err := writeFull(w, prefix[:n]); err != nil {
		return fmt.Errorf("write ssz length prefix: %w", err)
	}
	writer := snappy.NewBufferedWriter(fullWriter{Writer: w})
	if _, err := writer.Write(payload); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write snappy payload: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close snappy writer: %w", err)
	}
	return nil
}

func writeFull(w io.Writer, payload []byte) error {
	for len(payload) > 0 {
		n, err := w.Write(payload)
		if n < 0 || n > len(payload) {
			return fmt.Errorf("invalid write count %d for %d-byte buffer", n, len(payload))
		}
		payload = payload[n:]
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}

// fullWriter adapts writers that legally return short writes so codecs which
// assume an all-or-error Write contract cannot silently truncate a frame.
type fullWriter struct{ io.Writer }

func (w fullWriter) Write(payload []byte) (int, error) {
	if err := writeFull(w.Writer, payload); err != nil {
		return 0, err
	}
	return len(payload), nil
}

// RawSSZSnappyWithVarint frames payload after an exact caller-supplied prefix.
func RawSSZSnappyWithVarint(varintPrefix, payload []byte) ([]byte, error) {
	var out bytes.Buffer
	out.Grow(len(varintPrefix) + snappy.MaxEncodedLen(len(payload)) + 32)
	_, _ = out.Write(varintPrefix)
	writer := snappy.NewBufferedWriter(&out)
	if _, err := writer.Write(payload); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// RawSSZSnappy returns one canonical length-prefixed framed-snappy body.
func RawSSZSnappy(payload []byte) ([]byte, error) {
	var prefix [10]byte
	n := binary.PutUvarint(prefix[:], uint64(len(payload)))
	return RawSSZSnappyWithVarint(prefix[:n], payload)
}

// ReadResponseChunk decodes exactly one bounded response chunk.
func ReadResponseChunk(r io.Reader, hasContext bool, maxPayload uint64) (*ResponseChunk, error) {
	responses, err := NewResponseReader(r, hasContext, maxPayload, maxPayload, 1)
	if err != nil {
		return nil, err
	}
	response, err := responses.Next()
	if errors.Is(err, io.EOF) {
		return nil, ErrEmptyResponse
	}
	return response, err
}

// ResponseReader decodes consecutive response chunks through one persistent
// buffer while enforcing per-chunk, aggregate-byte, and chunk-count ceilings.
type ResponseReader struct {
	reader     *bufio.Reader
	hasContext bool
	maxPayload uint64
	maxTotal   uint64
	maxChunks  uint64
	total      uint64
	chunks     uint64
}

// NewResponseReader creates a bounded decoder for a sequence of response chunks.
func NewResponseReader(r io.Reader, hasContext bool, maxPayload, maxTotal, maxChunks uint64) (*ResponseReader, error) {
	if r == nil {
		return nil, errors.New("response reader is nil")
	}
	if maxPayload > uint64(MaxPayloadSize) {
		return nil, fmt.Errorf("max response payload %d exceeds hard ceiling %d", maxPayload, MaxPayloadSize)
	}
	if maxTotal > MaxTotalResponseSize {
		return nil, fmt.Errorf("max total response payload %d exceeds hard ceiling %d", maxTotal, MaxTotalResponseSize)
	}
	if maxChunks == 0 || maxChunks > MaxResponseChunks {
		return nil, fmt.Errorf("max response chunks %d is outside 1..%d", maxChunks, MaxResponseChunks)
	}
	if maxPayload == 0 || maxTotal == 0 {
		return nil, errors.New("response payload limits must be positive")
	}
	return &ResponseReader{reader: bufio.NewReader(r), hasContext: hasContext, maxPayload: maxPayload, maxTotal: maxTotal, maxChunks: maxChunks}, nil
}

// Next returns the next response chunk or io.EOF after a clean stream end.
func (r *ResponseReader) Next() (*ResponseChunk, error) {
	if r == nil || r.reader == nil {
		return nil, errors.New("response reader is nil")
	}
	if r.chunks == r.maxChunks {
		if _, err := r.reader.Peek(1); errors.Is(err, io.EOF) {
			return nil, io.EOF
		} else if err != nil {
			return nil, fmt.Errorf("check response chunk ceiling: %w", err)
		}
		return nil, fmt.Errorf("response exceeds maximum of %d chunks", r.maxChunks)
	}
	code, err := r.reader.ReadByte()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("read response code: %w", err)
	}
	response := &ResponseChunk{Code: code}
	if code == CodeSuccess && r.hasContext {
		response.Context = make([]byte, ContextBytesLen)
		if _, err := io.ReadFull(r.reader, response.Context); err != nil {
			return nil, fmt.Errorf("read context bytes: %w", err)
		}
	}
	length, err := readConsensusUvarint(r.reader)
	if err != nil {
		return nil, fmt.Errorf("read response length prefix: %w", err)
	}
	if length > r.maxPayload {
		return nil, fmt.Errorf("response length %d exceeds max %d", length, r.maxPayload)
	}
	if length > r.maxTotal-r.total {
		return nil, fmt.Errorf("aggregate response length exceeds max %d", r.maxTotal)
	}
	r.total += length
	r.chunks++
	if length == 0 {
		return response, nil
	}
	if length > uint64(maxInt()) {
		return nil, fmt.Errorf("response length %d cannot fit in memory index", length)
	}
	compressedLimit, err := maxCompressedLen(length)
	if err != nil {
		return nil, err
	}
	compressed := io.LimitReader(r.reader, compressedLimit)
	response.Payload = make([]byte, int(length))
	if _, err := io.ReadFull(snappy.NewReader(compressed), response.Payload); err != nil {
		return nil, fmt.Errorf("read snappy response payload: %w", err)
	}
	return response, nil
}

func readConsensusUvarint(r io.ByteReader) (uint64, error) {
	var value uint64
	var shift uint
	for i := 0; i < 10; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		if b < 0x80 {
			if i == 9 && b > 1 {
				return 0, errors.New("varint overflows uint64")
			}
			return value | uint64(b)<<shift, nil
		}
		value |= uint64(b&0x7f) << shift
		shift += 7
	}
	return 0, errors.New("varint exceeds 10 bytes")
}

func maxCompressedLen(n uint64) (int64, error) {
	if n > math.MaxUint64-32 || n+32 > math.MaxUint64-n/6 {
		return 0, fmt.Errorf("compressed response limit overflows uint64 for %d bytes", n)
	}
	limit := 32 + n + n/6
	if limit > math.MaxInt64 {
		return 0, fmt.Errorf("compressed response limit %d overflows int64", limit)
	}
	return int64(limit), nil
}

func maxInt() int { return int(^uint(0) >> 1) }
