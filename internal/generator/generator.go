// Copyright 2021-2022 Zenauth Ltd.
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"fmt"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"sort"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

const (
	funcSuffix    = "_hashpb_sum"
	hasherImp     = protogen.GoImportPath("hash")
	mathImp       = protogen.GoImportPath("math")
	protowireImp  = protogen.GoImportPath("google.golang.org/protobuf/encoding/protowire")
	mapsImp       = protogen.GoImportPath("maps")
	slicesImp     = protogen.GoImportPath("slices")
	receiverIdent = "m"
)

var (
	Version = "dev"

	appendBytesFn   = protowireImp.Ident("AppendBytes")
	appendFixed32Fn = protowireImp.Ident("AppendFixed32")
	appendFixed64Fn = protowireImp.Ident("AppendFixed64")
	appendStringFn  = protowireImp.Ident("AppendString")
	appendVarintFn  = protowireImp.Ident("AppendVarint")
	encodeBoolFn    = protowireImp.Ident("EncodeBool")
	encodeZigZagFn  = protowireImp.Ident("EncodeZigZag")
	float32BitsFn   = mathImp.Ident("Float32bits")
	float64BitsFn   = mathImp.Ident("Float64bits")
	hashFn          = hasherImp.Ident("Hash")
	mapKeysFn       = mapsImp.Ident("Keys")
	sortedFn        = slicesImp.Ident("Sorted")

	nonIdentifierChars = regexp.MustCompile(`[^\w]+`)
)

func init() {
	if Version == "dev" {
		if bi, ok := debug.ReadBuildInfo(); ok {
			Version = bi.Main.Version
		}
	}
}

func Generate(p *protogen.Plugin) error {
	p.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL) | uint64(pluginpb.CodeGeneratorResponse_FEATURE_SUPPORTS_EDITIONS)
	p.SupportedEditionsMinimum = descriptorpb.Edition_EDITION_2023
	p.SupportedEditionsMaximum = descriptorpb.Edition_EDITION_2024
	// group files by import path because the helpers need to be generated at the package level.
	pkgFiles := make(map[protogen.GoImportPath][]*protogen.File)
	for _, f := range p.Files {
		if !f.Generate {
			continue
		}

		switch f.Desc.Syntax() {
		case protoreflect.Editions, protoreflect.Proto3:
		default:
			return fmt.Errorf("unsupported syntax %q: %s", f.Desc.Syntax(), f.Desc.Path())
		}

		pkgFiles[f.GoImportPath] = append(pkgFiles[f.GoImportPath], f)
	}

	g := &codegen{Plugin: p}
	for _, files := range pkgFiles {
		g.generateHelpers(files)
		g.generateMethods(files)
	}

	return nil
}

type codegen struct {
	*protogen.Plugin
}

// generateHelpers generates helper functions for calculating the hash for each message type.
// Because messages can be recursive, we need to do this to avoid getting into an infinite loop.
func (g *codegen) generateHelpers(files []*protogen.File) {
	if len(files) == 0 {
		return
	}

	// find all messages referenced by the files.
	msgsToGen := make(map[string]*protogen.Message)
	for _, f := range files {
		for _, msg := range f.Messages {
			collectMessages(msgsToGen, msg)
		}
	}

	if len(msgsToGen) == 0 {
		return
	}

	fileName := filepath.Join(filepath.Dir(files[0].Desc.Path()), "hashpb_helpers.pb.go")
	gf := g.newGeneratedFile(fileName, files, nil)

	// sort message names to make the generated file predictable (no spurious diffs)
	msgNames := make([]string, len(msgsToGen))
	i := 0
	for mn := range msgsToGen {
		msgNames[i] = mn
		i++
	}
	sort.Strings(msgNames)

	for _, mn := range msgNames {
		g.genHelperForMsg(gf, msgsToGen[mn])
		gf.P()
	}
}

func collectMessages(col map[string]*protogen.Message, msg *protogen.Message) {
	// ignore the special messages generated for map entries
	if msg.Desc.IsMapEntry() {
		for _, f := range msg.Fields {
			if f.Message != nil {
				collectMessages(col, f.Message)
			}
		}
		return
	}

	fnName := sumFuncName(msg.Desc)
	if _, ok := col[fnName]; ok {
		return
	}

	col[fnName] = msg
	for _, f := range msg.Fields {
		if f.Message != nil {
			collectMessages(col, f.Message)
		}
	}
}

func sumFuncName(md protoreflect.MessageDescriptor) string {
	fqn := nonIdentifierChars.ReplaceAllLiteralString(string(md.FullName()), "_")
	return fqn + funcSuffix
}

func (g *codegen) genHelperForMsg(gf *protogen.GeneratedFile, msg *protogen.Message) {
	fields := make([]*protogen.Field, len(msg.Fields))
	copy(fields, msg.Fields)

	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Desc.Number() < fields[j].Desc.Number()
	})

	gf.P("func ", sumFuncName(msg.Desc), "(", receiverIdent, " *", msg.GoIdent, ",hasher ", hashFn, ", ignore map[string]struct{}) {")

	oneOfs := make(map[string]struct{})

	for _, field := range fields {
		if field.Oneof != nil && !field.Oneof.Desc.IsSynthetic() {
			if _, ok := oneOfs[field.Oneof.GoName]; !ok {
				g.genOneOfField(gf, field)
				oneOfs[field.Oneof.GoName] = struct{}{}
			}
		} else {
			g.genField(gf, field)
		}
	}

	gf.P("}")
}

func (g *codegen) genField(gf *protogen.GeneratedFile, field *protogen.Field) {
	gf.P("if _, ok := ignore[\"", field.Desc.FullName(), "\"]; !ok {")

	switch {
	case field.Desc.IsList():
		g.genListField(gf, field)
	case field.Desc.IsMap():
		g.genMapField(gf, field)
	default:
		g.genSingularField(gf, field.Desc, fieldAccess(fmt.Sprintf("Get%s()", field.GoName)))
	}

	gf.P("}")
}

func (g *codegen) genOneOfField(gf *protogen.GeneratedFile, field *protogen.Field) {
	fieldName := fieldAccess(field.Oneof.GoName)

	gf.P("if ", fieldName, " != nil {")
	gf.P("if _, ok := ignore[\"", field.Desc.ContainingOneof().FullName(), "\"]; !ok {")
	gf.P("switch t := ", fieldName, ".(type) {")
	for _, f := range field.Oneof.Fields {
		gf.P("case *", f.GoIdent, ":")
		g.genSingularField(gf, f.Desc, "t."+f.GoName)
	}
	gf.P("}")
	gf.P("}")
	gf.P("}")
}

func (g *codegen) genListField(gf *protogen.GeneratedFile, field *protogen.Field) {
	fieldName := fieldAccess(field.GoName)
	gf.P("if len(", fieldName, ") > 0 {")
	gf.P("for _, v := range ", fieldName, " {")
	g.genSingularField(gf, field.Desc, "v")
	gf.P("}")
	gf.P("}")
}

func (g *codegen) genMapField(gf *protogen.GeneratedFile, field *protogen.Field) {
	fieldName := fieldAccess(field.GoName)
	if field.Desc.MapKey().Kind() == protoreflect.BoolKind {
		for _, k := range []bool{false, true} {
			gf.P("if v, ok := ", fieldName, "[", k, "]; ok {")
			g.genSingularField(gf, field.Desc.MapValue(), "v")
			gf.P("}")
		}
	} else {
		gf.P("if len(", fieldName, ") > 0 {")
		gf.P("for _, k := range ", sortedFn, "(", mapKeysFn, "(", fieldName, ")) {")
		g.genSingularField(gf, field.Desc.MapValue(), fmt.Sprintf("%s[k]", fieldName))
		gf.P("}")
		gf.P("}")
	}
}

func (g *codegen) genSingularField(gf *protogen.GeneratedFile, fieldDesc protoreflect.FieldDescriptor, fieldName string) {
	writeFn := "_, _ = hasher.Write("

	switch fieldDesc.Kind() {
	case protoreflect.BoolKind:
		// hasher.Write(protowire.AppendVarint(nil, protowire.EncodeBool(...)))
		gf.P(writeFn, appendVarintFn, "(nil, ", encodeBoolFn, "(", fieldName, ")))")
	case protoreflect.EnumKind:
		// hasher.Write(protowire.AppendVarint(nil, uint64(...)))
		gf.P(writeFn, appendVarintFn, "(nil, uint64(", fieldName, ")))")
	case protoreflect.Int32Kind:
		// hasher.Write(protowire.AppendVarint(nil, uint64(...)))
		gf.P(writeFn, appendVarintFn, "(nil, uint64(", fieldName, ")))")
	case protoreflect.Sint32Kind:
		// hasher.Write(protowire.AppendVarint(nil, protowire.EncodeZigZag(int64(...))))
		gf.P(writeFn, appendVarintFn, "(nil, ", encodeZigZagFn, "(int64(", fieldName, "))))")
	case protoreflect.Uint32Kind:
		// hasher.Write(protowire.AppendVarint(nil, uint64(...)))
		gf.P(writeFn, appendVarintFn, "(nil, uint64(", fieldName, ")))")
	case protoreflect.Int64Kind:
		// hasher.Write(protowire.AppendVarint(nil, uint64(...)))
		gf.P(writeFn, appendVarintFn, "(nil, uint64(", fieldName, ")))")
	case protoreflect.Sint64Kind:
		// hasher.Write(protowire.AppendVarint(nil, protowire.EncodeZigZag(...)))
		gf.P(writeFn, appendVarintFn, "(nil, ", encodeZigZagFn, "(", fieldName, ")))")
	case protoreflect.Uint64Kind:
		// hasher.Write(protowire.AppendVarint(nil, ...))
		gf.P(writeFn, appendVarintFn, "(nil, ", fieldName, "))")
	case protoreflect.Sfixed32Kind:
		// hasher.Write(protowire.AppendFixed32(nil, uint32(...)))
		gf.P(writeFn, appendFixed32Fn, "(nil, uint32(", fieldName, ")))")
	case protoreflect.Fixed32Kind:
		// hasher.Write(protowire.AppendFixed32(nil, uint32(...)))
		gf.P(writeFn, appendFixed32Fn, "(nil, uint32(", fieldName, ")))")
	case protoreflect.FloatKind:
		// hasher.Write(protowire.AppendFixed32(nil, math.Float32bits(...)))
		gf.P(writeFn, appendFixed32Fn, "(nil,", float32BitsFn, "(", fieldName, ")))")
	case protoreflect.Sfixed64Kind:
		// hasher.Write(protowire.AppendFixed64(nil, uint64(...)))
		gf.P(writeFn, appendFixed64Fn, "(nil, uint64(", fieldName, ")))")
	case protoreflect.Fixed64Kind:
		// hasher.Write(protowire.AppendFixed64(nil, ...))
		gf.P(writeFn, appendFixed64Fn, "(nil, ", fieldName, "))")
	case protoreflect.DoubleKind:
		// hasher.Write(protowire.AppendFixed64(nil, math.Float64bits(...)))
		gf.P(writeFn, appendFixed64Fn, "(nil,", float64BitsFn, "(", fieldName, ")))")
	case protoreflect.StringKind:
		// hasher.Write(protowire.AppendString(nil, ...))
		gf.P(writeFn, appendStringFn, "(nil, ", fieldName, "))")
	case protoreflect.BytesKind:
		// hasher.Write(protowire.AppendBytes(nil, ...))
		gf.P(writeFn, appendBytesFn, "(nil, ", fieldName, "))")
	case protoreflect.MessageKind:
		gf.P("if ", fieldName, " != nil {")
		gf.P(sumFuncName(fieldDesc.Message()), "(", fieldName, ",hasher, ignore)")
		gf.P("}")
	default:
		panic(fmt.Errorf("unhandled field kind %s", fieldDesc.Kind().String()))
	}
}

func fieldAccess(name string) string {
	return fmt.Sprintf("%s.%s", receiverIdent, name)
}

// generateMethods generates helper methods (HashPB) for the top level messages defined in each file.
func (g *codegen) generateMethods(files []*protogen.File) {
	for _, f := range files {
		gf := g.newGeneratedFile(f.GeneratedFilenamePrefix+"_hashpb.pb.go", files, f)
		genFuncs := make(map[string]struct{})

		for _, msg := range f.Messages {
			g.genMethodForMsg(gf, genFuncs, msg)
		}
	}
}

func (g *codegen) newGeneratedFile(filename string, files []*protogen.File, source *protogen.File) *protogen.GeneratedFile {
	gf := g.NewGeneratedFile(filename, files[0].GoImportPath)
	gf.P("// Code generated by protoc-gen-go-hashpb. DO NOT EDIT.")
	gf.P("// protoc-gen-go-hashpb ", Version)
	if source != nil {
		gf.P("// Source: ", source.Desc.Path())
	}
	gf.P()
	gf.P("package ", files[0].GoPackageName)
	gf.P()
	return gf
}

func (g *codegen) genMethodForMsg(gf *protogen.GeneratedFile, genFuncs map[string]struct{}, msg *protogen.Message) {
	if msg.Desc.IsMapEntry() {
		return
	}

	if len(msg.Fields) == 0 {
		return
	}

	if _, ok := genFuncs[msg.GoIdent.GoName]; ok {
		return
	}

	genFuncs[msg.GoIdent.GoName] = struct{}{}

	gf.P("// HashPB computes a hash of the message using the given hash function")
	gf.P("// The ignore set must contain fully-qualified field names (pkg.msg.field) that should be ignored from the hash")
	gf.P("func (", receiverIdent, " *", msg.GoIdent, ") HashPB(hasher ", hashFn, ", ignore map[string]struct{}) {")
	gf.P("if ", receiverIdent, " != nil {")
	gf.P(sumFuncName(msg.Desc), "(", receiverIdent, ", hasher, ignore)")
	gf.P("}")
	gf.P("}")
	gf.P()

	for _, msg := range msg.Messages {
		g.genMethodForMsg(gf, genFuncs, msg)
	}
}
