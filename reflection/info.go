package reflection

import (
	"context"
	"strings"

	"github.com/golang/protobuf/protoc-gen-go/descriptor"
)

type Service struct {
	Name        string   `json:"name"`
	PackageName string   `json:"package_name"`
	Methods     []Method `json:"methods"`
}

type Method struct {
	Name string `json:"name"`

	In  TypeInfo `json:"in"`
	Out TypeInfo `json:"out"`

	InStream  bool `json:"in_stream,omitempty"`
	OutStream bool `json:"out_stream,omitempty"`
}

type TypeInfo struct {
	Id      int                        `json:"id"`
	Name    string                     `json:"name"`
	Fields  []FieldInfo                `json:"fields,omitempty"`
	Options *descriptor.MessageOptions `json:"options,omitempty"`
}

type FieldInfo struct {
	Name    string                                `json:"name"`
	Number  int                                   `json:"number"`
	Label   descriptor.FieldDescriptorProto_Label `json:"label,omitempty"`
	Type    TypeInfo                              `json:"type"`
	Enum    EnumInfo                              `json:"enum"`
	Options *descriptor.FieldOptions              `json:"options,omitempty"`
}

type EnumInfo struct {
	Name   string          `json:"name"`
	Values []EnumValueInfo `json:"values,omitempty"`
}

type EnumValueInfo struct {
	Name   string `json:"name"`
	Number int    `json:"number"`
}

func GetInfo(ctx context.Context, addr string) ([]Service, error) {
	pool := &descPool{}
	if err := pool.connect(ctx, addr); err != nil {
		return nil, err
	}

	defer pool.disconnect()

	services, err := pool.getServicesDescriptors()
	if err != nil {
		return nil, err
	}

	res := make([]Service, 0, len(services))
	for sname, descr := range services {

		packageName := strings.Split(sname, "/")[0]

		if packageName == "grpc.reflection.v1alpha" {
			continue
		}

		s := Service{
			Name:        *descr.Name,
			PackageName: packageName,
		}
		s.Methods = make([]Method, len(descr.Method))
		for i, method := range descr.Method {
			s.Methods[i] = Method{
				Name: method.GetName(),

				In:  GetTypeInfo(pool, method.GetInputType()),
				Out: GetTypeInfo(pool, method.GetOutputType()),

				InStream:  method.GetClientStreaming(),
				OutStream: method.GetServerStreaming(),
			}
		}

		res = append(res, s)
	}

	return res, nil
}

func GetTypeInfo(pool *descPool, typeName string) (res TypeInfo) {
	desc := pool.getTypeDescriptor(typeName)

	// ERROR
	if desc == nil {
		return TypeInfo{
			Id:   0, // ERROR TYPE
			Name: typeName,
		}
	}

	info := TypeInfo{
		Name:    desc.GetName(),
		Fields:  make([]FieldInfo, len(desc.GetField())),
		Options: desc.GetOptions(),
	}

	for i, field := range desc.GetField() {
		info.Fields[i].Name = field.GetName()
		info.Fields[i].Number = int(field.GetNumber())
		info.Fields[i].Label = field.GetLabel()
		info.Fields[i].Options = field.GetOptions()

		fieldType := field.GetType()
		fieldTypeName := field.GetTypeName()

		if fieldTypeName != "" {
			info.Fields[i].Type = GetTypeInfo(pool, fieldTypeName)
			info.Fields[i].Type.Id = int(fieldType)
		} else {
			info.Fields[i].Type = TypeInfo{
				Name: strings.ToLower(fieldType.String()[5:]),
				Id:   int(fieldType),
			}
		}

		if fieldType == descriptor.FieldDescriptorProto_TYPE_ENUM {
			info.Fields[i].Type.Name = "enum"
			if enumInfo := pool.getEnumDescriptor(fieldTypeName); enumInfo != nil {
				info.Fields[i].Enum.Name = enumInfo.GetName()
				for _, ee := range enumInfo.Value {
					info.Fields[i].Enum.Values = append(info.Fields[i].Enum.Values, EnumValueInfo{
						Name:   ee.GetName(),
						Number: int(ee.GetNumber()),
					})
				}
			}
		}

	}
	return info
}
