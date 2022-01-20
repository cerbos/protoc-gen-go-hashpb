// Copyright 2021-2022 Zenauth Ltd.
// SPDX-License-Identifier: Apache-2.0

package hashpb

import (
	"errors"
	"fmt"
	"hash"
	"math"
	"sort"

	"github.com/cespare/xxhash/v2"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var (
	errInvalidMsg  = errors.New("invalid message")
	errUnsupported = errors.New("operation not supported by underlying hash function")
)

type options struct {
	hasher       hash.Hash
	ignoreFields map[string]struct{}
}

func (o options) shouldIgnore(fd protoreflect.FieldDescriptor) bool {
	if od := fd.ContainingOneof(); od != nil {
		if _, ignore := o.ignoreFields[string(od.FullName())]; ignore {
			return true
		}
	}

	_, ignore := o.ignoreFields[string(fd.FullName())]
	return ignore
}

// Option represents the hashing options passed to the hasher.
type Option func(*options)

// WithHasher sets the hash function to use. Defaults to xxhash.
func WithHasher(h hash.Hash) Option {
	return func(o *options) {
		o.hasher = h
	}
}

// WithIgnoreFields sets the fields that should be ignored when calculating the hash.
// Field name must contain the fully qualified name of the message in which it is defined.
// E.g. my.package.MyMessage.my_field_name
func WithIgnoreFields(fieldNames ...string) Option {
	return func(o *options) {
		if o.ignoreFields == nil {
			o.ignoreFields = make(map[string]struct{})
		}

		for _, f := range fieldNames {
			o.ignoreFields[f] = struct{}{}
		}
	}
}

// Sum64 calculates the 64-bit hash of the given protobuf message.
func Sum64(pb proto.Message, optsList ...Option) (uint64, error) {
	ph, err := doHash(pb, optsList...)
	if err != nil {
		return 0, err
	}

	h64, ok := ph.hasher.(hash.Hash64)
	if !ok {
		return 0, errUnsupported
	}

	return h64.Sum64(), nil
}

// Sum calculates the hash of the protobuf message and appends it to the provided byte array.
func Sum(b []byte, pb proto.Message, optsList ...Option) ([]byte, error) {
	ph, err := doHash(pb, optsList...)
	if err != nil {
		return nil, err
	}

	return ph.hasher.Sum(b), nil
}

func doHash(pb proto.Message, optsList ...Option) (*pbHasher, error) {
	if pb == nil {
		return nil, errInvalidMsg
	}

	msg := pb.ProtoReflect()
	if msg == nil {
		return nil, errInvalidMsg
	}

	ph := &pbHasher{}
	for _, o := range optsList {
		o(&ph.options)
	}

	if ph.hasher == nil {
		ph.hasher = xxhash.New()
	}

	if err := ph.hash(msg); err != nil {
		return nil, err
	}

	return ph, nil
}

type pbHasher struct {
	options
}

func (ph *pbHasher) hash(msg protoreflect.Message) error {
	md := msg.Descriptor()
	if md == nil {
		return errInvalidMsg
	}

	fields := md.Fields()
	numFields := fields.Len()
	setFields := make([]protoreflect.FieldNumber, 0, numFields)
	setDescriptors := make(map[protoreflect.FieldNumber]protoreflect.FieldDescriptor, numFields)

	for i := 0; i < numFields; i++ {
		fd := fields.Get(i)
		if ph.shouldIgnore(fd) {
			continue
		}

		if msg.Has(fd) {
			n := fd.Number()
			setFields = append(setFields, n)
			setDescriptors[n] = fd
		}
	}

	sort.Slice(setFields, func(i, j int) bool { return setFields[i] < setFields[j] })
	for _, fn := range setFields {
		fd := setDescriptors[fn]
		if err := ph.writeValue(msg, fd); err != nil {
			return fmt.Errorf("failed to write value of %q: %w", fd.FullName(), err)
		}
	}

	return nil
}

func (ph *pbHasher) writeValue(msg protoreflect.Message, fd protoreflect.FieldDescriptor) error {
	v := msg.Get(fd)
	switch {
	case fd.IsList():
		return ph.writeListValue(msg, fd, v.List())
	case fd.IsMap():
		return ph.writeMapValue(msg, fd, v.Map())
	default:
		return ph.writeSingularValue(msg, fd, v)
	}
}

func (ph *pbHasher) writeListValue(msg protoreflect.Message, fd protoreflect.FieldDescriptor, list protoreflect.List) error {
	for i := 0; i < list.Len(); i++ {
		if err := ph.writeSingularValue(msg, fd, list.Get(i)); err != nil {
			return err
		}
	}

	return nil
}

func (ph *pbHasher) writeMapValue(msg protoreflect.Message, fd protoreflect.FieldDescriptor, mapv protoreflect.Map) error {
	if mapv.Len() == 0 {
		return nil
	}

	entries := make([]kv, 0, mapv.Len())
	mapv.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		entries = append(entries, kv{key: k, value: v})
		return true
	})

	sort.Slice(entries, func(i, j int) bool {
		return compareMapKeys(entries[i].key, entries[j].key)
	})

	for _, entry := range entries {
		if err := ph.writeSingularValue(msg, fd.MapValue(), entry.value); err != nil {
			return err
		}
	}

	return nil
}

func (ph *pbHasher) writeSingularValue(msg protoreflect.Message, fd protoreflect.FieldDescriptor, v protoreflect.Value) (err error) {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		_, err = ph.hasher.Write(protowire.AppendVarint(nil, protowire.EncodeBool(v.Bool())))
	case protoreflect.EnumKind:
		_, err = ph.hasher.Write(protowire.AppendVarint(nil, uint64(v.Enum())))
	case protoreflect.Int32Kind:
		_, err = ph.hasher.Write(protowire.AppendVarint(nil, uint64(int32(v.Int()))))
	case protoreflect.Sint32Kind:
		_, err = ph.hasher.Write(protowire.AppendVarint(nil, protowire.EncodeZigZag(int64(int32(v.Int())))))
	case protoreflect.Uint32Kind:
		_, err = ph.hasher.Write(protowire.AppendVarint(nil, uint64(uint32(v.Uint()))))
	case protoreflect.Int64Kind:
		_, err = ph.hasher.Write(protowire.AppendVarint(nil, uint64(v.Int())))
	case protoreflect.Sint64Kind:
		_, err = ph.hasher.Write(protowire.AppendVarint(nil, protowire.EncodeZigZag(v.Int())))
	case protoreflect.Uint64Kind:
		_, err = ph.hasher.Write(protowire.AppendVarint(nil, v.Uint()))
	case protoreflect.Sfixed32Kind:
		_, err = ph.hasher.Write(protowire.AppendFixed32(nil, uint32(v.Int())))
	case protoreflect.Fixed32Kind:
		_, err = ph.hasher.Write(protowire.AppendFixed32(nil, uint32(v.Uint())))
	case protoreflect.FloatKind:
		_, err = ph.hasher.Write(protowire.AppendFixed32(nil, math.Float32bits(float32(v.Float()))))
	case protoreflect.Sfixed64Kind:
		_, err = ph.hasher.Write(protowire.AppendFixed64(nil, uint64(v.Int())))
	case protoreflect.Fixed64Kind:
		_, err = ph.hasher.Write(protowire.AppendFixed64(nil, v.Uint()))
	case protoreflect.DoubleKind:
		_, err = ph.hasher.Write(protowire.AppendFixed64(nil, math.Float64bits(v.Float())))
	case protoreflect.StringKind:
		_, err = ph.hasher.Write(protowire.AppendString(nil, v.String()))
	case protoreflect.BytesKind:
		_, err = ph.hasher.Write(protowire.AppendBytes(nil, v.Bytes()))
	case protoreflect.MessageKind:
		err = ph.hash(v.Message())
	default:
		err = fmt.Errorf("unsupported field kind %q", fd.Kind().String())
	}

	return err
}

type kv struct {
	key   protoreflect.MapKey
	value protoreflect.Value
}

func compareMapKeys(a, b protoreflect.MapKey) bool {
	switch t := a.Interface().(type) {
	case bool:
		return !a.Bool() && b.Bool()
	case int32, int64:
		return a.Int() < b.Int()
	case uint32, uint64:
		return a.Uint() < b.Uint()
	case string:
		return a.String() < b.String()
	default:
		panic(fmt.Errorf("unexpected map key type %T", t))
	}
}
