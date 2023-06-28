package slack

import (
	"strconv"
	"time"
)

type UnixTimestamp time.Time

func (t *UnixTimestamp) UnmarshalJSON(bytes []byte) error {
	q, err := strconv.ParseInt(string(bytes), 10, 64)
	if err != nil {
		return err
	}

	*(*time.Time)(t) = time.Unix(q, 0)

	return nil
}

func (t *UnixTimestamp) Time() *time.Time {
	return (*time.Time)(t)
}
