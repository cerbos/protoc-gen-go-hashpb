// Copyright 2021-2022 Zenauth Ltd.
// SPDX-License-Identifier: Apache-2.0

package generator_test

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"math/rand"
	"testing"
	"time"

	"github.com/cerbos/protoc-gen-go-hashpb/internal/pb"
	"github.com/cespare/xxhash/v2"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var sink byte

type Hashable interface {
	HashPB(hash.Hash, map[string]struct{})
}

func TestHashPB(t *testing.T) {
	testCases := []struct {
		name  string
		input Hashable
	}{
		{
			name:  "nil",
			input: (*pb.TestAllTypes)(nil),
		},
		{
			name:  "empty",
			input: &pb.TestAllTypes{},
		},
		{
			name:  "fully populated",
			input: mkTestAllTypesMsg(),
		},
		{
			name:  "fully populated nested",
			input: mkNestedTestAllTypesMsg(3),
		},
		{
			name:  "fully populated optional",
			input: mkTestAllTypesOptionalMsg(),
		},
		{
			name:  "fully populated optional and empty",
			input: mkTestAllTypesOptionalMsgEmpty(),
		},
	}

	options := []struct {
		name   string
		hashFn func() hash.Hash
	}{
		{name: "xxhash", hashFn: func() hash.Hash { return xxhash.New() }},
		{name: "sha256", hashFn: sha256.New},
	}

	for _, o := range options {
		o := o
		t.Run(o.name, func(t *testing.T) {
			for _, tc := range testCases {
				tc := tc
				t.Run(tc.name, func(t *testing.T) {
					h := o.hashFn()
					tc.input.HashPB(h, nil)
					have := h.Sum(nil)

					if len(have) == 0 {
						t.Fatal("Empty hash")
					}
				})
			}
		})

	}
}

func TestIgnore(t *testing.T) {
	testCases := []struct {
		name   string
		input  func() Hashable
		ignore map[string]struct{}
	}{
		{
			name: "oneOfField",
			input: func() Hashable {
				m := mkTestAllTypesMsg()
				m.NestedType = &pb.TestAllTypes_SingleNestedEnum{
					SingleNestedEnum: pb.TestAllTypes_BAZ,
				}
				return m
			},
			ignore: map[string]struct{}{
				"cerbos.hashpb.test.TestAllTypes.nested_type": {},
			},
		},
		{
			name: "individualFields",
			input: func() Hashable {
				m := mkTestAllTypesMsg()
				m.SingleTimestamp = timestamppb.Now()
				m.MapBoolString = map[bool]string{false: "foo"}
				return m
			},
			ignore: map[string]struct{}{
				"cerbos.hashpb.test.TestAllTypes.single_timestamp": {},
				"cerbos.hashpb.test.TestAllTypes.map_bool_string":  {},
			},
		},
	}

	m1 := mkTestAllTypesMsg()
	m2 := mkTestAllTypesMsg()

	h1 := sum64(m1, nil)
	h2 := sum64(m2, nil)

	if h1 != h2 {
		t.Fatalf("Expected h1 == h2. Was h1=%d h2=%d", h1, h2)
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			input := tc.input()
			h3 := sum64(input, nil)

			if h1 == h3 {
				t.Fatalf("Expected h1 != h3. Was h1=%d h3=%d", h1, h2)
			}

			h4 := sum64(m1, tc.ignore)
			h5 := sum64(input, tc.ignore)
			if h4 != h5 {
				t.Fatalf("Expected h4 == h5. Was h4=%d h5=%d", h4, h5)
			}
		})
	}
}

func sum64(m Hashable, ignore map[string]struct{}) uint64 {
	h := xxhash.New()
	m.HashPB(h, ignore)
	return h.Sum64()
}

func mkNestedTestAllTypesMsg(nesting int) *pb.NestedTestAllTypes {
	m := &pb.NestedTestAllTypes{
		Payload: mkTestAllTypesMsg(),
	}

	if nesting <= 1 {
		return m
	}

	m.Child = mkNestedTestAllTypesMsg(nesting - 1)
	return m
}

func mkTestAllTypesMsg() *pb.TestAllTypes {
	return &pb.TestAllTypes{
		SingleInt32:           42,
		SingleInt64:           42,
		SingleUint32:          42,
		SingleUint64:          42,
		SingleSint32:          42,
		SingleSint64:          42,
		SingleFixed32:         42,
		SingleFixed64:         42,
		SingleSfixed32:        42,
		SingleSfixed64:        42,
		SingleFloat:           42.42,
		SingleDouble:          42.42,
		SingleBool:            true,
		SingleString:          "wibble wobble",
		SingleBytes:           []byte("wibble wobble"),
		StandaloneEnum:        pb.TestAllTypes_BAZ,
		SingleDuration:        durationpb.New(10 * time.Minute),
		SingleTimestamp:       timestamppb.New(time.Unix(1642694886, 0)),
		SingleInt64Wrapper:    wrapperspb.Int64(42),
		SingleStringWrapper:   wrapperspb.String("wibble wobble"),
		NestedType:            &pb.TestAllTypes_SingleNestedMessage{SingleNestedMessage: &pb.TestAllTypes_NestedMessage{Bb: 42}},
		RepeatedInt32:         []int32{1, 2, 3},
		RepeatedInt64:         []int64{1, 2, 3},
		RepeatedUint32:        []uint32{1, 2, 3},
		RepeatedUint64:        []uint64{1, 2, 3},
		RepeatedSint32:        []int32{1, 2, 3},
		RepeatedSint64:        []int64{1, 2, 3},
		RepeatedFixed32:       []uint32{1, 2, 3},
		RepeatedFixed64:       []uint64{1, 2, 3},
		RepeatedSfixed32:      []int32{1, 2, 3},
		RepeatedSfixed64:      []int64{1, 2, 3},
		RepeatedFloat:         []float32{1.2, 2.3, 3.4},
		RepeatedDouble:        []float64{1.2, 2.3, 3.4},
		RepeatedBool:          []bool{true, false, true},
		RepeatedString:        []string{"wibble", "wobble", "flub"},
		RepeatedBytes:         [][]byte{[]byte("wibble"), []byte("wobble"), []byte("flub")},
		RepeatedNestedMessage: []*pb.TestAllTypes_NestedMessage{{Bb: 1}, {Bb: 2}, {Bb: 3}},
		RepeatedNestedEnum:    []pb.TestAllTypes_NestedEnum{pb.TestAllTypes_BAR, pb.TestAllTypes_BAZ},
		MapStringString:       map[string]string{"a": "b", "c": "d", "e": "f"},
		MapUint64String:       map[uint64]string{1: "a", 2: "b", 3: "c"},
		MapInt32String:        map[int32]string{1: "a", 2: "b", 3: "c"},
		MapBoolString:         map[bool]string{true: "a", false: "b"},
		MapInt64NestedType:    map[int64]*pb.TestAllTypes_NestedMessage{1: {Bb: 1}},
	}
}

func mkTestAllTypesOptionalMsg() *pb.TestAllTypesOptional {
	return &pb.TestAllTypesOptional{
		SingleInt32:         proto.Int32(42),
		SingleInt64:         proto.Int64(42),
		SingleUint32:        proto.Uint32(42),
		SingleUint64:        proto.Uint64(42),
		SingleSint32:        proto.Int32(42),
		SingleSint64:        proto.Int64(42),
		SingleFixed32:       proto.Uint32(42),
		SingleFixed64:       proto.Uint64(42),
		SingleSfixed32:      proto.Int32(42),
		SingleSfixed64:      proto.Int64(42),
		SingleFloat:         proto.Float32(42.42),
		SingleDouble:        proto.Float64(42.42),
		SingleBool:          proto.Bool(true),
		SingleString:        proto.String("wibble wobble"),
		SingleBytes:         []byte("wibble wobble"),
		StandaloneEnum:      pb.TestAllTypesOptional_BAR.Enum(),
		SingleDuration:      durationpb.New(10 * time.Minute),
		SingleTimestamp:     timestamppb.New(time.Unix(1642694886, 0)),
		SingleInt64Wrapper:  wrapperspb.Int64(42),
		SingleStringWrapper: wrapperspb.String("wibble wobble"),
		SingleNestedMessage: &pb.TestAllTypesOptional_NestedMessage{Bb: proto.Int32(42)},
	}
}

func mkTestAllTypesOptionalMsgEmpty() *pb.TestAllTypesOptional {
	return &pb.TestAllTypesOptional{}
}

func BenchmarkHashPB(b *testing.B) {
	for _, n := range []int{1, 5, 10, 50, 100} {
		b.Run(fmt.Sprintf("nesting=%d", n), func(b *testing.B) {
			buf := make([]byte, 8)
			m := mkNestedTestAllTypesMsg(n)
			size := proto.Size(m)
			b.SetBytes(int64(size))
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				digest := xxhash.New()
				m.HashPB(digest, nil)
				sum := digest.Sum(buf)
				sink = sum[rand.Intn(8)]
			}
		})
	}
}
