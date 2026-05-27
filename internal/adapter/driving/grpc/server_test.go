package grpcadapter

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"

	chsmdocv1 "github.com/pt9912/c-hsm-doc/internal/gen/chsmdocv1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
)

const bufSize = 1024 * 1024

// dialBufconn baut einen in-process gRPC-Client gegen einen
// bufconn-Listener. Vermeidet echte TCP-/TLS-Sockets im Unit-Test.
func dialBufconn(t *testing.T) (chsmdocv1.HsmDocServiceClient, func()) {
	t.Helper()

	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	chsmdocv1.RegisterHsmDocServiceServer(srv, NewServer())

	go func() {
		_ = srv.Serve(lis)
	}()

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}
	conn, err := grpc.NewClient(
		"passthrough://bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	client := chsmdocv1.NewHsmDocServiceClient(conn)

	cleanup := func() {
		_ = conn.Close()
		srv.GracefulStop()
		_ = lis.Close()
	}
	return client, cleanup
}

func assertUnimplemented(t *testing.T, rpc string, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected error, got nil", rpc)
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("%s: not a gRPC status: %v", rpc, err)
	}
	if st.Code() != codes.Unimplemented {
		t.Errorf("%s: code = %s, want Unimplemented", rpc, st.Code())
	}
}

func TestListKeysUnimplemented(t *testing.T) {
	client, cleanup := dialBufconn(t)
	defer cleanup()

	_, err := client.ListKeys(context.Background(), &chsmdocv1.ListKeysRequest{TenantId: "test"})
	assertUnimplemented(t, "ListKeys", err)
}

func TestHealthUnimplemented(t *testing.T) {
	client, cleanup := dialBufconn(t)
	defer cleanup()

	_, err := client.Health(context.Background(), &emptypb.Empty{})
	assertUnimplemented(t, "Health", err)
}

func TestEncryptUnimplemented(t *testing.T) {
	client, cleanup := dialBufconn(t)
	defer cleanup()

	stream, err := client.Encrypt(context.Background())
	if err != nil {
		t.Fatalf("Encrypt open: %v", err)
	}
	// Send kann EOF zurückgeben, sobald der Server Unimplemented schließt
	// (Race zwischen Send und Server-Close). Der maßgebliche Fehler liegt
	// im Recv-Pfad.
	if err := stream.Send(&chsmdocv1.EncryptRequest{
		Body: &chsmdocv1.EncryptRequest_Header{Header: &chsmdocv1.EncryptHeader{DocId: "d", KeyId: "k", TenantId: "t"}},
	}); err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("Encrypt send: %v", err)
	}
	_, err = stream.Recv()
	assertUnimplemented(t, "Encrypt", err)
}

func TestDecryptUnimplemented(t *testing.T) {
	client, cleanup := dialBufconn(t)
	defer cleanup()

	stream, err := client.Decrypt(context.Background())
	if err != nil {
		t.Fatalf("Decrypt open: %v", err)
	}
	if err := stream.Send(&chsmdocv1.DecryptRequest{
		Body: &chsmdocv1.DecryptRequest_Header{Header: &chsmdocv1.DecryptHeader{DocId: "d", TenantId: "t"}},
	}); err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("Decrypt send: %v", err)
	}
	_, err = stream.Recv()
	assertUnimplemented(t, "Decrypt", err)
}

func TestNewServerNotNil(t *testing.T) {
	if NewServer() == nil {
		t.Fatal("NewServer returned nil")
	}
}
