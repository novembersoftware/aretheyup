package workers

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/url"
	"syscall"

	"github.com/novembersoftware/aretheyup/structs"
)

func classifyInvalidProbeRequest(_ error) structs.ProbeFailureType {
	return structs.ProbeFailureTypeInvalidRequest
}

func classifyProbeFailure(err error) structs.ProbeFailureType {
	if err == nil {
		return ""
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return structs.ProbeFailureTypeTimeout
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return structs.ProbeFailureTypeTimeout
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return structs.ProbeFailureTypeDNS
	}

	var certificateVerificationError *tls.CertificateVerificationError
	if errors.As(err, &certificateVerificationError) {
		return structs.ProbeFailureTypeTLS
	}

	var recordHeaderError tls.RecordHeaderError
	if errors.As(err, &recordHeaderError) {
		return structs.ProbeFailureTypeTLS
	}

	var unknownAuthorityError x509.UnknownAuthorityError
	if errors.As(err, &unknownAuthorityError) {
		return structs.ProbeFailureTypeTLS
	}

	var hostnameError x509.HostnameError
	if errors.As(err, &hostnameError) {
		return structs.ProbeFailureTypeTLS
	}

	var certificateInvalidError x509.CertificateInvalidError
	if errors.As(err, &certificateInvalidError) {
		return structs.ProbeFailureTypeTLS
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if opErr.Op == "dial" || opErr.Op == "read" || opErr.Op == "write" {
			return structs.ProbeFailureTypeConnect
		}
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		var errno syscall.Errno
		if errors.As(urlErr.Err, &errno) {
			switch errno {
			case syscall.ECONNREFUSED, syscall.ECONNRESET, syscall.ENETUNREACH, syscall.EHOSTUNREACH:
				return structs.ProbeFailureTypeConnect
			}
		}
	}

	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.ECONNREFUSED, syscall.ECONNRESET, syscall.ENETUNREACH, syscall.EHOSTUNREACH:
			return structs.ProbeFailureTypeConnect
		}
	}

	return structs.ProbeFailureTypeUnknown
}
