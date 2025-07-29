package grpc

import (
	"context"
	"image"
	"net"

	"github.com/Gleb988/online-shop_proto/protoimage"
	"google.golang.org/grpc"
)

const (
	maxSize        = 5 << 20
	maxMessageSize = 1 << 20
)

type AppAPI interface {
	InitialSave(ctx context.Context, service, dirName string, img image.Image) error
}

type ImageServer struct {
	app AppAPI
	protoimage.UnimplementedImageServer
}

func NewServer(app AppAPI) *ImageServer {
	return &ImageServer{app: app}
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
