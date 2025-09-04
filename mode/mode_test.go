package mode_test

import (
	"flag"
	"testing"

	"github.com/devagame/due/v2/mode"
)

func TestGetMode(t *testing.T) {
	flag.Parse()

	t.Log(mode.GetMode())
}
