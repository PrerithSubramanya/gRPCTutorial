package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	pb "gRPCSerialization/protogen"
)

type customMarshaler struct {
	*runtime.JSONPb
}

func (m *customMarshaler) Marshal(v interface{}) ([]byte, error) {
	// Marshal the response to JSON using the default marshaler
	jsonBytes, err := m.JSONPb.Marshal(v)
	if err != nil {
		return nil, err
	}

	// Unmarshal the JSON into a map
	var jsonMap map[string]interface{}
	err = json.Unmarshal(jsonBytes, &jsonMap)
	if err != nil {
		return nil, err
	}

	// Modify the 'status' field to include both the enum value and descriptor
	if user, ok := jsonMap["user"].(map[string]interface{}); ok {
		if status, ok := user["status"].(string); ok {
			enumValue := pb.UserStatus(pb.UserStatus_value[status])
			delete(user, "status")
			user["status_value"] = getUserStatusDescription(enumValue)
		}
	}

	// Marshal the modified JSON map back to bytes
	return json.Marshal(jsonMap)
}

func getUserStatusDescription(status pb.UserStatus) string {
	statusValDesc := status.Descriptor().Values().ByNumber(protoreflect.EnumNumber(status))
	options := statusValDesc.Options().(*descriptorpb.EnumValueOptions)
	ext := proto.GetExtension(options, pb.E_UserStatusValueOption)
	if str, ok := ext.(string); ok {
		return str
	}

	return "unknown"
}

func getStatusFromString(description string) pb.UserStatus {
	enumType := pb.UserStatus(0).Descriptor()
	for i := 0; i < enumType.Values().Len(); i++ {
		val := enumType.Values().Get(i)
		options := val.Options().(*descriptorpb.EnumValueOptions)
		ext := proto.GetExtension(options, pb.E_UserStatusValueOption)
		if ext.(string) == description {
			return pb.UserStatus(val.Number())
		}

	}

	return pb.UserStatus_USER_STATUS_UNKNOWN
}

// Iterate through all enum values in UserStatu
type userServiceServer struct {
	pb.UnimplementedUserServiceServer
}

func (s *userServiceServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	// Implement the GetUser logic here
	userID := req.UserId

	// Simulate retrieving the user from a database
	user := &pb.User{
		Id:     userID,
		Name:   "John Doe",
		Email:  "john@example.com",
		Status: pb.UserStatus_USER_STATUS_ACTIVE,
	}

	return &pb.GetUserResponse{User: user}, nil
}

func main() {

	status := pb.UserStatus_USER_STATUS_ACTIVE // Assuming this is from your generated code
	description := getUserStatusDescription(status)
	convStatus := getStatusFromString(description)
	fmt.Println("Status description:", convStatus)

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
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &customMarshaler{
			JSONPb: &runtime.JSONPb{
				MarshalOptions: protojson.MarshalOptions{
					UseProtoNames:   true,
					EmitUnpopulated: true,
				},
				UnmarshalOptions: protojson.UnmarshalOptions{
					DiscardUnknown: true,
				},
			},
		}),
	)
	err = pb.RegisterUserServiceHandler(context.Background(), mux, conn)
	if err != nil {
		log.Fatalf("Failed to register gateway: %v", err)
	}

	// httpHandler := newStatusMiddleware(mux)
	log.Println("Starting HTTP server. Listening on port 8080...")
	http.ListenAndServe(":8080", mux)
}
