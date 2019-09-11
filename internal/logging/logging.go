package logging

import (
	"context"
	"path"
	"time"

	"github.com/gofrs/uuid"
	grpc_logging "github.com/grpc-ecosystem/go-grpc-middleware/logging"
	grpc_logrus "github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus/ctxlogrus"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// ContextKey defines the context key type.
type ContextKey string

// ContextIDKey holds the key of the context ID.
const ContextIDKey ContextKey = "ctx_id"

type contextIDGetter interface {
	GetContextId() []byte
}

// UnaryServerCtxIDInterceptor adds the ContextIDKey to the context and sets
// it as a log field.
func UnaryServerCtxIDInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	ctxID, err := uuid.NewV4()
	if err != nil {
		return nil, errors.Wrap(err, "new uuid error")
	}
	ctx = context.WithValue(ctx, ContextIDKey, ctxID)
	ctxlogrus.AddFields(ctx, log.Fields{
		"ctx_id": ctxID,
	})

	return handler(ctx, req)
}

func UnaryClientCtxIDInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	startTime := time.Now()
	err := invoker(ctx, method, req, reply, cc, opts...)

	code := grpc_logging.DefaultErrorToCode(err)
	level := grpc_logrus.DefaultCodeToLevel(code)
	logFields := clientLoggerFields(ctx, method, reply, err, code, startTime)

	levelLogf(log.WithFields(logFields), level, "finished client unary call")

	return err
}

func clientLoggerFields(ctx context.Context, fullMethodString string, resp interface{}, err error, code codes.Code, start time.Time) logrus.Fields {
	service := path.Dir(fullMethodString)[1:]
	method := path.Base(fullMethodString)

	fields := logrus.Fields{
		"system":        "grpc",
		"span.kind":     "client",
		"grpc.service":  service,
		"grpc.method":   method,
		"grpc.duration": time.Since(start),
		"grpc.code":     code.String(),
	}

	if getter, ok := resp.(contextIDGetter); ok {
		var ctxID uuid.UUID
		copy(ctxID[:], getter.GetContextId())

		fields["ctx_id"] = ctxID
	}

	if err != nil {
		fields[logrus.ErrorKey] = err
	}

	return fields
}

func levelLogf(entry *logrus.Entry, level logrus.Level, format string, args ...interface{}) {
	switch level {
	case logrus.DebugLevel:
		entry.Debugf(format, args...)
	case logrus.InfoLevel:
		entry.Infof(format, args...)
	case logrus.WarnLevel:
		entry.Warningf(format, args...)
	case logrus.ErrorLevel:
		entry.Errorf(format, args...)
	case logrus.FatalLevel:
		entry.Fatalf(format, args...)
	case logrus.PanicLevel:
		entry.Panicf(format, args...)
	}
}
