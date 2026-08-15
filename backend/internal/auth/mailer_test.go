package auth

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
)

func TestSMTPMailerRequiresSTARTTLS(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	done := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		if _, err := fmt.Fprint(conn, "220 localhost ESMTP\r\n"); err != nil {
			done <- err
			return
		}
		if _, err := bufio.NewReader(conn).ReadString('\n'); err != nil {
			done <- err
			return
		}
		_, err = fmt.Fprint(conn, "250 localhost\r\n")
		done <- err
	}()

	mailer, err := NewSMTPMailer(listener.Addr().String(), "", "", "noreply@example.com")
	if err != nil {
		t.Fatal(err)
	}
	err = mailer.Send(context.Background(), "user@example.com", "Verify", "token")
	if err == nil || !strings.Contains(err.Error(), "STARTTLS") {
		t.Fatalf("Send error = %v, want STARTTLS requirement", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
