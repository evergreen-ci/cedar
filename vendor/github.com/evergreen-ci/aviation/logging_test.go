package aviation

import (
	"context"
	"testing"

	"github.com/mongodb/grip"
	"github.com/mongodb/grip/logging"
	"github.com/mongodb/grip/send"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestGripInterceptors(t *testing.T) {
	for _, test := range []struct {
		name       string
		unaryInfo  *grpc.UnaryServerInfo
		streamInfo *grpc.StreamServerInfo
		err        bool
	}{
		{
			name:      "ValidLogging",
			unaryInfo: &grpc.UnaryServerInfo{},
		},
		{
			name:      "PanicLogging",
			unaryInfo: &grpc.UnaryServerInfo{},
			err:       true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Run("Unary", func(t *testing.T) {
				sender, err := send.NewInternalLogger("test", grip.GetSender().Level())
				require.NoError(t, err)
				journaler := logging.MakeGrip(sender)
				startAt := getNumber()

				interceptor := MakeGripUnaryInterceptor(journaler)
				if test.err {
					_, err = interceptor(context.TODO(), "panic", &grpc.UnaryServerInfo{}, mockUnaryHandler)
					assert.Error(t, err)
				} else {
					_, err = interceptor(context.TODO(), nil, &grpc.UnaryServerInfo{}, mockUnaryHandler)
					assert.NoError(t, err)
				}

				assert.Equal(t, startAt+2, getNumber())
				assert.True(t, sender.HasMessage())
				assert.Equal(t, 2, sender.Len())
			})
			t.Run("Streaming", func(t *testing.T) {
				sender, err := send.NewInternalLogger("test", grip.GetSender().Level())
				require.NoError(t, err)
				journaler := logging.MakeGrip(sender)
				startAt := getNumber()

				interceptor := MakeGripStreamInterceptor(journaler)
				if test.err {
					err = interceptor("panic", &mockServerStream{}, &grpc.StreamServerInfo{}, mockStreamHandler)
					assert.Error(t, err)
				} else {
					err = interceptor(nil, &mockServerStream{}, &grpc.StreamServerInfo{}, mockStreamHandler)
					assert.NoError(t, err)
				}

				assert.Equal(t, startAt+2, getNumber())
				assert.True(t, sender.HasMessage())
				assert.Equal(t, 2, sender.Len())
			})
		})
	}
}
