package internal

import (
	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

type subscriberService struct {
	env cedar.Environment
}

// AttachSubscriberService attaches the subscriber service to the given gRPC
// server.
func AttachSubscriberService(env cedar.Environment, s *grpc.Server) {
	srv := &subscriberService{
		env: env,
	}
	RegisterSubscriberServer(s, srv)
}

func (s *subscriberService) Subscribe(req *SubscribeRequest, srv Subscriber_SubscribeServer) error {
	ctx := srv.Context()

	topic, err := model.GetTopic(req.Topic)
	if err != nil {
		return newRPCError(codes.NotFound, err)
	}

	data, err := model.WatchTopic(ctx, s.env, req.Topic, req.ResumeToken)
	if err != nil {
		return newRPCError(codes.Internal, err)
	}

	var resumeToken []byte
	for msgEntry := range data {
		resumeToken = msgEntry.ResumeToken

		var msg []byte
		if msgEntry.Err == nil {
			msg, err = topic.ExtractMessage(msgEntry.ForeignID)
			if err != nil {
				msgEntry.Err = errors.Wrap(err, "problem extracting message")
			}
		}

		err = srv.Send(&SubscribeResponse{
			Topic:       req.Topic,
			ResumeToken: resumeToken,
			Msg:         msg,
			ErrorMsg:    msgEntry.Err.Error(),
		})
		if err != nil {
			return newRPCError(codes.Internal, err)
		}
	}

	return nil
}
