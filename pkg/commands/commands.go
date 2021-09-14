package commands

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"fx_alert/pkg/db"
	"fx_alert/pkg/quoter"
	"fx_alert/pkg/telegram"
)

type CommandType string

const (
	AddValue    CommandType = "/add"
	DeleteValue CommandType = "/del"
	ListValues  CommandType = "/ls"
	DeltaValue  CommandType = "/delta"
	Help        CommandType = "/help"
)

func CommandFromString(txt string) (CommandType, error) {
	txt = strings.TrimSpace(strings.ToLower(txt))
	switch {
	case strings.HasPrefix(txt, string(AddValue)+" "):
		return AddValue, nil
	case strings.HasPrefix(txt, string(DeleteValue)+" ") || (txt == string(DeleteValue)):
		return DeleteValue, nil
	case strings.HasPrefix(txt, string(ListValues)+" ") || (txt == string(ListValues)):
		return ListValues, nil
	case strings.HasPrefix(txt, string(DeltaValue)+" "):
		return DeltaValue, nil
	case txt == string(Help):
		return Help, nil
	}

	return "", errors.New("Unsupported command")
}

type CommandValue struct {
	Command CommandType
	Value   *db.Value
	Delta   uint
}

func Parse(msg string) (*CommandValue, error) {
	msg = strings.TrimSpace(strings.ToLower(msg))
	cmdT, err := CommandFromString(msg)
	if err != nil {
		return nil, fmt.Errorf("Can't parse command: %w", err)
	}

	if cmdT == ListValues {
		if msg == string(ListValues) {
			return &CommandValue{Command: ListValues}, nil
		}
		cv := &CommandValue{
			Command: cmdT,
			Value: &db.Value{
				Key: strings.TrimSpace(strings.TrimPrefix(msg, string(ListValues))),
			},
		}
		return cv, nil
	}
	if cmdT == Help {
		return &CommandValue{Command: Help}, nil
	}

	if cmdT == AddValue {
		v, err := parseValue(msg)
		if err != nil {
			return nil, err
		}
		cv := &CommandValue{
			Command: cmdT,
			Value:   v,
		}

		return cv, nil
	}

	if cmdT == DeleteValue {
		if msg == string(DeleteValue) {
			return &CommandValue{Command: DeleteValue}, nil
		}
		v, err := parseValue(msg)
		if err != nil {
			return nil, err
		}
		cv := &CommandValue{
			Command: cmdT,
			Value:   v,
		}

		return cv, nil
	}

	if cmdT == DeltaValue {
		v, err := parseDeltaValue(msg)
		if err != nil {
			return nil, err
		}
		cv := &CommandValue{
			Command: cmdT,
			Delta:   v,
		}

		return cv, nil
	}

	return nil, errors.New("Unsupported command")
}

func parseValue(msg string) (*db.Value, error) {
	parts := strings.Split(msg, " ")
	if len(parts) != 4 {
		return nil, errors.New("Unsupported command format")
	}
	k := strings.ToUpper(parts[1])
	vt, err := db.ValueTypeFromString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("Can't parse value type: %w", err)
	}
	rawVal := strings.ReplaceAll(parts[3], ",", ".")
	v, err := strconv.ParseFloat(rawVal, 64)
	if err != nil {
		return nil, fmt.Errorf("Can't parse value: %q. %w", rawVal, err)
	}

	return &db.Value{
		Key:       k,
		Value:     v,
		Type:      vt,
		Precision: quoter.GetPrecision(k),
	}, nil
}

func parseDeltaValue(msg string) (uint, error) {
	parts := strings.Split(msg, " ")
	if len(parts) != 2 {
		return 0, errors.New("Unsupported command format")
	}
	rawVal := strings.ReplaceAll(parts[1], ",", ".")
	v, err := strconv.ParseInt(rawVal, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("Can't parse delta value: %q. %w", rawVal, err)
	}
	if v <= 0 {
		return 0, errors.New("Delta value must be > 0")
	}

	return uint(v), nil
}

func HelpAnswer() *telegram.Answer {
	answer := fmt.Sprintf(
		`
Add: %s EURUSD %s 1.2550
Delete: %s EURUSD %s 1.2550
Keyboard delete: %s
List: %s
List: %s usd
Delta: %s 500
Help: %s
`,
		AddValue,
		db.AboveCurrent,
		DeleteValue,
		db.BelowCurrent,
		DeleteValue,
		ListValues,
		ListValues,
		DeltaValue,
		Help,
	)

	return &telegram.Answer{Text: answer}
}
