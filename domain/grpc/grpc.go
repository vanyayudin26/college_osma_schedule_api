package grpc

import (
	"net"

	"github.com/vanyayudin26/medcolosma_parser/v2"
	"github.com/vanyayudin26/medcolosma_schedule_api/config"
	"github.com/vanyayudin26/medcolosma_schedule_api/domain/grpc/protobuf"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func Start(cfg config.GRPC, sch *hmtpk_parser.Controller) (err error) {
	lis, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return
	}

	s := grpc.NewServer()

	protobuf.RegisterScheduleServer(s, protobuf.NewServer(cfg, sch))

	log.Tracef("grpc server: %s%s", cfg.Domain, cfg.Address)

	return s.Serve(lis)
}
