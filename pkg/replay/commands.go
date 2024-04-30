package replay

import (
	"encoding/json"
	"fmt"

	"github.com/tradingcage/tradingcage-go/pkg/bars"
)

type Command struct {
	Cmd     string `json:"cmd"`
	payload interface{}
}

func NewCommand(cmd string, payload interface{}) Command {
	return Command{
		Cmd:     cmd,
		payload: payload,
	}
}

func ParseCommand(data []byte) (Command, error) {
	var rawCmd Command
	if err := json.Unmarshal(data, &rawCmd); err != nil {
		return Command{}, err
	}

	switch rawCmd.Cmd {
	case "play":
		var cmd PlayCommand
		if err := json.Unmarshal(data, &cmd); err != nil {
			return Command{}, err
		}
		return NewCommand(rawCmd.Cmd, cmd), nil
	case "pause":
		var cmd PauseCommand
		return NewCommand(rawCmd.Cmd, cmd), nil
	default:
		return Command{}, fmt.Errorf("unknown command: %s", rawCmd.Cmd)
	}
}

func (c *Command) GetPayload() interface{} {
	return c.payload
}

type PlayCommand struct {
	Frame      bars.Timeframe `json:"frame"`
	ChartFrame bars.Timeframe `json:"chartFrame"`
	Seconds    int            `json:"seconds"`
	RTH        bool           `json:"rth"`
}

func (c *PlayCommand) Valid() error {
	if c.Frame.Empty() {
		return fmt.Errorf("frame is empty")
	}
	if c.ChartFrame.Empty() {
		return fmt.Errorf("chartFrame is empty")
	}
	if c.Seconds <= 0 {
		return fmt.Errorf("seconds is empty")
	}
	return nil
}

type PauseCommand struct{}
