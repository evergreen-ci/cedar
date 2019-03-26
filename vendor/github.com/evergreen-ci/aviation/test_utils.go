package aviation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type mockServerStream struct {
	ctx     context.Context
	sentMsg string
	recvMsg string
}

func (_ *mockServerStream) SetHeader(_ metadata.MD) error  { return nil }
func (_ *mockServerStream) SendHeader(_ metadata.MD) error { return nil }
func (_ *mockServerStream) SetTrailer(_ metadata.MD)       {}
func (s *mockServerStream) Context() context.Context       { return s.ctx }
func (_ *mockServerStream) RecvMsg(_ interface{}) error    { return nil }
func (s *mockServerStream) SendMsg(m interface{}) error {
	switch i := m.(type) {
	case string:
		s.sentMsg = i
	case []byte:
		s.sentMsg = string(i)
	}

	return nil
}

func mockUnaryHandler(_ context.Context, req interface{}) (interface{}, error) {
	switch t := req.(type) {
	case string:
		if t == "panic" {
			panic("test panic")
		}
	}
	return nil, nil
}

func mockStreamHandler(srv interface{}, stream grpc.ServerStream) error {
	switch t := srv.(type) {
	case string:
		if t == "panic" {
			panic("test panic")
		}
	}
	return nil
}

func requireContextValue(t *testing.T, ctx context.Context, key string, expectedVal interface{}, msg ...interface{}) {
	val := ctx.Value(key)
	require.NotNil(t, val, msg...)
	require.Equal(t, expectedVal, val, msg...)
}
