package handlers

import (
	"fmt"
	"strconv"
	"strings"
)

// parseTimestampToMs menerima format "HH:MM:SS", "MM:SS", atau "SS"
// dan mengembalikan total milidetik
func parseTimestampToMs(ts string) (int, error) {
	parts := strings.Split(strings.TrimSpace(ts), ":")

	var h, m, s int
	var err error

	switch len(parts) {
	case 3:
		h, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid hour in timestamp %q", ts)
		}
		m, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, fmt.Errorf("invalid minute in timestamp %q", ts)
		}
		s, err = strconv.Atoi(parts[2])
		if err != nil {
			return 0, fmt.Errorf("invalid second in timestamp %q", ts)
		}
	case 2:
		m, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid minute in timestamp %q", ts)
		}
		s, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, fmt.Errorf("invalid second in timestamp %q", ts)
		}
	case 1:
		s, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid second in timestamp %q", ts)
		}
	default:
		return 0, fmt.Errorf("unrecognized timestamp format %q", ts)
	}

	return (h*3600+m*60+s) * 1000, nil
}