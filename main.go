package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	pb "gRPCSerialization/protogen"
)

var userList []pb.User

type customEnumMarshaler struct {
	runtime.JSONPb
}

func (m *customEnumMarshaler) Marshal(v interface{}) ([]byte, error) {
	pb, ok := v.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("expected proto.Message but got %T", v)
	}

	// fmt.Printf("This is Marshalpb %s", pb)

	jsonBytes, err := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(pb)
	if err != nil {
		return nil, err
	}

	var jsonData map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &jsonData); err != nil {
		return nil, err
	}

	convertEnumsToCustomOptions(jsonData, pb.ProtoReflect())

	return json.Marshal(jsonData)
}

func (m *customEnumMarshaler) Unmarshal(data []byte, v interface{}) error {
	fmt.Printf("This is Unmarshalpb %s", v.(string))
	pb, ok := v.(proto.Message)
	if !ok {
		return fmt.Errorf("expected proto.Message but got %T", v)
	}

	// Unmarshal JSON data into proto message
	if err := protojson.Unmarshal(data, pb); err != nil {
		return err
	}

	// Convert custom options to enum values
	convertCustomOptionsToEnums(pb.ProtoReflect())

	return nil
}

func convertCustomOptionsToEnums(pb protoreflect.Message) {
	fields := pb.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		if field.Kind() == protoreflect.EnumKind {
			enumDesc := field.Enum()
			value := pb.Get(field).Enum()

			customOption, err := getCustomOptionForEnumValue(enumDesc, protoreflect.EnumNumber(value))
			if err == nil {
				enumValue := enumDesc.Values().ByName(protoreflect.Name(customOption))
				pb.Set(field, protoreflect.ValueOfEnum(enumValue.Number()))
			}
		} else if field.Kind() == protoreflect.MessageKind {
			nestedMsg := pb.Mutable(field).Message()
			convertCustomOptionsToEnums(nestedMsg)
		}
	}
}

func getCustomOptionForEnumValue(enumDesc protoreflect.EnumDescriptor, value protoreflect.EnumNumber) (string, error) {
	for i := 0; i < enumDesc.Values().Len(); i++ {
		val := enumDesc.Values().Get(i)
		opts := val.Options().(*descriptorpb.EnumValueOptions)
		ext := proto.GetExtension(opts, pb.E_EnumTrim)

		if ext.(string) == fmt.Sprintf("%d", value) {
			return string(val.Name()), nil
		}
	}
	return "", fmt.Errorf("enum value %d not found for enum %s", value, enumDesc.FullName())
}

func convertEnumsToCustomOptions(jsonData map[string]interface{}, pb protoreflect.Message) {
	fields := pb.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		if field.Kind() == protoreflect.EnumKind {
			if value, exists := jsonData[field.JSONName()]; exists {
				if enumValue, ok := value.(string); ok {
					customValue, err := getCustomOptionForEnum(field.Enum(), enumValue)
					if err == nil {
						jsonData[field.JSONName()] = strings.TrimPrefix(enumValue, customValue)
					}
				}
			}
		} else if field.Kind() == protoreflect.MessageKind {
			if nestedValue, exists := jsonData[field.JSONName()]; exists {
				if nestedMap, ok := nestedValue.(map[string]interface{}); ok {
					convertEnumsToCustomOptions(nestedMap, pb.Get(field).Message())
				}
			}
		}
	}
}

func getCustomOptionForEnum(enumDesc protoreflect.EnumDescriptor, value string) (string, error) {
	var enumValueDesc protoreflect.EnumValueDescriptor
	for i := 0; i < enumDesc.Values().Len(); i++ {
		val := enumDesc.Values().Get(i)
		if val.Name() == protoreflect.Name(value) {
			enumValueDesc = val
			break
		}
	}

	if enumValueDesc == nil {
		return "", fmt.Errorf("enum value %s not found for enum %s", value, enumDesc.FullName())
	}

	opts := enumValueDesc.Options().(*descriptorpb.EnumValueOptions)
	ext := proto.GetExtension(opts, pb.E_EnumTrim)

	return ext.(string), nil
}

type userServiceServer struct {
	pb.UnimplementedUserServiceServer
}

func (s *userServiceServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	// Implement the GetUser logic here
	userID := int(req.UserId)

	// fmt.Printf("Getting user %d", userID)
	if userID < 0 {
		return nil, fmt.Errorf("user not found")
	}
	for i := 0; i < len(userList); i++ {
		if int(userList[i].Id) == userID {
			return &pb.GetUserResponse{User: &userList[i]}, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

func (s *userServiceServer) CreateUser(ctx context.Context, req *pb.User) (*pb.GetUserResponse, error) {
	newUser := proto.Clone(req).(*pb.User)

	// fmt.Println("Creating new user: ", req)

	userList = append(userList, *newUser)

	return &pb.GetUserResponse{User: newUser}, nil
}

func main() {

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterUserServiceServer(s, &userServiceServer{})

	go func() {
		log.Println("Starting gRPC server. Listening on port 50051...")
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to dial gRPC server: %v", err)
	}
	defer conn.Close()

	mux := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &customEnumMarshaler{}),
	)
	err = pb.RegisterUserServiceHandler(context.Background(), mux, conn)
	if err != nil {
		log.Fatalf("Failed to register gateway: %v", err)
	}

	// httpHandler := newStatusMiddleware(mux)
	log.Println("Starting HTTP server. Listening on port 8080...")
	http.ListenAndServe(":8080", mux)
}
