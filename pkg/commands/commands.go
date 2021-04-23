package commands

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"fx_alert/pkg/db"
)

type CommandType string

const (
	AddValue    CommandType = "/add"
	DeleteValue CommandType = "/del"
	ListValues  CommandType = "/ls"
	Help        CommandType = "/help"
)

func CommandFromString(txt string) (CommandType, error) {
	txt = strings.TrimSpace(strings.ToLower(txt))
	switch {
	case strings.HasPrefix(txt, string(AddValue)+" "):
		return AddValue, nil
	case strings.HasPrefix(txt, string(DeleteValue)+" "):
		return DeleteValue, nil
	case txt == string(ListValues):
		return ListValues, nil
	case txt == string(Help):
		return Help, nil
	}

	return "", errors.New("Unsupported command")
}

type CommandValue struct {
	Command CommandType
	Value   *db.Value
}

func Parse(msg string) (*CommandValue, error) {
	msg = strings.TrimSpace(strings.ToLower(msg))
	cmdT, err := CommandFromString(msg)
	if err != nil {
		return nil, fmt.Errorf("Can't parse command: %w", err)
	}

	if cmdT == ListValues {
		return &CommandValue{Command: ListValues}, nil
	}
	if cmdT == Help {
		return &CommandValue{Command: Help}, nil
	}

	if (cmdT == AddValue) || (cmdT == DeleteValue) {
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

	return nil, errors.New("Unsupported command")
}

func parseValue(msg string) (*db.Value, error) {
	parts := strings.Split(msg, " ")
	if len(parts) != 4 {
		return nil, errors.New("Unsupported command format")
	}
	k := parts[1]
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
		Key:   k,
		Value: v,
		Type:  vt,
	}, nil
}

func HelpAnswer() string {
	answer := fmt.Sprintf(
		`
Add: %s EURUSD %s 1.2550
Delete: %s EURUSD %s 1.2550
List: %s
Help: %s
`,
		AddValue,
		db.AboveCurrent,
		DeleteValue,
		db.BelowCurrent,
		ListValues,
		Help,
	)

	return answer
}
