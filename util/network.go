package util

import "fmt"

// NetworkInterface is the base type for structures
// thad need network interface behaviour.
type NetworkInterface struct {
	Host string `json:"Host"`
	Port uint16 `json:"Port"`
}

func (ni *NetworkInterface) Interface() string {
	return fmt.Sprint(ni.Host, ":", ni.Port)
}
