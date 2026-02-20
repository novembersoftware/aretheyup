package algorithm

// TEMP: this is a temporary algorithm for determining the status of a service
// in the future, this will be more robust
// for now, 5 < reports < 10 is Degraded, 10+ is Outage, 5- is Operational

type Status string

const (
	StatusOperational Status = "Operational"
	StatusDegraded    Status = "Degraded"
	StatusOutage      Status = "Outage"
)

func StatusFromCount(count int64) Status {
	if count >= 10 {
		return StatusOutage
	} else if count > 5 {
		return StatusDegraded
	}
	return StatusOperational
}
