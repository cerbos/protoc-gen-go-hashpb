// Copyright 2021-2022 Zenauth Ltd.
// SPDX-License-Identifier: Apache-2.0

package hashpb_test

import (
	"crypto/sha256"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/cerbos/hashpb"
	"github.com/cerbos/hashpb/internal"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var sink byte

func TestSum(t *testing.T) {
	testCases := []struct {
		name    string
		input   proto.Message
		wantErr bool
	}{
		{
			name:    "nil",
			wantErr: true,
		},
		{
			name:  "empty",
			input: &internal.TestAllTypes{},
		},
		{
			name:  "fully populated",
			input: mkTestAllTypesMsg(),
		},
		{
			name:  "fully populated nested",
			input: mkNestedTestAllTypesMsg(3),
		},
	}

	options := []struct {
		name string
		opts []hashpb.Option
	}{
		{name: "default"},
		{name: "sha256", opts: []hashpb.Option{hashpb.WithHasher(sha256.New())}},
	}

	for _, o := range options {
		o := o
		t.Run(o.name, func(t *testing.T) {
			for _, tc := range testCases {
				tc := tc
				t.Run(tc.name, func(t *testing.T) {
					have, err := hashpb.Sum(nil, tc.input, o.opts...)
					if tc.wantErr {
						if err == nil {
							t.Fatal("Required error but was nil")
						}
						return
					}

					if err != nil {
						t.Fatalf("Unexpected error: %v", err)
					}

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
		name string
		opt  hashpb.Option
	}{
		{
			name: "oneOfField",
			opt:  hashpb.WithIgnoreFields("cerbos.hashpb.test.TestAllTypes.nested_type"),
		},
		{
			name: "individualFields",
			opt: hashpb.WithIgnoreFields(
				"cerbos.hashpb.test.TestAllTypes.single_nested_message",
				"cerbos.hashpb.test.TestAllTypes.single_nested_enum",
			),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			m1 := mkTestAllTypesMsg()
			m2 := mkTestAllTypesMsg()

			h1, _ := hashpb.Sum64(m1)
			h2, _ := hashpb.Sum64(m2)

			if h1 != h2 {
				t.Fatalf("Expected h1 == h2. Was h1=%d h2=%d", h1, h2)
			}

			m3 := mkTestAllTypesMsg()
			m3.NestedType = &internal.TestAllTypes_SingleNestedEnum{SingleNestedEnum: internal.TestAllTypes_BAZ}

			h3, _ := hashpb.Sum64(m3)

			if h1 == h3 {
				t.Fatalf("Expected h1 != h3. Was h1=%d h3=%d", h1, h2)
			}

			h4, _ := hashpb.Sum64(m1, tc.opt)
			h5, _ := hashpb.Sum64(m3, tc.opt)
			if h4 != h5 {
				t.Fatalf("Expected h4 == h5. Was h4=%d h5=%d", h4, h5)
			}
		})
	}
}

func mkNestedTestAllTypesMsg(nesting int) *internal.NestedTestAllTypes {
	m := &internal.NestedTestAllTypes{
		Payload: mkTestAllTypesMsg(),
	}

	if nesting <= 1 {
		return m
	}

	m.Child = mkNestedTestAllTypesMsg(nesting - 1)
	return m
}

func mkTestAllTypesMsg() *internal.TestAllTypes {
	return &internal.TestAllTypes{
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
		StandaloneEnum:        internal.TestAllTypes_BAZ,
		SingleDuration:        durationpb.New(10 * time.Minute),
		SingleTimestamp:       timestamppb.New(time.Unix(1642694886, 0)),
		SingleInt64Wrapper:    wrapperspb.Int64(42),
		SingleStringWrapper:   wrapperspb.String("wibble wobble"),
		NestedType:            &internal.TestAllTypes_SingleNestedMessage{SingleNestedMessage: &internal.TestAllTypes_NestedMessage{Bb: 42}},
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
		RepeatedNestedMessage: []*internal.TestAllTypes_NestedMessage{{Bb: 1}, {Bb: 2}, {Bb: 3}},
		RepeatedNestedEnum:    []internal.TestAllTypes_NestedEnum{internal.TestAllTypes_BAR, internal.TestAllTypes_BAZ},
		MapStringString:       map[string]string{"a": "b", "c": "d", "e": "f"},
		MapUint64String:       map[uint64]string{1: "a", 2: "b", 3: "c"},
		MapInt32String:        map[int32]string{1: "a", 2: "b", 3: "c"},
		MapBoolString:         map[bool]string{true: "a", false: "b"},
		MapInt64NestedType:    map[int64]*internal.TestAllTypes_NestedMessage{1: {Bb: 1}},
	}
}

func BenchmarkSum64(b *testing.B) {
	for _, n := range []int{1, 5, 10, 50, 100} {
		b.Run(fmt.Sprintf("nesting=%d", n), func(b *testing.B) {
			buf := make([]byte, 8)
			m := mkNestedTestAllTypesMsg(n)
			size := proto.Size(m)
			b.SetBytes(int64(size))
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				sum, err := hashpb.Sum(buf, m)
				if err != nil {
					b.Errorf("Failed to calculate sum: %v", err)
				}
				sink = sum[rand.Intn(8)]
			}
		})
	}
}
