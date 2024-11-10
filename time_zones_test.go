package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestAvailableTimeZones(t *testing.T) {
	for _, zones := range AvailableTimeZones {
		for _, zone := range zones {
			_, err := time.LoadLocation(zone)
			assert.NoError(t, err)
		}
	}
}
