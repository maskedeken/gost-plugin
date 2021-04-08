package gun

import (
	context "context"

	grpc "google.golang.org/grpc"
)

type GunServiceClientX interface {
	Tun(ctx context.Context, opts ...grpc.CallOption) (GunService_TunClient, error)
	TunCustomName(ctx context.Context, name string, opts ...grpc.CallOption) (GunService_TunClient, error)
}

func (c *gunServiceClient) TunCustomName(ctx context.Context, name string, opts ...grpc.CallOption) (GunService_TunClient, error) {
	stream, err := c.cc.NewStream(ctx, &ServerDesc(name).Streams[0], "/"+name+"/Tun", opts...)
	if err != nil {
		return nil, err
	}
	x := &gunServiceTunClient{stream}
	return x, nil
}

func ServerDesc(name string) grpc.ServiceDesc {
	return grpc.ServiceDesc{
		ServiceName: name,
		HandlerType: (*GunServiceServer)(nil),
		Methods:     []grpc.MethodDesc{},
		Streams: []grpc.StreamDesc{
			{
				StreamName:    "Tun",
				Handler:       _GunService_Tun_Handler,
				ServerStreams: true,
				ClientStreams: true,
			},
		},
		Metadata: "gost/protocol/gun/gun.proto",
	}
}

func NewGunServiceClientX(cc grpc.ClientConnInterface) GunServiceClientX {
	return &gunServiceClient{cc}
}
