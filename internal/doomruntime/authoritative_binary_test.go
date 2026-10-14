package doomruntime

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand/v2"
	"reflect"
	"strconv"
	"testing"
)

func TestAuthorityBinarySchemaMatchesAllReachableFields(t *testing.T) {
	if got := newAuthorityBinaryGenerator().fingerprint(); got != authorityBinarySchemaFingerprint {
		t.Fatal("authority binary schema changed: regenerate with AUTHORITY_BINARY_GENERATE=1 go test ./internal/doomruntime -run '^TestGenerateAuthorityBinaryCodec$'")
	}
}

func populatedAuthorityBinaryReplica() authorityReplica {
	var v authorityReplica
	serial := 0
	var fill func(reflect.Value)
	fill = func(v reflect.Value) {
		serial++
		switch v.Kind() {
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				fill(v.Field(i))
			}
		case reflect.Pointer:
			v.Set(reflect.New(v.Type().Elem()))
			fill(v.Elem())
		case reflect.Array:
			for i := 0; i < v.Len(); i++ {
				fill(v.Index(i))
			}
		case reflect.Slice:
			v.Set(reflect.MakeSlice(v.Type(), 2, 2))
			for i := 0; i < v.Len(); i++ {
				fill(v.Index(i))
			}
		case reflect.Map:
			v.Set(reflect.MakeMap(v.Type()))
			for _, n := range []int64{17, -3} {
				key := reflect.New(v.Type().Key()).Elem()
				key.SetInt(n)
				value := reflect.New(v.Type().Elem()).Elem()
				fill(value)
				v.SetMapIndex(key, value)
			}
		case reflect.Bool:
			v.SetBool(serial%2 != 0)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			v.SetInt(int64(serial%97 - 48))
		case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			v.SetUint(uint64(serial % 251))
		case reflect.Float32, reflect.Float64:
			v.SetFloat(float64(serial) + .125)
		case reflect.String:
			v.SetString("DÖÖM\x00texture-" + strconv.Itoa(serial%7))
		default:
			panic(v.Type())
		}
	}
	fill(reflect.ValueOf(&v).Elem())
	return v
}

func assertAuthorityBinaryRoundTrip(t *testing.T, original authorityReplica) []byte {
	t.Helper()
	encoded, err := encodeAuthorityReplicaPayload(original)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeAuthorityReplicaPayload(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, got) {
		t.Fatal("binary round trip changed a field, pointer, collection or nil/empty distinction")
	}
	// All decoded slices, maps and strings must own their data after the
	// network envelope is reused, including interned string references.
	owned := bytes.Clone(encoded)
	clear(encoded)
	if !reflect.DeepEqual(original, got) {
		t.Fatal("decoded world aliases the received payload")
	}
	return owned
}

func TestAuthorityBinaryRoundTripEveryFieldAndIntegerBoundary(t *testing.T) {
	v := populatedAuthorityBinaryReplica()
	v.Game.ThingX = []int64{math.MinInt64, math.MaxInt64}
	v.Game.ThingHP = []int{math.MinInt, math.MaxInt}
	v.Game.LineSpecial = []uint16{0, math.MaxUint16}
	v.Game.ThingAngleState = []uint32{0, math.MaxUint32}
	v.SoundCursor = math.MaxUint64
	v.Game.View.CamX = math.Copysign(0, -1)
	encoded := assertAuthorityBinaryRoundTrip(t, v)
	got, err := decodeAuthorityReplicaPayload(encoded)
	if err != nil || !math.Signbit(got.Game.View.CamX) {
		t.Fatal("binary float representation lost negative zero")
	}
	for range 12 {
		again, err := encodeAuthorityReplicaPayload(v)
		if err != nil || !bytes.Equal(again, encoded) {
			t.Fatal("map iteration changed deterministic binary output")
		}
	}
}

func TestAuthorityBinaryNilAndEmptyCollectionsRemainDistinct(t *testing.T) {
	assertAuthorityBinaryRoundTrip(t, authorityReplica{})
	var empty authorityReplica
	var makeEmpty func(reflect.Value)
	makeEmpty = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				makeEmpty(v.Field(i))
			}
		case reflect.Slice:
			v.Set(reflect.MakeSlice(v.Type(), 0, 0))
		case reflect.Map:
			v.Set(reflect.MakeMap(v.Type()))
		case reflect.Pointer:
			v.Set(reflect.New(v.Type().Elem()))
			makeEmpty(v.Elem())
		}
	}
	makeEmpty(reflect.ValueOf(&empty).Elem())
	assertAuthorityBinaryRoundTrip(t, empty)
}

func TestAuthorityBinaryAppendsToOuterFrameWithoutChangingPrefix(t *testing.T) {
	v := populatedAuthorityBinaryReplica()
	want := assertAuthorityBinaryRoundTrip(t, v)
	prefix := []byte("outer authenticated framing")
	dst := make([]byte, len(prefix), len(prefix)+len(want))
	copy(dst, prefix)
	start := &dst[0]
	got, err := appendAuthorityReplicaPayload(dst, v)
	if err != nil || &got[0] != start || !bytes.Equal(got[:len(prefix)], prefix) || !bytes.Equal(got[len(prefix):], want) {
		t.Fatal("append encoder changed framing, output or unnecessarily replaced its buffer")
	}
}

func TestAuthorityBinaryRejectsTruncationTrailingAndWrongSchema(t *testing.T) {
	encoded := assertAuthorityBinaryRoundTrip(t, populatedAuthorityBinaryReplica())
	for n := 0; n < len(encoded); n++ {
		if _, err := decodeAuthorityReplicaPayload(encoded[:n]); err == nil {
			t.Fatalf("accepted payload truncated at %d/%d", n, len(encoded))
		}
	}
	for _, offset := range []int{0, 3, 4, authorityBinaryHeaderBytes - 1} {
		bad := bytes.Clone(encoded)
		bad[offset] ^= 1
		if _, err := decodeAuthorityReplicaPayload(bad); err == nil {
			t.Fatalf("accepted incompatible binary header byte %d", offset)
		}
	}
	if _, err := decodeAuthorityReplicaPayload(append(encoded, 0)); err == nil {
		t.Fatal("accepted trailing binary data")
	}
}

func TestAuthorityBinaryRejectsInvalidPrimitiveRepresentations(t *testing.T) {
	for _, test := range []struct {
		name string
		data []byte
		read func(*authorityBinaryReader)
	}{
		{"boolean", []byte{2}, func(r *authorityBinaryReader) { r.boolean() }},
		{"overlong integer", []byte{0x80, 0}, func(r *authorityBinaryReader) { r.unsigned(64) }},
		{"truncated integer", []byte{0x80}, func(r *authorityBinaryReader) { r.unsigned(64) }},
		{"overflow integer", []byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 2}, func(r *authorityBinaryReader) { r.unsigned(64) }},
		{"uint32 overflow", binary.AppendUvarint(nil, 1<<32), func(r *authorityBinaryReader) { r.unsigned(32) }},
		{"int16 overflow", binary.AppendUvarint(nil, 65536), func(r *authorityBinaryReader) { r.signed(16) }},
		{"int32 overflow", binary.AppendUvarint(nil, 1<<32), func(r *authorityBinaryReader) { r.signed(32) }},
		{"invalid UTF-8", []byte{3, 255}, func(r *authorityBinaryReader) { r.string() }},
		{"unknown string ID", []byte{2}, func(r *authorityBinaryReader) { r.string() }},
		{"empty literal definition", []byte{1}, func(r *authorityBinaryReader) { r.string() }},
		{"non-finite float", binary.LittleEndian.AppendUint64(nil, math.Float64bits(math.Inf(1))), func(r *authorityBinaryReader) { r.float64() }},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := authorityBinaryReader{data: test.data, allocationLeft: authorityBinaryAllocationCap}
			test.read(&r)
			if r.err == nil {
				t.Fatal("accepted invalid primitive")
			}
		})
	}
	for _, bad := range []float64{math.Inf(-1), math.Inf(1), math.NaN()} {
		v := authorityReplica{}
		v.Game.View.CamX = bad
		if _, err := encodeAuthorityReplicaPayload(v); err == nil {
			t.Fatal("encoded non-finite view state")
		}
	}
	v := authorityReplica{WADHash: string([]byte{255})}
	if _, err := encodeAuthorityReplicaPayload(v); err == nil {
		t.Fatal("encoded invalid UTF-8")
	}
}

func TestAuthorityBinaryStringTableOwnsAndReusesRepeatedNames(t *testing.T) {
	w := authorityBinaryWriter{}
	for _, value := range []string{"FLOOR4_8", "", "FLOOR4_8", "STONE2", "FLOOR4_8"} {
		w.string(value)
	}
	if w.err != nil || len(w.strings) != 2 {
		t.Fatal("writer did not deduplicate repeated textures")
	}
	r := authorityBinaryReader{data: w.data, allocationLeft: authorityBinaryAllocationCap}
	for _, want := range []string{"FLOOR4_8", "", "FLOOR4_8", "STONE2", "FLOOR4_8"} {
		if got := r.string(); r.err != nil || got != want {
			t.Fatalf("interned string=%q err=%v want=%q", got, r.err, want)
		}
	}
	if len(r.strings) != 2 || len(r.data) != 0 {
		t.Fatal("reader did not share owned definitions")
	}
	clear(w.data)
	if r.strings[0] != "FLOOR4_8" {
		t.Fatal("string definition aliases network input")
	}
	duplicate := authorityBinaryReader{data: []byte{3, 'a', 3, 'a'}, allocationLeft: authorityBinaryAllocationCap}
	duplicate.string()
	duplicate.string()
	if duplicate.err == nil {
		t.Fatal("accepted a duplicate literal definition instead of a string reference")
	}
}

func TestAuthorityBinaryBoundsCollectionsAndDecodedMemory(t *testing.T) {
	for _, test := range []struct {
		data   []byte
		budget int
	}{
		{binary.AppendUvarint(nil, authorityBinaryMaxElements+2), authorityBinaryAllocationCap},
		{[]byte{101}, authorityBinaryAllocationCap}, // 100 elements, no payload.
		{[]byte{4, 0, 0, 0}, 8}, // 3 int64s exceed allocation budget.
	} {
		r := authorityBinaryReader{data: test.data, allocationLeft: test.budget}
		if _, _ = r.collection(1, 8); r.err == nil {
			t.Fatal("accepted an impossible collection or excessive decoded allocation")
		}
	}
	v := authorityReplica{ThingTelefragTick: make([]int, authorityBinaryMaxElements+1)}
	if _, err := encodeAuthorityReplicaPayload(v); err == nil {
		t.Fatal("encoded an oversized collection")
	}
}

func TestAuthorityBinaryWriterAndReaderShareAllocationBudget(t *testing.T) {
	original := populatedAuthorityBinaryReplica()
	w := authorityBinaryWriter{}
	w.writedoomruntime_authorityReplica(&original)
	if w.err != nil {
		t.Fatal(w.err)
	}
	r := authorityBinaryReader{data: w.data, allocationLeft: authorityBinaryAllocationCap}
	var decoded authorityReplica
	r.readdoomruntime_authorityReplica(&decoded)
	if r.err != nil || w.allocationUsed != authorityBinaryAllocationCap-r.allocationLeft {
		t.Fatalf("asymmetric decoded allocation bound: writer=%d reader=%d error=%v", w.allocationUsed, authorityBinaryAllocationCap-r.allocationLeft, r.err)
	}
	w.allocationUsed = authorityBinaryAllocationCap - 4
	if w.collection(1, false, 8) || w.err == nil {
		t.Fatal("writer accepted an allocation the receiving decoder would reject")
	}
}

func TestAuthorityBinaryRejectsUnorderedAndDuplicateMapKeys(t *testing.T) {
	// playerInventorySaveState has seventeen scalar fields followed by its
	// weapon map. Its generated reader is the same one nested in real replicas.
	for _, keys := range [][2]int64{{7, 7}, {7, 3}} {
		w := authorityBinaryWriter{}
		v := playerInventorySaveState{}
		w.writedoomruntime_playerInventorySaveState(&v)
		w.data = w.data[:len(w.data)-1] // Replace the final nil-map tag.
		w.collection(2, false, 2*(authorityBinarySizeOf[int16]()+authorityBinarySizeOf[bool]()+16))
		for _, key := range keys {
			w.signed(key)
			w.boolean(true)
		}
		r := authorityBinaryReader{data: w.data, allocationLeft: authorityBinaryAllocationCap}
		r.readdoomruntime_playerInventorySaveState(&v)
		if r.err == nil {
			t.Fatalf("accepted invalid map ordering %v", keys)
		}
	}
}

func TestAuthorityBinaryGeneratedThingShapeChecksEveryParallelArray(t *testing.T) {
	typ := reflect.TypeOf(gameSaveState{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if len(field.Name) < 5 || field.Name[:5] != "Thing" || field.Type.Kind() != reflect.Slice {
			continue
		}
		var s gameSaveState
		reflect.ValueOf(&s).Elem().Field(i).Set(reflect.MakeSlice(field.Type, 2, 2))
		if err := validateAuthorityThingShapes(&s, 1); err == nil {
			t.Fatalf("generated shape check omitted %s", field.Name)
		}
		if err := validateAuthorityThingShapes(&s, 2); err != nil {
			t.Fatalf("generated shape check rejected valid %s: %v", field.Name, err)
		}
	}
}

func TestAuthorityBinaryGeneratedInventoryComparisonMatchesEveryField(t *testing.T) {
	original := populatedAuthorityBinaryReplica().Game.Inventory
	if !equalAuthorityInventory(original, original) {
		t.Fatal("equal inventory rejected")
	}
	typ := reflect.TypeOf(original)
	for i := 0; i < typ.NumField(); i++ {
		changed := original
		value := reflect.ValueOf(&changed).Elem().Field(i)
		switch value.Kind() {
		case reflect.Bool:
			value.SetBool(!value.Bool())
		case reflect.Int:
			value.SetInt(value.Int() + 1)
		case reflect.Map:
			value.Set(reflect.MakeMap(value.Type()))
		default:
			t.Fatalf("add comparison test for inventory field %s", typ.Field(i).Name)
		}
		if equalAuthorityInventory(original, changed) {
			t.Fatalf("inventory comparison omitted %s", typ.Field(i).Name)
		}
	}
	empty, nilMap := playerInventorySaveState{Weapons: map[int16]bool{}}, playerInventorySaveState{}
	if equalAuthorityInventory(empty, nilMap) {
		t.Fatal("inventory equality lost nil-map distinction")
	}
}

func TestAuthorityBinaryMutatedPayloadsRemainCanonical(t *testing.T) {
	seed := assertAuthorityBinaryRoundTrip(t, populatedAuthorityBinaryReplica())
	random := rand.New(rand.NewPCG(14, 57))
	for i := 0; i < 500; i++ {
		candidate := bytes.Clone(seed)
		for j := 0; j < 1+i%3; j++ {
			index := authorityBinaryHeaderBytes + random.IntN(len(candidate)-authorityBinaryHeaderBytes)
			candidate[index] ^= 1 << random.IntN(8)
		}
		decoded, err := decodeAuthorityReplicaPayload(candidate)
		if err != nil {
			continue
		}
		again, err := encodeAuthorityReplicaPayload(decoded)
		if err != nil || !bytes.Equal(again, candidate) {
			t.Fatalf("accepted mutation %d is not a canonical representation: %v", i, err)
		}
	}
}

func FuzzAuthorityBinaryPayload(f *testing.F) {
	for _, value := range []authorityReplica{{}, populatedAuthorityBinaryReplica()} {
		encoded, err := encodeAuthorityReplicaPayload(value)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(encoded)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		decoded, err := decodeAuthorityReplicaPayload(data)
		if err != nil {
			return
		}
		again, err := encodeAuthorityReplicaPayload(decoded)
		if err != nil || !bytes.Equal(again, data) {
			t.Fatal("accepted binary payload is not a canonical round trip")
		}
	})
}
