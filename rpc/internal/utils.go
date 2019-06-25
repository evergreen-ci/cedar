package internal

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newRPCError(code codes.Code, err error) error {
	if err == nil {
		return nil
	}
	return status.Errorf(code, "%v", err)
}
