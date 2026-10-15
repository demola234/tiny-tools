package protos

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strings"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/linker"
	"github.com/bufbuild/protocompile/reporter"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

type Problem struct {
	File string
	Line int
	Msg  string
}

var compiling = context.Background()

type Set struct {
	files   linker.Files
	methods map[string]protoreflect.MethodDescriptor
}

func Compile(sources map[string]string) (*Set, []Problem) {
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	slices.Sort(names)
	var probs []Problem
	report := func(err reporter.ErrorWithPos) error {
		pos := err.GetPosition()
		probs = append(probs, Problem{File: pos.Filename, Line: pos.Line, Msg: err.Unwrap().Error()})
		return nil
	}
	c := protocompile.Compiler{
		Resolver:       protocompile.WithStandardImports(&protocompile.SourceResolver{Accessor: protocompile.SourceAccessorFromMap(sources)}),
		Reporter:       reporter.NewReporter(report, nil),
		SourceInfoMode: protocompile.SourceInfoStandard,
	}
	files, err := c.Compile(compiling, names...)
	if err != nil && len(probs) == 0 {
		probs = append(probs, Problem{Msg: err.Error()})
	}
	if len(probs) > 0 {
		return nil, probs
	}
	s := &Set{files: files, methods: map[string]protoreflect.MethodDescriptor{}}
	for _, f := range files {
		for i := range f.Services().Len() {
			svc := f.Services().Get(i)
			for j := range svc.Methods().Len() {
				m := svc.Methods().Get(j)
				s.methods[string(svc.FullName())+"/"+string(m.Name())] = m
			}
		}
	}
	return s, nil
}

func (s *Set) Methods() []string {
	out := make([]string, 0, len(s.methods))
	for name := range s.methods {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

func (s *Set) Method(name string) (protoreflect.MethodDescriptor, bool) {
	m, ok := s.methods[name]
	return m, ok
}

func (s *Set) Services() []protoreflect.ServiceDescriptor {
	var out []protoreflect.ServiceDescriptor
	for _, f := range s.files {
		for i := range f.Services().Len() {
			out = append(out, f.Services().Get(i))
		}
	}
	return out
}

func (s *Set) Registry() (*protoregistry.Files, error) {
	reg := new(protoregistry.Files)
	for _, f := range s.files {
		if err := addWithDeps(reg, f); err != nil {
			return nil, err
		}
	}
	return reg, nil
}

func addWithDeps(reg *protoregistry.Files, f protoreflect.FileDescriptor) error {
	if _, err := reg.FindFileByPath(f.Path()); err == nil {
		return nil
	}
	for i := range f.Imports().Len() {
		if err := addWithDeps(reg, f.Imports().Get(i).FileDescriptor); err != nil {
			return err
		}
	}
	fd, err := protodesc.NewFile(protodesc.ToFileDescriptorProto(f), reg)
	if err != nil {
		return err
	}
	return reg.RegisterFile(fd)
}

var (
	position  = regexp.MustCompile(`\(line \d+:\d+\):\s*`)
	unmarshal = protojson.UnmarshalOptions{}
	marshal   = protojson.MarshalOptions{UseProtoNames: true}
)

func FromJSON(desc protoreflect.MessageDescriptor, data []byte) (proto.Message, error) {
	msg := dynamicpb.NewMessage(desc)
	if err := unmarshal.Unmarshal(data, msg); err != nil {
		return nil, errors.New(strings.TrimSpace(position.ReplaceAllString(strings.ReplaceAll(err.Error(), "proto:", ""), "")))
	}
	return msg, nil
}

func ToJSON(msg proto.Message) ([]byte, error) {
	data, err := marshal.Marshal(msg)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	err = json.Compact(&b, data)
	return b.Bytes(), err
}

func New(desc protoreflect.MessageDescriptor) *dynamicpb.Message { return dynamicpb.NewMessage(desc) }
