package replay

import (
	"reflect"
	"testing"

	"github.com/tradingcage/tradingcage-go/pkg/bars"
)

func TestParseCommand_Play(t *testing.T) {
	playPayload := `{"cmd":"play","frame":{"value":1,"unit":"m"},"chartFrame":{"value":1,"unit":"m"},"seconds":10,"rth":true}`
	expectedFrame := bars.Timeframe{Value: 1, Unit: "m"}
	expectedChartFrame := bars.Timeframe{Value: 1, Unit: "m"}
	expectedCmd := Command{
		Cmd: "play",
		payload: &PlayCommand{
			Frame:      expectedFrame,
			ChartFrame: expectedChartFrame,
			Seconds:    10,
			RTH:        true,
		},
	}

	cmd, err := ParseCommand([]byte(playPayload))
	if err != nil {
		t.Fatalf("ParseCommand() error = %v, wantErr %v", err, false)
	}

	if cmd.Cmd != expectedCmd.Cmd {
		t.Errorf("ParseCommand() Cmd = %v, want %v", cmd.Cmd, expectedCmd.Cmd)
	}

	if !reflect.DeepEqual(cmd.GetPayload(), expectedCmd.payload) {
		t.Errorf("ParseCommand() payload = %v, want %v", cmd.GetPayload(), expectedCmd.payload)
	}
}

func TestParseCommand_Pause(t *testing.T) {
	pausePayload := `{"cmd":"pause"}`
	expectedCmd := Command{
		Cmd:     "pause",
		payload: &PauseCommand{},
	}

	cmd, err := ParseCommand([]byte(pausePayload))
	if err != nil {
		t.Fatalf("ParseCommand() error = %v, wantErr %v", err, false)
	}

	if cmd.Cmd != expectedCmd.Cmd {
		t.Errorf("ParseCommand() Cmd = %v, want %v", cmd.Cmd, expectedCmd.Cmd)
	}

	if _, ok := cmd.GetPayload().(*PauseCommand); !ok {
		t.Errorf("ParseCommand() payload = %v, want %v", cmd.GetPayload(), expectedCmd.payload)
	}
}

func TestParseCommand_Invalid(t *testing.T) {
	invalidPayload := `{"cmd":"invalid"}`
	_, err := ParseCommand([]byte(invalidPayload))
	if err == nil {
		t.Fatalf("ParseCommand() error = %v, wantErr %v", err, true)
	}
}
