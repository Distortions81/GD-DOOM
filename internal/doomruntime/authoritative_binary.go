package doomruntime

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"unicode/utf8"
	"unsafe"
)

// The outer snapshot framing/checksum remains the caller's responsibility. The
// payload identifies both this binary grammar and the exact generated schema.
// Go ints use signed 64-bit zigzag on the wire, with checked conversion to the
// receiving platform's int width; they never depend on host memory layout.
var authorityBinaryMagic = [4]byte{'A', 'B', 'I', 1}

const (
	authorityBinaryHeaderBytes   = 4 + 16
	authorityBinaryMaxElements   = 65536
	authorityBinaryAllocationCap = 32 << 20
)

func encodeAuthorityReplicaPayload(v authorityReplica) ([]byte, error) {
	return appendAuthorityReplicaPayload(make([]byte, 0, 32<<10), v)
}

// appendAuthorityReplicaPayload permits the outer authenticated framing to
// share its output buffer, avoiding a complete second payload copy.
func appendAuthorityReplicaPayload(dst []byte, v authorityReplica) ([]byte, error) {
	w := authorityBinaryWriter{data: dst, start: len(dst)}
	w.raw(authorityBinaryMagic[:])
	w.raw(authorityBinarySchemaFingerprint[:])
	w.writedoomruntime_authorityReplica(&v)
	if w.err != nil {
		return nil, w.err
	}
	return w.data, nil
}

func decodeAuthorityReplicaPayload(data []byte) (authorityReplica, error) {
	var v authorityReplica
	if len(data) < authorityBinaryHeaderBytes || len(data) > MaxAuthoritySnapshotBytes ||
		!bytes.Equal(data[:4], authorityBinaryMagic[:]) ||
		!bytes.Equal(data[4:authorityBinaryHeaderBytes], authorityBinarySchemaFingerprint[:]) {
		return v, fmt.Errorf("authority binary payload version, schema or size mismatch")
	}
	r := authorityBinaryReader{data: data[authorityBinaryHeaderBytes:], allocationLeft: authorityBinaryAllocationCap}
	r.readdoomruntime_authorityReplica(&v)
	if r.err != nil {
		return authorityReplica{}, r.err
	}
	if len(r.data) != 0 {
		return authorityReplica{}, fmt.Errorf("authority binary payload has trailing data")
	}
	return v, nil
}

type authorityBinaryWriter struct {
	data           []byte
	start          int
	err            error
	strings        map[string]uint64
	allocationUsed int
}

func (w *authorityBinaryWriter) space(n int) bool {
	if w.err != nil {
		return false
	}
	if n < 0 || n > MaxAuthoritySnapshotBytes-(len(w.data)-w.start) {
		w.err = fmt.Errorf("authority binary payload exceeds size limit")
		return false
	}
	return true
}

func (w *authorityBinaryWriter) raw(v []byte) {
	if w.space(len(v)) {
		w.data = append(w.data, v...)
	}
}

func (w *authorityBinaryWriter) byte(v byte) {
	if w.space(1) {
		w.data = append(w.data, v)
	}
}

func (w *authorityBinaryWriter) boolean(v bool) {
	if v {
		w.byte(1)
	} else {
		w.byte(0)
	}
}

func (w *authorityBinaryWriter) unsigned(v uint64) {
	if !w.space(1) {
		return
	}
	// Common game counters and zero-filled arrays fit in one byte.
	if v < 128 {
		w.data = append(w.data, byte(v))
		return
	}
	var encoded [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(encoded[:], v)
	w.raw(encoded[:n])
}

func (w *authorityBinaryWriter) signed(v int64) {
	w.unsigned(uint64(v<<1) ^ uint64(v>>63))
}

func (w *authorityBinaryWriter) float64(v float64) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		w.err = fmt.Errorf("authority binary payload has non-finite float")
		return
	}
	if w.space(8) {
		w.data = binary.LittleEndian.AppendUint64(w.data, math.Float64bits(v))
	}
}

func (w *authorityBinaryWriter) string(v string) {
	if w.err != nil {
		return
	}
	if v == "" {
		w.unsigned(0)
		return
	}
	if id := w.strings[v]; id != 0 {
		w.unsigned(id << 1)
		return
	}
	if !utf8.ValidString(v) {
		w.err = fmt.Errorf("authority binary payload has invalid UTF-8")
		return
	}
	if len(w.strings) >= authorityBinaryMaxElements {
		w.err = fmt.Errorf("authority binary string table exceeds element limit")
		return
	}
	if !w.space(len(v)) {
		return
	}
	if !w.allocate(len(v), 1) || !w.allocate(1, 128) {
		return
	}
	if w.strings == nil {
		w.strings = make(map[string]uint64)
	}
	w.strings[v] = uint64(len(w.strings)) + 1
	w.unsigned(uint64(len(v))<<1 | 1)
	if w.space(len(v)) {
		w.data = append(w.data, v...)
	}
}

func (w *authorityBinaryWriter) allocate(n, elementSize int) bool {
	if w.err != nil {
		return false
	}
	if n < 0 || elementSize <= 0 || n > (authorityBinaryAllocationCap-w.allocationUsed)/elementSize {
		w.err = fmt.Errorf("authority binary decoded allocation budget exceeded")
		return false
	}
	w.allocationUsed += n * elementSize
	return true
}

func (w *authorityBinaryWriter) collection(n int, nilValue bool, elementSize int) bool {
	if w.err != nil {
		return false
	}
	if n < 0 || n > authorityBinaryMaxElements {
		w.err = fmt.Errorf("authority binary collection exceeds element limit")
		return false
	}
	if !w.allocate(n, elementSize) {
		return false
	}
	if nilValue {
		w.unsigned(0)
	} else {
		w.unsigned(uint64(n) + 1)
	}
	return w.err == nil
}

type authorityBinaryReader struct {
	data           []byte
	err            error
	allocationLeft int
	strings        []string
	stringSet      map[string]struct{}
}

func (r *authorityBinaryReader) fail(message string) {
	if r.err == nil {
		r.err = fmt.Errorf("authority binary payload: %s", message)
	}
}

func (r *authorityBinaryReader) raw(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || n > len(r.data) {
		r.fail("truncated field")
		return nil
	}
	v := r.data[:n]
	r.data = r.data[n:]
	return v
}

func (r *authorityBinaryReader) byte() byte {
	if r.err != nil {
		return 0
	}
	if len(r.data) == 0 {
		r.fail("truncated byte")
		return 0
	}
	v := r.data[0]
	r.data = r.data[1:]
	return v
}

func (r *authorityBinaryReader) boolean() bool {
	v := r.byte()
	if v > 1 {
		r.fail("invalid boolean")
	}
	return v == 1
}

func (r *authorityBinaryReader) unsigned(bits int) uint64 {
	if r.err != nil {
		return 0
	}
	if len(r.data) == 0 {
		r.fail("truncated integer")
		return 0
	}
	v, n := uint64(r.data[0]), 1
	if v >= 128 {
		v, n = binary.Uvarint(r.data)
		if n <= 0 {
			r.fail("truncated or overflowing integer")
			return 0
		}
		// Leading zero groups have multiple encodings; reject them so payloads
		// have a unique canonical representation.
		if r.data[n-1] == 0 {
			r.fail("non-canonical integer")
			return 0
		}
	}
	r.data = r.data[n:]
	if bits < 64 && v >= uint64(1)<<bits {
		r.fail("integer overflows field width")
		return 0
	}
	return v
}

func (r *authorityBinaryReader) signed(bits int) int64 {
	u := r.unsigned(64)
	v := int64(u>>1) ^ -int64(u&1)
	if bits < 64 && (v < -(int64(1)<<(bits-1)) || v > (int64(1)<<(bits-1))-1) {
		r.fail("signed integer overflows field width")
		return 0
	}
	return v
}

func (r *authorityBinaryReader) float64() float64 {
	b := r.raw(8)
	if len(b) != 8 {
		return 0
	}
	v := math.Float64frombits(binary.LittleEndian.Uint64(b))
	if math.IsNaN(v) || math.IsInf(v, 0) {
		r.fail("non-finite float")
	}
	return v
}

func (r *authorityBinaryReader) string() string {
	tag := r.unsigned(64)
	if r.err != nil || tag == 0 {
		return ""
	}
	if tag&1 == 0 {
		id := tag >> 1
		if id > uint64(len(r.strings)) {
			r.fail("invalid string reference")
			return ""
		}
		return r.strings[id-1]
	}
	n := tag >> 1
	// Account conservatively for the owned bytes, table capacity and hash-set
	// entry before allocating either. Repeated textures then cost no allocation.
	if n == 0 || n > uint64(len(r.data)) || len(r.strings) >= authorityBinaryMaxElements ||
		!r.allocate(int(n), 1) || !r.allocate(1, 128) {
		r.fail("invalid string length")
		return ""
	}
	b := r.raw(int(n))
	if !utf8.Valid(b) {
		r.fail("invalid UTF-8")
		return ""
	}
	v := string(b)
	if _, exists := r.stringSet[v]; exists {
		r.fail("duplicate string definition")
		return ""
	}
	if r.stringSet == nil {
		r.stringSet = make(map[string]struct{})
	}
	r.stringSet[v] = struct{}{}
	r.strings = append(r.strings, v)
	return v
}

func authorityBinarySizeOf[T any]() int {
	var value T
	return int(unsafe.Sizeof(value))
}

func (r *authorityBinaryReader) allocate(n, elementSize int) bool {
	if r.err != nil {
		return false
	}
	if n < 0 || elementSize <= 0 || n > r.allocationLeft/elementSize {
		r.fail("allocation budget exceeded")
		return false
	}
	r.allocationLeft -= n * elementSize
	return true
}

func (r *authorityBinaryReader) collection(minimumBytes, elementSize int) (int, bool) {
	tag := r.unsigned(64)
	if r.err != nil || tag == 0 {
		return 0, false
	}
	n := tag - 1
	if n > authorityBinaryMaxElements || minimumBytes <= 0 || n > uint64(len(r.data)/minimumBytes) {
		r.fail("invalid collection length")
		return 0, false
	}
	if !r.allocate(int(n), elementSize) {
		return 0, false
	}
	return int(n), true
}
