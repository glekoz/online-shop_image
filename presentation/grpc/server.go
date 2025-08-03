package grpc

import (
	"net"

	"github.com/glekoz/online-shop_proto/protoimage"
	"google.golang.org/grpc"
)

const (
	maxSize        = 5 << 20
	maxMessageSize = 1 << 20
)

type ImageServer struct {
	App AppAPI
	protoimage.UnimplementedImageServer
}

func NewServer(app AppAPI) *ImageServer {
	return &ImageServer{App: app}
}

func (IS *ImageServer) RunServer(app AppAPI) error { // все общие компоненты должны настраиваться в мейне
	listen, err := net.Listen("tcp", ":8080")
	if err != nil {
		return err
	}
	grpcServer := grpc.NewServer(grpc.MaxRecvMsgSize(maxMessageSize))
	protoimage.RegisterImageServer(grpcServer, IS)
	return grpcServer.Serve(listen)
}
