package structs

import "strings"

type ProbeFailureType string

const (
	ProbeFailureTypeTimeout        ProbeFailureType = "timeout"
	ProbeFailureTypeDNS            ProbeFailureType = "dns"
	ProbeFailureTypeConnect        ProbeFailureType = "connect"
	ProbeFailureTypeTLS            ProbeFailureType = "tls"
	ProbeFailureTypeHTTPStatus     ProbeFailureType = "http_status"
	ProbeFailureTypeInvalidRequest ProbeFailureType = "invalid_request"
	ProbeFailureTypeUnknown        ProbeFailureType = "unknown"
)

func NormalizeProbeFailureType(success bool, raw ProbeFailureType) ProbeFailureType {
	if success {
		return ""
	}

	switch ProbeFailureType(strings.TrimSpace(string(raw))) {
	case ProbeFailureTypeTimeout,
		ProbeFailureTypeDNS,
		ProbeFailureTypeConnect,
		ProbeFailureTypeTLS,
		ProbeFailureTypeHTTPStatus,
		ProbeFailureTypeInvalidRequest:
		return raw
	default:
		return ProbeFailureTypeUnknown
	}
}
